package store

import "time"

type Product struct {
	ID                  string    `json:"id"`
	Name                string    `json:"name"`
	Description         string    `json:"description"`
	StockQuantity       int       `json:"stock_quantity"`
	TotalItemsSoldCount int       `json:"total_items_sold_count"`
	VendorID            string    `json:"vendor_id"`
	Discount            float64   `json:"discount"`
	Price               float64   `json:"price"`
	CategoryID          string    `json:"category_id"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
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
