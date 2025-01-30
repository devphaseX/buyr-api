package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/devphaseX/buyr-api.git/internal/db"
	"github.com/lib/pq"
)

var (
	ErrProductWishlistedAlready = errors.New("product already in wishlists")
)

type Wishlist struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	ProductID string    `json:"product_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type WhitelistStore interface {
	AddItem(ctx context.Context, whitelist *Wishlist) error
	RemoveItem(ctx context.Context, itemID, userID string) error
	GetWishlistItems(ctx context.Context, userID, vendorID string, filter PaginateQueryFilter) ([]*VendorGroupWishlistItem, Metadata, error)
	GetGroupWishlistItems(ctx context.Context, userID string, itemLimit int, filter PaginateQueryFilter) ([]*VendorWithWishlistItems, Metadata, error)
}

type WishlistModel struct {
	db *sql.DB
}

func NewWishlistModel(db *sql.DB) WhitelistStore {
	return &WishlistModel{db}
}

func (m *WishlistModel) AddItem(ctx context.Context, whitelist *Wishlist) error {
	whitelist.ID = db.GenerateULID()

	query := `
			INSERT INTO wishlists (id, user_id, product_id) VALUES ($1, $2, $3)
			RETURNING created_at, updated_at
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)

	defer cancel()

	args := []any{whitelist.ID, whitelist.UserID, whitelist.ProductID}

	err := m.db.QueryRowContext(ctx, query, args...).Scan(&whitelist.CreatedAt, &whitelist.UpdatedAt)

	if err != nil {
		var pgErr *pq.Error
		switch {
		case errors.As(err, &pgErr):
			if pgErr.Constraint == "wishlists_user_id_product_id_uniq" {
				return ErrProductWishlistedAlready
			}

			fallthrough
		default:
			return err
		}
	}

	return nil
}

func (m *WishlistModel) RemoveItem(ctx context.Context, itemID, userID string) error {
	query := `
			DELETE FROM wishlists
			WHERE id = $1 AND user_id = $2
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	args := []any{itemID, userID}

	res, err := m.db.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}

	rowsCount, err := res.RowsAffected()

	if err != nil {
		return err
	}

	if rowsCount == 0 {
		return ErrRecordNotFound
	}

	return nil
}

type VendorGroupWishlistItem struct {
	Wishlist
	Product struct {
		ID                  string        `json:"id"`
		Name                string        `json:"name"`
		Description         string        `json:"description"`
		StockQuantity       int           `json:"stock_quantity"`
		Status              ProductStatus `json:"status"`
		AvatarURL           string        `json:"avatar_url"`
		Published           bool          `json:"published"`
		TotalItemsSoldCount int           `json:"total_items_sold_count"`
		VendorID            string        `json:"vendor_id"`
		Discount            float64       `json:"discount"`
		Price               float64       `json:"price"`
		CategoryID          string        `json:"category_id"`
		CreatedAt           time.Time     `json:"created_at"`
		UpdatedAt           time.Time     `json:"updated_at"`
		Vendor              struct {
			ID              string `json:"id"`
			VendorName      string `json:"name"`
			VendorAvatarURL string `json:"avatar_url"`
			CreatedAt       string `json:"created_at"`
		} `json:"vendor"`
	} `json:"product"`
}

type VendorWithWishlistItems struct {
	ID              string                     `json:"id"`
	VendorName      string                     `json:"vendor_name"`
	VendorAvatarURL string                     `json:"vendor_avatar_url"`
	Items           []*VendorGroupWishlistItem `json:"items"`
	Metadata        Metadata                   `json:"metadata"`
}

func (m *WishlistModel) GetGroupWishlistItems(ctx context.Context, userID string, itemLimit int, filter PaginateQueryFilter) ([]*VendorWithWishlistItems, Metadata, error) {
	query := `
        WITH vendor_items AS (
            SELECT
                v.id AS vendor_id,
                v.business_name AS vendor_name,
                v.created_at AS vendor_created_at,
                u.avatar_url AS vendor_avatar_url,
                w.id AS wishlist_id,
                w.user_id,
                w.product_id,
                w.created_at AS wishlist_created_at,
                w.updated_at AS wishlist_updated_at,
                p.name AS product_name,
                p.description AS product_description,
                p.stock_quantity,
                p.status,
                pi.url AS product_avatar_url,
                p.published,
                p.total_items_sold_count,
                p.discount,
                p.price,
                p.category_id,
                p.created_at AS product_created_at,
                p.updated_at AS product_updated_at,
                ROW_NUMBER() OVER (PARTITION BY v.id ORDER BY w.created_at DESC) AS item_rank,
                COUNT(*) OVER (PARTITION BY v.id) AS total_items_per_vendor
            FROM wishlists w
            JOIN products p ON w.product_id = p.id
            LEFT JOIN product_images pi ON pi.product_id = p.id AND pi.is_primary = true
            JOIN vendor_users v ON p.vendor_id = v.id
            JOIN users u ON u.id = v.user_id
            WHERE w.user_id = $1
        ),
        paginated_vendors AS (
            SELECT DISTINCT
                vendor_id,
                vendor_name,
                vendor_created_at,
                vendor_avatar_url,
                total_items_per_vendor
            FROM vendor_items
            ORDER BY vendor_id
            LIMIT $2 OFFSET $3
        )
        SELECT
            pv.vendor_id,
            pv.vendor_name,
            pv.vendor_avatar_url,
            pv.total_items_per_vendor,
            (
                SELECT json_agg(
                    json_build_object(
                        'id', vi.wishlist_id,
                        'user_id', vi.user_id,
                        'product_id', vi.product_id,
                        'created_at', vi.wishlist_created_at,
                        'updated_at', vi.wishlist_updated_at,
                        'product', json_build_object(
                            'id', vi.product_id,
                            'name', vi.product_name,
                            'description', vi.product_description,
                            'stock_quantity', vi.stock_quantity,
                            'status', vi.status,
                            'vendor_id', vi.vendor_id,
                            'avatar_url', vi.product_avatar_url,
                            'published', vi.published,
                            'total_items_sold_count', vi.total_items_sold_count,
                            'discount', vi.discount,
                            'price', vi.price,
                            'category_id', vi.category_id,
                            'created_at', vi.product_created_at,
                            'updated_at', vi.product_updated_at,
                            'vendor', json_build_object(
                                'id', pv.vendor_id,
                                'name', pv.vendor_name,
                                'avatar_url', pv.vendor_avatar_url,
                                'created_at', vendor_created_at
                            )
                        )
                    )
                )
                FROM vendor_items vi
                WHERE vi.vendor_id = pv.vendor_id AND vi.item_rank <= $4
            ) AS items
        FROM paginated_vendors pv
        ORDER BY pv.vendor_id;
    `

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	rows, err := m.db.QueryContext(ctx, query, userID, filter.Limit(), filter.Offset(), itemLimit)
	if err != nil {
		return nil, Metadata{}, fmt.Errorf("failed to query wishlist items: %w", err)
	}
	defer rows.Close()

	var (
		vendorsWithItems = []*VendorWithWishlistItems{}
		totalVendors     int
	)

	for rows.Next() {
		var (
			vendorID            string
			vendorName          string
			vendorAvatarURL     sql.NullString
			totalItemsPerVendor int
			itemsJSON           []byte
		)

		err := rows.Scan(
			&vendorID, &vendorName, &vendorAvatarURL, &totalItemsPerVendor, &itemsJSON,
		)
		if err != nil {
			return nil, Metadata{}, fmt.Errorf("failed to scan vendor: %w", err)
		}

		var items []*VendorGroupWishlistItem
		if err := json.Unmarshal(itemsJSON, &items); err != nil {
			return nil, Metadata{}, fmt.Errorf("failed to unmarshal items JSON: %w", err)
		}

		vendor := &VendorWithWishlistItems{
			ID:              vendorID,
			VendorName:      vendorName,
			VendorAvatarURL: vendorAvatarURL.String,
			Items:           items,
			Metadata: Metadata{
				CurrentPage:  1,
				PageSize:     itemLimit,
				TotalRecords: totalItemsPerVendor,
				LastPage:     int(math.Ceil(float64(totalItemsPerVendor) / float64(itemLimit))),
			},
		}

		vendorsWithItems = append(vendorsWithItems, vendor)
	}

	if err = rows.Err(); err != nil {
		return nil, Metadata{}, fmt.Errorf("error after iterating over rows: %w", err)
	}

	// Calculate top-level pagination metadata
	totalVendors = len(vendorsWithItems)
	metadata := calculateMetadata(totalVendors, filter.Page, filter.PageSize)

	return vendorsWithItems, metadata, nil
}

func (m *WishlistModel) GetWishlistItems(ctx context.Context, userID, vendorID string, filter PaginateQueryFilter) ([]*VendorGroupWishlistItem, Metadata, error) {
	query := `
        WITH vendor_items AS (
            SELECT
                v.id AS vendor_id,
                v.business_name AS vendor_name,
                v.created_at AS vendor_created_at,
                u.avatar_url AS vendor_avatar_url,
                w.id AS wishlist_id,
                w.user_id,
                w.product_id,
                w.created_at AS wishlist_created_at,
                w.updated_at AS wishlist_updated_at,
                p.name AS product_name,
                p.description AS product_description,
                p.stock_quantity,
                p.status,
                pi.url AS product_avatar_url,
                p.published,
                p.total_items_sold_count,
                p.discount,
                p.price,
                p.category_id,
                p.created_at AS product_created_at,
                p.updated_at AS product_updated_at,
                COUNT(*) OVER () AS total_items_count
            FROM wishlists w
            JOIN products p ON w.product_id = p.id
            LEFT JOIN product_images pi ON pi.product_id = p.id AND pi.is_primary = true
            JOIN vendor_users v ON p.vendor_id = v.id
            JOIN users u ON u.id = v.user_id
            WHERE w.user_id = $1 AND v.id = $2
        )
        SELECT
            vi.total_items_count,
            vi.wishlist_id,
            vi.user_id,
            vi.product_id,
            vi.wishlist_created_at,
            vi.wishlist_updated_at,
            json_build_object(
                'id', vi.product_id,
                'name', vi.product_name,
                'description', vi.product_description,
                'stock_quantity', vi.stock_quantity,
                'status', vi.status,
                'vendor_id', vi.vendor_id,
                'avatar_url', vi.product_avatar_url,
                'published', vi.published,
                'total_items_sold_count', vi.total_items_sold_count,
                'discount', vi.discount,
                'price', vi.price,
                'category_id', vi.category_id,
                'created_at', vi.product_created_at,
                'updated_at', vi.product_updated_at,
                'vendor', json_build_object(
                    'id', vi.vendor_id,
                    'name', vi.vendor_name,
                    'avatar_url', vi.vendor_avatar_url,
                    'created_at', vi.vendor_created_at
                )
            ) AS product
        FROM vendor_items vi
        ORDER BY vi.wishlist_created_at DESC
        LIMIT $3 OFFSET $4;
    `

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	rows, err := m.db.QueryContext(ctx, query, userID, vendorID, filter.PageSize, (filter.Page-1)*filter.PageSize)
	if err != nil {
		return nil, Metadata{}, fmt.Errorf("failed to query wishlist items: %w", err)
	}
	defer rows.Close()

	var (
		wishlistItems []*VendorGroupWishlistItem
		totalRecords  int
	)

	for rows.Next() {
		var (
			item      VendorGroupWishlistItem
			itemsJSON []byte
		)

		err := rows.Scan(
			&totalRecords, &item.ID, &item.UserID, &item.ProductID,
			&item.CreatedAt, &item.UpdatedAt, &itemsJSON,
		)
		if err != nil {
			return nil, Metadata{}, fmt.Errorf("failed to scan row: %w", err)
		}

		if err := json.Unmarshal(itemsJSON, &item.Product); err != nil {
			return nil, Metadata{}, fmt.Errorf("failed to unmarshal product JSON: %w", err)
		}

		wishlistItems = append(wishlistItems, &item)
	}

	if err := rows.Err(); err != nil {
		return nil, Metadata{}, fmt.Errorf("rows error: %w", err)
	}

	metadata := calculateMetadata(totalRecords, filter.Page, filter.PageSize)
	return wishlistItems, metadata, nil
}
