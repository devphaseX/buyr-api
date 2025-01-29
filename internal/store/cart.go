package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/devphaseX/buyr-api.git/internal/db"
)

type Cart struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	IsActive  bool      `json:"is_active"`
}

type CartItem struct {
	ID        string    `json:"id"`
	CartID    string    `json:"cart_id"`
	ProductID string    `json:"product_id"`
	AddedAt   time.Time `json:"added_at"`
	Quantity  int       `json:"quantity"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CartStore interface {
	CreateCart(userID string) (*Cart, error)
	GetCartByUserID(userID string) (*Cart, error)
}

type CartModel struct {
	db *sql.DB
}

func NewCartModel(db *sql.DB) CartStore {
	return &CartModel{db}
}

// CreateCart creates a new cart for a user
func (m *CartModel) CreateCart(userID string) (*Cart, error) {
	cartID := db.GenerateULID()
	query := `
		INSERT INTO carts (id, user_id)
		VALUES ($1, $2)
		RETURNING created_at, updated_at, is_active
	`

	cart := &Cart{
		ID:     cartID,
		UserID: userID,
	}

	err := m.db.QueryRow(query, cartID, userID).Scan(&cart.CreatedAt, &cart.UpdatedAt, &cart.IsActive)
	if err != nil {
		return nil, err
	}

	return cart, nil
}

func (m *CartModel) GetCartByUserID(userID string) (*Cart, error) {
	query := `
		SELECT id, user_id, created_at, updated_at, is_active
		FROM carts
		WHERE user_id = $1
	`

	cart := &Cart{}
	var isActive sql.NullBool
	err := m.db.QueryRow(query, userID).Scan(&cart.ID, &cart.UserID, &cart.CreatedAt, &cart.UpdatedAt, &isActive)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}

	if isActive.Valid {
		cart.IsActive = isActive.Bool
	}

	return cart, nil
}

type CartItemStore interface {
	AddItem(ctx context.Context, cartID, productID string, quantity int) (*CartItem, error)
	GetItemByID(ctx context.Context, cartID, itemID string) (*CartItemDetails, error)
	UpdateItem(ctx context.Context, itemID string, quantity int) error
	DeleteItem(ictx context.Context, temID string) error
	GetItems(ctx context.Context, cartID string) ([]*CartItemDetails, error)
}

type CartItemModel struct {
	DB *sql.DB
}

func NewCartItemModel(db *sql.DB) CartItemStore {
	return &CartItemModel{db}
}

// AddCartItem adds a new item to the cart
func (m *CartItemModel) AddItem(ctx context.Context, cartID, productID string, quantity int) (*CartItem, error) {
	itemID := db.GenerateULID()
	query := `
		INSERT INTO cart_items (id, cart_id, product_id, added_at, quantity)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING  created_at, updated_at
	`

	item := &CartItem{
		ID:        itemID,
		CartID:    cartID,
		ProductID: productID,
		Quantity:  quantity,
		AddedAt:   time.Now(),
	}

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()
	err := m.DB.QueryRowContext(ctx, query, itemID, cartID,
		productID, item.AddedAt, quantity).
		Scan(&item.CreatedAt, &item.UpdatedAt)

	if err != nil {
		return nil, err
	}

	return item, nil
}

type CartItemDetails struct {
	CartItem
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
	} `json:"product"`

	Vendor struct {
		ID              string    `json:"id"`
		BusinessName    string    `json:"business_name"`
		BusinessAddress string    `json:"business_address"`
		ContactNumber   string    `json:"contact_number"`
		AvatarURL       string    `json:"avatar_url"`
		UserID          string    `json:"user_id"`
		City            string    `json:"city"`
		Country         string    `json:"country"`
		CreatedAt       time.Time `json:"created_at"`
		UpdatedAt       time.Time `json:"updated_at"`
	}
}

func (m *CartItemModel) GetItemByID(ctx context.Context, cartID, itemID string) (*CartItemDetails, error) {
	query := `
		SELECT
			ci.id, ci.cart_id, ci.product_id, ci.added_at, ci.quantity, ci.created_at, ci.updated_at,
			p.id, p.name, p.description, p.stock_quantity, p.status, pi.url, p.published,
			p.total_items_sold_count, p.vendor_id, p.discount, p.price, p.category_id, p.created_at, p.updated_at,
			v.id, v.business_name, v.business_address, v.contact_number, u.avatar_url, v.user_id,
			v.city, v.country, v.created_at, v.updated_at
		FROM cart_items ci
		JOIN products p ON ci.product_id = p.id
		LEFT JOIN product_images pi ON pi.product_id = p.id AND pi.is_primary = true
		JOIN vendor_users v ON p.vendor_id = v.id
		JOIN users u ON u.id = v.user_id
		WHERE ci.id = $1 AND ci.cart_id = $2
	`

	var details CartItemDetails

	var (
		vendorAvatarURL  sql.NullString
		productAvatarURL sql.NullString
	)

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()
	err := m.DB.QueryRowContext(ctx, query, itemID, cartID).Scan(
		&details.ID, &details.CartID, &details.ProductID, &details.AddedAt, &details.Quantity,
		&details.CreatedAt, &details.UpdatedAt,
		&details.Product.ID, &details.Product.Name, &details.Product.Description, &details.Product.StockQuantity,
		&details.Product.Status, &productAvatarURL, &details.Product.Published,
		&details.Product.TotalItemsSoldCount, &details.Product.VendorID, &details.Product.Discount,
		&details.Product.Price, &details.Product.CategoryID, &details.Product.CreatedAt, &details.Product.UpdatedAt,
		&details.Vendor.ID, &details.Vendor.BusinessName, &details.Vendor.BusinessAddress,
		&details.Vendor.ContactNumber, &vendorAvatarURL, &details.Vendor.UserID,
		&details.Vendor.City, &details.Vendor.Country, &details.Vendor.CreatedAt, &details.Vendor.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}

	if vendorAvatarURL.Valid {
		details.Product.AvatarURL = vendorAvatarURL.String
	}

	if productAvatarURL.Valid {
		details.Product.AvatarURL = productAvatarURL.String
	}

	return &details, nil
}

// UpdateCartItem updates a cart item's quantity
func (m *CartItemModel) UpdateItem(ctx context.Context, itemID string, quantity int) error {
	query := `
		UPDATE cart_items
		SET quantity = $1, updated_at = CURRENT_TIMESTAMP
		WHERE id = $2
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()
	_, err := m.DB.Exec(query, quantity, itemID)
	return err
}

// DeleteCartItem deletes a cart item by its ID
func (m *CartItemModel) DeleteItem(ctx context.Context, itemID string) error {
	query := `
		DELETE FROM cart_items
		WHERE id = $1
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()
	_, err := m.DB.Exec(query, itemID)
	return err
}

func (m *CartItemModel) GetItems(ctx context.Context, cartID string) ([]*CartItemDetails, error) {
	query := `
		SELECT count(ci.id) OVER(),
			ci.id, ci.cart_id, ci.product_id, ci.added_at, ci.quantity, ci.created_at, ci.updated_at,
			p.id, p.name, p.description, p.stock_quantity, p.status,  pi.url, p.published,
			p.total_items_sold_count, p.vendor_id,  p.discount, p.price, p.category_id, p.created_at, p.updated_at,
			v.id, v.business_name, v.business_address, v.contact_number, u.avatar_url, v.user_id,
			v.city, v.country, v.created_at, v.updated_at
		FROM cart_items ci
		JOIN products p ON ci.product_id = p.id
		LEFT JOIN product_images pi ON pi.product_id = p.id AND pi.is_primary = true
		JOIN vendor_users v ON p.vendor_id = v.id
		JOIN users u ON u.id = v.user_id
		WHERE ci.cart_id = $1
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	rows, err := m.DB.QueryContext(ctx, query, cartID)
	if err != nil {
		return nil, fmt.Errorf("failed to query cart items: %w", err)
	}
	defer rows.Close()

	var (
		items        = []*CartItemDetails{}
		totalRecords int
	)

	for rows.Next() {
		var (
			details          CartItemDetails
			productAvatarURL sql.NullString
			vendorAvatarURL  sql.NullString
		)

		err := rows.Scan(
			&totalRecords,
			&details.ID, &details.CartID, &details.ProductID, &details.AddedAt, &details.Quantity,
			&details.CreatedAt, &details.UpdatedAt,
			&details.Product.ID, &details.Product.Name, &details.Product.Description, &details.Product.StockQuantity,
			&details.Product.Status, &productAvatarURL, &details.Product.Published,
			&details.Product.TotalItemsSoldCount, &details.Product.VendorID, &details.Product.Discount,
			&details.Product.Price, &details.Product.CategoryID, &details.Product.CreatedAt, &details.Product.UpdatedAt,
			&details.Vendor.ID, &details.Vendor.BusinessName, &details.Vendor.BusinessAddress,
			&details.Vendor.ContactNumber, &vendorAvatarURL, &details.Vendor.UserID,
			&details.Vendor.City, &details.Vendor.Country, &details.Vendor.CreatedAt, &details.Vendor.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan cart item: %w", err)
		}

		if productAvatarURL.Valid {
			details.Product.AvatarURL = productAvatarURL.String
		}

		if vendorAvatarURL.Valid {
			details.Vendor.AvatarURL = vendorAvatarURL.String
		}

		items = append(items, &details)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error after iterating over cart items rows: %w", err)
	}

	return items, nil
}
