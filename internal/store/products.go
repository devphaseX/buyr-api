package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/devphaseX/buyr-api.git/internal/db"
)

type Product struct {
	ID                  string            `json:"id"`
	Name                string            `json:"name"`
	Description         string            `json:"description"`
	Images              []*ProductImage   `json:"images"`
	Featues             []*ProductFeature `json:"features"`
	StockQuantity       int               `json:"stock_quantity"`
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
	View           string                 `json:"view"`
	FeatureEntries map[string]interface{} `json:"feature_entries"`
	ProductID      string                 `json:"product_id"`
}

type ProductStore interface {
	Create(ctx context.Context, product *Product) error
	Publish(ctx context.Context, productID string, vendorID string) error
	Unpublish(ctx context.Context, productID string, vendorID string) error
}

type ProductModel struct {
	db *sql.DB
}

func NewProductModel(db *sql.DB) ProductStore {
	return &ProductModel{db}
}

func create(ctx context.Context, tx *sql.Tx, product *Product) error {
	query := `INSERT INTO products(id, name, description,
			 stock_quantity, total_items_sold_count, vendor_id,
			 discount, price, category_id) 	VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9)
			 RETURNING id, created_at, updated_at
				`
	id := db.GenerateULID()

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	args := []any{id, product.Name, product.Description, product.StockQuantity,
		product.TotalItemsSoldCount, product.VendorID, product.Discount, product.Price, product.CategoryID}

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

		if err := createProductFeatures(ctx, tx, product.ID, product.Featues); err != nil {
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
