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
	ErrProductAlreadyCarted = errors.New("product already carted")
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
	Price     float64   `json:"-"`
	AddedAt   time.Time `json:"added_at"`
	Quantity  int       `json:"quantity"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Product   *Product  `json:"product,omitempty"`
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
		INSERT INTO carts (id, is_active, user_id)
		VALUES ($1, $2, $3)
		RETURNING created_at, updated_at, is_active
	`

	cart := &Cart{
		ID:       cartID,
		UserID:   userID,
		IsActive: true,
	}

	err := m.db.QueryRow(query, cartID, cart.IsActive, userID).Scan(&cart.CreatedAt, &cart.UpdatedAt, &cart.IsActive)
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
	GetGroupCartItems(ctx context.Context, cartID string, itemLimit int, filter PaginateQueryFilter) ([]*VendorWithItems, Metadata, error)
	GetCartItems(ctx context.Context, cartID, vendorID string, filter PaginateQueryFilter) ([]*VendorGroupCartItem, Metadata, error)
	SetItemQuantity(ctx context.Context, cartID, cartItemID string, quantity int) error
	GetItemsByIDS(ctx context.Context, cartID string, ids []string) ([]*CartItem, error)
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
		var pgErr *pq.Error
		switch {
		case errors.As(err, &pgErr):
			if pgErr.Constraint == "cart_items_cart_product_unique" {
				return nil, ErrProductAlreadyCarted
			}

			fallthrough

		default:
			return nil, err
		}
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

type VendorGroupCartItem struct {
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
		Vendor              struct {
			ID              string `json:"id"`
			VendorName      string `json:"name"`
			VendorAvatarURL string `json:"avatar_url"`
			CreatedAt       string `json:"created_at"`
		} `json:"vendor"`
	} `json:"product"`
}

type VendorWithItems struct {
	ID              string                 `json:"id"`
	VendorName      string                 `json:"vendor_name"`
	VendorAvatarURL string                 `json:"vendor_avatar_url"`
	Items           []*VendorGroupCartItem `json:"items"`
	Metadata        Metadata               `json:"metadata"`
}

func (m *CartItemModel) GetGroupCartItems(ctx context.Context, cartID string,
	itemLimit int, filter PaginateQueryFilter) ([]*VendorWithItems, Metadata, error) {
	query := `
		WITH vendor_items AS (
			SELECT
				v.id AS vendor_id,
				v.business_name AS vendor_name,
				v.created_at as vendor_created_at,
				u.avatar_url AS vendor_avatar_url,
				ci.id AS item_id,
				ci.cart_id,
				ci.product_id,
				ci.added_at,
				ci.quantity,
				ci.created_at AS ci_created_at,
				ci.updated_at AS ci_updated_at,
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
				ROW_NUMBER() OVER (PARTITION BY v.id ORDER BY ci.added_at DESC) AS item_rank,
				COUNT(*) OVER (PARTITION BY v.id) AS total_items_per_vendor
			FROM cart_items ci
			JOIN products p ON ci.product_id = p.id
			LEFT JOIN product_images pi ON pi.product_id = p.id AND pi.is_primary = true
			JOIN vendor_users v ON p.vendor_id = v.id
			JOIN users u ON u.id = v.user_id
			WHERE ci.cart_id = $1
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
						'id', vi.item_id,
						'product_id', vi.product_id,
						'cart_id', vi.cart_id,
						'added_at', vi.added_at,
						'quantity', vi.quantity,
						'created_at', vi.ci_created_at,
						'updated_at', vi.ci_updated_at,
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

	rows, err := m.DB.QueryContext(ctx, query, cartID, filter.Limit(), filter.Offset(), itemLimit)
	if err != nil {
		return nil, Metadata{}, fmt.Errorf("failed to query cart items: %w", err)
	}
	defer rows.Close()

	var (
		vendorsWithItems = []*VendorWithItems{}
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

		var items []*VendorGroupCartItem
		if err := json.Unmarshal(itemsJSON, &items); err != nil {
			return nil, Metadata{}, fmt.Errorf("failed to unmarshal items JSON: %w", err)
		}

		vendor := &VendorWithItems{
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

func (m *CartItemModel) GetCartItems(ctx context.Context, cartID, vendorID string, filter PaginateQueryFilter) ([]*VendorGroupCartItem, Metadata, error) {
	query := `
        WITH vendor_items AS (
            SELECT
                v.id AS vendor_id,
                v.business_name AS vendor_name,
                v.created_at AS vendor_created_at,
                u.avatar_url AS vendor_avatar_url,
                ci.id AS item_id,
                ci.cart_id,
                ci.product_id,
                ci.added_at,
                ci.quantity,
                ci.created_at AS ci_created_at,
                ci.updated_at AS ci_updated_at,
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
            FROM cart_items ci
            JOIN products p ON ci.product_id = p.id
            LEFT JOIN product_images pi ON pi.product_id = p.id AND pi.is_primary = true
            JOIN vendor_users v ON p.vendor_id = v.id
            JOIN users u ON u.id = v.user_id
            WHERE ci.cart_id = $1 AND v.id = $2
        )
        SELECT
            vi.total_items_count,
            vi.item_id,
            vi.product_id,
            vi.cart_id,
            vi.added_at,
            vi.quantity,
            vi.ci_created_at,
            vi.ci_updated_at,
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
        ORDER BY vi.added_at DESC
        LIMIT $3 OFFSET $4;
    `

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	rows, err := m.DB.QueryContext(ctx, query, cartID, vendorID, filter.PageSize, (filter.Page-1)*filter.PageSize)
	if err != nil {
		return nil, Metadata{}, fmt.Errorf("failed to query cart items: %w", err)
	}
	defer rows.Close()

	var (
		cartItems    []*VendorGroupCartItem
		totalRecords int
	)

	for rows.Next() {
		var (
			item      VendorGroupCartItem
			itemsJSON []byte
		)

		err := rows.Scan(
			&totalRecords, &item.ID, &item.ProductID, &item.CartID,
			&item.AddedAt, &item.Quantity, &item.CreatedAt, &item.UpdatedAt, &itemsJSON,
		)
		if err != nil {
			return nil, Metadata{}, fmt.Errorf("failed to scan row: %w", err)
		}

		if err := json.Unmarshal(itemsJSON, &item.Product); err != nil {
			return nil, Metadata{}, fmt.Errorf("failed to unmarshal product JSON: %w", err)
		}

		cartItems = append(cartItems, &item)
	}

	if err := rows.Err(); err != nil {
		return nil, Metadata{}, fmt.Errorf("rows error: %w", err)
	}

	metadata := calculateMetadata(totalRecords, filter.Page, filter.PageSize)
	return cartItems, metadata, nil
}

func (m *CartItemModel) SetItemQuantity(ctx context.Context, cartID, cartItemID string, quantity int) error {
	query := `UPDATE cart_items ci SET quantity = $1 WHERE ci.id = $2 AND ci.cart_id = $3`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	result, err := m.DB.ExecContext(ctx, query, quantity, cartItemID, cartID)

	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()

	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrRecordNotFound
	}
	return nil
}

func (m *CartItemModel) GetItemsByIDS(ctx context.Context, cartID string, ids []string) ([]*CartItem, error) {
	query := `
        SELECT
            id,
            cart_id,
            product_id,
            added_at,
            quantity,
            created_at,
            updated_at
        FROM
            cart_items
        WHERE
            id = ANY($1::text[]) AND cart_id = $2
    `

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	var cartItems []*CartItem

	rows, err := m.DB.QueryContext(ctx, query, pq.Array(ids), cartID)
	if err != nil {
		return nil, fmt.Errorf("failed to query cart items: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		cartItem := &CartItem{}

		err := rows.Scan(
			&cartItem.ID,
			&cartItem.CartID,
			&cartItem.ProductID,
			&cartItem.AddedAt,
			&cartItem.Quantity,
			&cartItem.CreatedAt,
			&cartItem.UpdatedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan cart item: %w", err)
		}

		cartItems = append(cartItems, cartItem)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error after iterating over rows: %w", err)
	}

	return cartItems, nil
}

func (m *OrderItemModel) GetItemByOrderID(ctx context.Context, orderID string) ([]*CartItem, error) {
	// SQL query to fetch OrderItems and their associated Products by orderID
	query := `
        SELECT
            oi.id, oi.order_id, oi.product_id, oi.quantity, oi.price, oi.created_at, oi.updated_at,
            p.id, p.name, p.description, p.stock_quantity, p.status, p.published,
            p.total_items_sold_count, p.vendor_id, p.discount, p.price, p.category_id,
            p.created_at, p.updated_at
        FROM
            order_items oi
        JOIN
            products p ON oi.product_id = p.id
        WHERE
            oi.order_id = $1
    `

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()
	// Execute the query
	rows, err := m.db.QueryContext(ctx, query, orderID)
	if err != nil {
		return nil, fmt.Errorf("failed to query order items: %w", err)
	}
	defer rows.Close()

	var cartItems []*CartItem
	for rows.Next() {
		var orderItem OrderItem
		var product Product

		// Scan the row into the OrderItem and Product structs
		err := rows.Scan(
			&orderItem.ID,
			&orderItem.OrderID,
			&orderItem.ProductID,
			&orderItem.Quantity,
			&orderItem.Price,
			&orderItem.CreatedAt,
			&orderItem.UpdatedAt,
			&product.ID,
			&product.Name,
			&product.Description,
			&product.StockQuantity,
			&product.Status,
			&product.Published,
			&product.TotalItemsSoldCount,
			&product.VendorID,
			&product.Discount,
			&product.Price,
			&product.CategoryID,
			&product.CreatedAt,
			&product.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan order item and product: %w", err)
		}

		// Assign the product to the order item
		orderItem.Product = product

		// Convert OrderItem to CartItem
		cartItem := &CartItem{
			ID:        orderItem.ID,
			ProductID: orderItem.ProductID,
			Quantity:  orderItem.Quantity,
			Price:     orderItem.Price,
			Product:   &orderItem.Product,
		}
		cartItems = append(cartItems, cartItem)
	}

	// Check for errors after iterating through rows
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return cartItems, nil
}
