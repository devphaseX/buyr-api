package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/devphaseX/buyr-api.git/internal/db"
)

type ProductStatus string

var (
	PendingProductStatus  ProductStatus = "pending"
	ApprovedProductStatus ProductStatus = "approved"
	RejectedProductStatus ProductStatus = "rejected"
)

type ProductFeatureView string

var (
	TableProductFeatureView  ProductFeatureView = "table"
	ListProductFeatureView   ProductFeatureView = "list"
	BulletProductFeatureView ProductFeatureView = "bullet"
)

type Product struct {
	ID                  string            `json:"id"`
	Name                string            `json:"name"`
	Description         string            `json:"description"`
	Images              []*ProductImage   `json:"images"`
	Features            []*ProductFeature `json:"features,omitempty"`
	StockQuantity       int               `json:"stock_quantity"`
	Status              ProductStatus     `json:"status"`
	Published           bool              `json:"published"`
	TotalItemsSoldCount int               `json:"total_items_sold_count"`
	VendorID            string            `json:"vendor_id"`
	Discount            float64           `json:"discount"`
	Price               float64           `json:"price"`
	CategoryID          string            `json:"category_id"`
	CreatedAt           time.Time         `json:"created_at"`
	UpdatedAt           time.Time         `json:"updated_at"`
}

type ProductImage struct {
	ID        string    `json:"id"`
	ProductID string    `json:"product_id"`
	URL       string    `json:"url"`
	IsPrimary bool      `json:"is_primary"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ProductFeature struct {
	ID             string                 `json:"id"`
	Title          string                 `json:"title"`
	View           ProductFeatureView     `json:"view"`
	FeatureEntries map[string]interface{} `json:"feature_entries"`
	ProductID      string                 `json:"product_id"`
}

type ProductStore interface {
	Create(ctx context.Context, product *Product) error
	Publish(ctx context.Context, productID string, vendorID string) error
	Unpublish(ctx context.Context, productID string, vendorID string) error
	Reject(ctx context.Context, productID string) error
	Approve(ctx context.Context, productID string) error
	GetWithDetails(ctx context.Context, productID string) (*Product, error)
	GetProductByID(ctx context.Context, productID string) (*Product, error)
	GetProducts(ctx context.Context, filter PaginateQueryFilter) ([]*Product, Metadata, error)
}

type ProductModel struct {
	db *sql.DB
}

func NewProductModel(db *sql.DB) ProductStore {
	return &ProductModel{db}
}

func create(ctx context.Context, tx *sql.Tx, product *Product) error {
	query := `INSERT INTO products(id, name, description,
			 stock_quantity, total_items_sold_count, status, published, vendor_id,
			 discount, price, category_id) 	VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9)
			 RETURNING id, created_at, updated_at
				`
	id := db.GenerateULID()

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	args := []any{id, product.Name, product.Description, product.StockQuantity,
		product.TotalItemsSoldCount, product.Status, product.Published, product.VendorID,
		product.Discount, product.Price, product.CategoryID}

	err := tx.QueryRowContext(ctx, query, args...).Scan(&product.ID, &product.CreatedAt, &product.UpdatedAt)

	if err != nil {
		return err
	}

	return nil
}

func createProductImages(ctx context.Context, tx *sql.Tx, productID string, images []*ProductImage) error {
	query := `INSERT INTO product_images(id, product_id, url, is_primary)
				  VALUES ($1, $2, $3, $4) RETURNING id, created_at, updated_at`
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)

	defer cancel()
	var wg sync.WaitGroup

	errCh := make(chan error, len(images))

	for _, image := range images {
		wg.Add(1)

		go func(img *ProductImage) {
			defer wg.Done()

			id := db.GenerateULID()
			img.ProductID = productID

			args := []any{id, img.ProductID, img.URL, img.IsPrimary}

			err := tx.QueryRowContext(ctx, query, args...).Scan(&img.ID, &img.CreatedAt, &img.UpdatedAt)

			if err != nil {
				errCh <- err
				return
			}

		}(image)
	}

	wg.Wait()

	close(errCh)

	for err := range errCh {
		if err != nil {
			return err
		}
	}

	return nil
}

func createProductFeatures(ctx context.Context, tx *sql.Tx, productID string, features []*ProductFeature) error {
	query := `INSERT INTO product_features(id, title, view, feature_entries, product_id)
				  VALUES ($1, $2, $3, $4, $5)`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	var wg sync.WaitGroup

	errCh := make(chan error, len(features))

	for _, feature := range features {
		wg.Add(1)
		go func(feat *ProductFeature) {
			defer wg.Done()

			feat.ID = db.GenerateULID()
			feat.ProductID = productID

			featureEntriesJSON, err := json.Marshal(feat.FeatureEntries)

			if err != nil {
				errCh <- fmt.Errorf("failed to serialize feature entries: %w", err)
				return
			}

			args := []any{feat.ID, feat.Title, feat.View, featureEntriesJSON, feat.ProductID}

			_, err = tx.ExecContext(ctx, query, args...)

			if err != nil {
				errCh <- err
				return
			}
		}(feature)
	}

	wg.Wait()

	close(errCh)

	for err := range errCh {
		if err != nil {
			return err
		}

	}
	return nil
}

func (m *ProductModel) Create(ctx context.Context, product *Product) error {
	return withTrx(m.db, ctx, func(tx *sql.Tx) error {
		if err := create(ctx, tx, product); err != nil {
			return err
		}

		if err := createProductImages(ctx, tx, product.ID, product.Images); err != nil {
			return err
		}

		if err := createProductFeatures(ctx, tx, product.ID, product.Features); err != nil {
			return err
		}

		return nil
	})
}

func (m *ProductModel) Publish(ctx context.Context, productID string, vendorID string) error {
	query := `UPDATE products SET published = true WHERE id = $1 AND vendor_id = $2`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	// Execute the query
	result, err := m.db.ExecContext(ctx, query, productID, vendorID)
	if err != nil {
		return fmt.Errorf("failed to publish product: %w", err)
	}

	// Check if the product was actually updated
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrRecordNotFound
	}

	return nil
}

func (m *ProductModel) Unpublish(ctx context.Context, productID string, vendorID string) error {
	query := `UPDATE products SET published = false WHERE id = $1 AND vendor_id = $2`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()
	// Execute the query
	result, err := m.db.ExecContext(ctx, query, productID, vendorID)
	if err != nil {
		return fmt.Errorf("failed to unpublish product: %w", err)
	}

	// Check if the product was actually updated
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrRecordNotFound
	}

	return nil
}

func (m *ProductModel) Reject(ctx context.Context, productID string) error {
	query := `
		UPDATE products
		SET status = $1, published = $2
		WHERE id = $4 AND status = $5
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	updatedAt := time.Now()
	result, err := m.db.ExecContext(ctx, query, RejectedProductStatus, false, updatedAt, productID, PendingProductStatus)
	if err != nil {
		return fmt.Errorf("failed to reject product: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrRecordNotFound
	}

	return nil
}

func (m *ProductModel) Approve(ctx context.Context, productID string) error {
	query := `
		UPDATE products
		SET status = $1
		WHERE id = $3 AND status = $4
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	updatedAt := time.Now()
	result, err := m.db.ExecContext(ctx, query, ApprovedProductStatus, updatedAt, productID, PendingProductStatus)
	if err != nil {
		return fmt.Errorf("failed to approve product: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrRecordNotFound
	}

	return nil
}
func (s *ProductModel) GetWithDetails(ctx context.Context, productID string) (*Product, error) {
	query := `
		SELECT
			p.id, p.name, p.description, p.stock_quantity, p.status, p.published, p.discount, p.price, p.category_id,
			p.total_items_sold_count, p.vendor_id, p.created_at, p.updated_at,
			COALESCE(
				(SELECT json_agg(DISTINCT jsonb_build_object(
					'id', pi.id,
					'url', pi.url,
					'is_primary', pi.is_primary,
					'product_id', pi.product_id,
					'created_at', pi.created_at,
					'updated_at', pi.updated_at
				))
				FROM product_images pi
				WHERE pi.product_id = p.id),
				'[]'
			) AS images,
			COALESCE(
				(SELECT json_agg(DISTINCT jsonb_build_object(
					'id', pf.id,
					'title', pf.title,
					'view', pf.view,
					'product_id', pf.product_id,
					'feature_entries', pf.feature_entries
				))
				FROM product_features pf
				WHERE pf.product_id = p.id),
				'[]'
			) AS features
		FROM
			products p
		WHERE
			p.id = $1;
	`
	row := s.db.QueryRowContext(ctx, query, productID)
	var (
		product     = &Product{}
		imageJSON   string
		featureJSON string
	)
	err := row.Scan(&product.ID, &product.Name, &product.Description,
		&product.StockQuantity, &product.Status, &product.Published, &product.Discount, &product.Price,
		&product.CategoryID, &product.TotalItemsSoldCount,
		&product.VendorID, &product.CreatedAt, &product.UpdatedAt,
		&imageJSON, &featureJSON)
	if err != nil {
		switch {
		case errors.Is(err, ErrRecordNotFound):
			return nil, ErrRecordNotFound
		default:
			return nil, fmt.Errorf("failed to scan product details: %w", err)
		}
	}
	product.Images = parseImages(imageJSON)
	product.Features = parseFeatures(featureJSON)
	return product, nil
}

// Helper function to parse images from JSON string
func parseImages(imagesJSON string) []*ProductImage {
	var images []*ProductImage
	if err := json.Unmarshal([]byte(imagesJSON), &images); err != nil {
		return nil
	}
	return images
}

// Helper function to parse features from JSON string
func parseFeatures(featuresJSON string) []*ProductFeature {
	var features []*ProductFeature
	if err := json.Unmarshal([]byte(featuresJSON), &features); err != nil {
		return nil // Handle error appropriately in production code
	}
	return features
}

func (s *ProductModel) GetProducts(ctx context.Context, filter PaginateQueryFilter) ([]*Product, Metadata, error) {
	query := fmt.Sprintf(`
		SELECT
			count(p.id) OVER(),
			p.id, p.name, p.description, p.stock_quantity, p.status, p.published,
			p.discount, p.price, p.category_id, p.total_items_sold_count,
			p.vendor_id, p.created_at, p.updated_at,
			COALESCE(
				(SELECT json_agg(DISTINCT jsonb_build_object(
					'id', pi.id,
					'url', pi.url,
					'is_primary', pi.is_primary,
					'product_id', pi.product_id,
					'created_at', pi.created_at,
					'updated_at', pi.updated_at
				))
				FROM product_images pi
				WHERE pi.product_id = p.id and pi.is_primary = true),
				'[]'
			) AS images
		FROM products p
		ORDER BY p.%s %s
		LIMIT $1 OFFSET $2
	`, filter.SortColumn(), filter.SortDirection())

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	rows, err := s.db.QueryContext(ctx, query, filter.Limit(), filter.Offset())
	if err != nil {
		return nil, Metadata{}, fmt.Errorf("failed to query products: %w", err)
	}
	defer rows.Close()

	var (
		products     = []*Product{}
		totalRecords int
	)

	for rows.Next() {
		var (
			product   = &Product{}
			imageJSON string
		)

		err := rows.Scan(
			&totalRecords,
			&product.ID,
			&product.Name,
			&product.Description,
			&product.StockQuantity,
			&product.Status,
			&product.Published,
			&product.Discount,
			&product.Price,
			&product.CategoryID,
			&product.TotalItemsSoldCount,
			&product.VendorID,
			&product.CreatedAt,
			&product.UpdatedAt,
			&imageJSON,
		)

		if err != nil {
			return nil, Metadata{}, fmt.Errorf("failed to scan product row: %w", err)
		}

		product.Images = parseImages(imageJSON)
		products = append(products, product)
	}

	if err = rows.Err(); err != nil {
		return nil, Metadata{}, fmt.Errorf("error after iterating over product rows: %w", err)
	}

	metadata := calculateMetadata(totalRecords, filter.Page, filter.PageSize)

	return products, metadata, nil
}

func (m *ProductModel) GetProductByID(ctx context.Context, productID string) (*Product, error) {
	query := `SELECT id, name, description, stock_quantity, status, published, total_items_sold_count,vendor_id,
			 discount,  price,category_id, created_at, updated_at  FROM products WHERE id = $1`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	product := &Product{}

	err := m.db.QueryRowContext(ctx, query, productID).Scan(&product.ID, &product.Name, &product.Description,
		&product.StockQuantity, &product.Status, &product.Published,
		&product.TotalItemsSoldCount, &product.VendorID, &product.Discount, &product.Price,
		&product.CategoryID, &product.CreatedAt, &product.UpdatedAt)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound

		default:
			return nil, err
		}
	}

	return product, nil
}
