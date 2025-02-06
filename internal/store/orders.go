package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/devphaseX/buyr-api.git/internal/db"
)

type OrderStatus string

var (
	PendingOrderStatus    OrderStatus = "pending"
	ProcessingOrderStatus OrderStatus = "processing"
	ShippedOrderStatus    OrderStatus = "shipped"
	DeliveredOrderStatus  OrderStatus = "delivered"
	CancelledOrderStatus  OrderStatus = "cancelled"
)

type Order struct {
	ID                string      `json:"id"`
	UserID            string      `json:"user_id"`
	TotalAmount       float64     `json:"total_amount"`
	PromoCode         string      `json:"promo_code"`
	Discount          float64     `json:"discount"`
	Status            OrderStatus `json:"status"`
	Paid              bool        `json:"paid"`
	ShippingAddressId string      `json:"shipping_address_id"`
	PaymentMethod     string      `json:"payment_method"`
	CreatedAt         time.Time   `json:"created_at"`
	UpdatedAt         time.Time   `json:"updated_at"`
}

type OrderItem struct {
	ID         string    `json:"id"`
	OrderID    string    `json:"order_id"`
	ProductID  string    `json:"product_id"`
	Quantity   int       `json:"quantity"`
	CartItemID string    `json:"-"`
	Price      float64   `json:"price"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	Product    Product   `json:"product"`
}

type OrderStore interface {
	Create(ctx context.Context, productStore ProductStore, order *Order, cartItems []*CartItem) error
	GetUserOrderByID(ctx context.Context, userId, id string) (*Order, error)
	GetOrderByID(ctx context.Context, id string) (*Order, error)
	UpdateStatus(ctx context.Context, orderID string, status OrderStatus) error
	GetAbandonedOrders(ctx context.Context, cutoffTime time.Time) ([]Order, error)
}

type OrderItemStore interface {
	GetItemsByOrderID(ctx context.Context, orderID string) ([]*OrderItem, error)
}

type OrderModel struct {
	db *sql.DB
}

func NewOrderModel(db *sql.DB) OrderStore {
	return &OrderModel{db}
}

type OrderItemModel struct {
	db *sql.DB
}

func NewOrderItemModel(db *sql.DB) OrderItemStore {
	return &OrderItemModel{db}
}

func createOrder(ctx context.Context, tx *sql.Tx, order *Order) error {
	order.ID = db.GenerateULID()
	query := `INSERT INTO orders(id, user_id, total_amount, promo_code, shipping_address_id, status, paid, payment_method)
			 VALUES($1, $2, $3, $4, $5, $6, $7) RETURNING created_at, updated_at
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)

	defer cancel()

	args := []any{order.ID, order.UserID, order.TotalAmount, order.PromoCode, order.ShippingAddressId, order.Status, order.Paid, order.PaymentMethod}
	err := tx.QueryRowContext(ctx, query, args...).Scan(&order.CreatedAt, &order.UpdatedAt)

	if err != nil {
		return nil
	}

	return nil
}

func (m *OrderModel) Create(ctx context.Context, productStore ProductStore, order *Order, cartItems []*CartItem) error {
	return withTrx(m.db, ctx, func(tx *sql.Tx) error {
		if err := createOrder(ctx, tx, order); err != nil {
			return err
		}

		if err := createOrderItems(ctx, tx, order.ID, cartItems); err != nil {
			return err
		}

		return nil
	})
}

func createOrderItems(ctx context.Context, tx *sql.Tx, orderID string, cartItems []*CartItem) error {

	query := `INSERT INTO order_items(id, order_id, product_id,cart_item_id, quantity, price)
			VALUES ($1, $2, $3, $4, $5, $6) RETURNING created_at, updated_at`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	var wg sync.WaitGroup

	errCh := make(chan error, len(cartItems))

	for _, item := range cartItems {
		wg.Add(1)

		go func(orderItem *OrderItem) {
			defer wg.Done()

			orderItem.ID = db.GenerateULID()

			args := []any{orderItem.ID, orderItem.OrderID, orderItem.ProductID,
				orderItem.CartItemID, orderItem.Quantity, orderItem.Price}

			err := tx.QueryRowContext(ctx, query, args...).Scan(&orderItem.CreatedAt, &orderItem.UpdatedAt)

			if err != nil {
				errCh <- err
				return
			}

		}(&OrderItem{
			OrderID:    orderID,
			ProductID:  item.ProductID,
			Quantity:   item.Quantity,
			Price:      item.Price,
			CartItemID: item.ID,
		})
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

func (m *OrderModel) GetUserOrderByID(ctx context.Context, userId, id string) (*Order, error) {
	query := `SELECT
				id,
				user_id,
				total_amount,
				promo_code,
				discount,
				status,
				paid,
				payment_method,
				created_at,
				updated_at
				FROM orders
			 	WHERE id = $1 AND user_id = $2
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)

	defer cancel()

	order := &Order{}

	var discount sql.NullFloat64
	var promoCode sql.NullString
	var paymentMethod sql.NullString

	err := m.db.QueryRowContext(ctx, query, id, userId).Scan(&order.ID, &order.UserID, &order.TotalAmount, &promoCode, &discount, &order.Status, &order.Paid,
		&paymentMethod, &order.CreatedAt, &order.UpdatedAt)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound

		default:
			return nil, err
		}
	}

	if discount.Valid {
		order.Discount = discount.Float64
	}

	if promoCode.Valid {
		order.PromoCode = promoCode.String
	}

	if paymentMethod.Valid {
		order.PaymentMethod = paymentMethod.String
	}

	return order, nil
}

func (m *OrderModel) GetOrderByID(ctx context.Context, id string) (*Order, error) {
	query := `SELECT
				id,
				user_id,
				total_amount,
				promo_code,
				discount,
				status,
				paid,
				payment_method,
				created_at,
				updated_at
				FROM orders
			 	WHERE id = $1
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)

	defer cancel()

	order := &Order{}

	err := m.db.QueryRowContext(ctx, query, id).Scan(&order.ID, &order.UserID, &order.TotalAmount, &order.PromoCode, &order.Discount, &order.Status, &order.Paid,
		&order.PaymentMethod, &order.CreatedAt, &order.UpdatedAt)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound

		default:
			return nil, err
		}
	}

	return order, nil
}

func (m *OrderModel) UpdateStatus(ctx context.Context, orderID string, status OrderStatus) error {
	query := `UPDATE orders SET status = $1 WHERE id = $2`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	result, err := m.db.ExecContext(ctx, query, status, orderID)

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

func (m *OrderItemModel) GetItemsByOrderID(ctx context.Context, orderID string) ([]*OrderItem, error) {
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

	var orderItems []*OrderItem
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

		orderItems = append(orderItems, &orderItem)
	}

	// Check for errors after iterating through rows
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return orderItems, nil
}

func (s *OrderModel) GetAbandonedOrders(ctx context.Context, cutoffTime time.Time) ([]Order, error) {
	query := `
		SELECT id, promo_code
		FROM orders
		WHERE status = 'pending' AND created_at < $1
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()
	rows, err := s.db.QueryContext(ctx, query, cutoffTime)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []Order
	for rows.Next() {
		var order Order
		if err := rows.Scan(&order.ID, &order.PromoCode); err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to retrieve abandonedOrder: %w", err)
	}

	return orders, nil
}
