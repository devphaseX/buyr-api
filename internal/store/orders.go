package store

import (
	"context"
	"database/sql"
	"errors"
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
	ID            string      `json:"id"`
	UserID        string      `json:"user_id"`
	TotalAmount   float64     `json:"total_amount"`
	PromoCode     string      `json:"promo_code"`
	Discount      float64     `json:"discount"`
	Status        OrderStatus `json:"status"`
	Paid          bool        `json:"paid"`
	PaymentMethod string      `json:"payment_method"`
	CreatedAt     time.Time   `json:"created_at"`
	UpdatedAt     time.Time   `json:"updated_at"`
}

type OrderItem struct {
	ID        string    `json:"id"`
	OrderID   string    `json:"order_id"`
	ProductID string    `json:"product_id"`
	Quantity  int       `json:"quantity"`
	Price     float64   `json:"price"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type OrderStore interface {
	Create(ctx context.Context, productStore ProductStore, order *Order, cartItems []*CartItem) error
	GetByID(ctx context.Context, userId, id string) (*Order, error)
	UpdateStatus(ctx context.Context, orderID string, status OrderStatus) error
}

type OrderItemStore interface {
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
	query := `INSERT INTO orders(id, user_id, total_amount, promo_code, status, paid, payment_method)
			 VALUES($1, $2, $3, $4, $5, $6, $7) RETURNING created_at, updated_at
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)

	defer cancel()

	args := []any{order.ID, order.UserID, order.TotalAmount, order.PromoCode, order.Status, order.Paid, order.PaymentMethod}
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

	query := `INSERT INTO order_items(id, order_id, product_id, quantity, price)
			VALUES ($1, $2, $3, $4, $5) RETURNING created_at, updated_at`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	var wg sync.WaitGroup

	errCh := make(chan error, len(cartItems))

	for _, item := range cartItems {
		wg.Add(1)

		go func(orderItem *OrderItem) {
			defer wg.Done()

			orderItem.ID = db.GenerateULID()

			args := []any{orderItem.ID, orderItem.OrderID, orderItem.ProductID, orderItem.Quantity, orderItem.Quantity}

			err := tx.QueryRowContext(ctx, query, args...).Scan(orderItem.CreatedAt, orderItem.UpdatedAt)

			if err != nil {
				errCh <- err
				return
			}

		}(&OrderItem{
			OrderID:   orderID,
			ProductID: item.ProductID,
			Quantity:  item.Quantity,
			Price:     item.Price,
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

func (m *OrderModel) GetByID(ctx context.Context, userId, id string) (*Order, error) {
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

	err := m.db.QueryRowContext(ctx, query, userId, id).Scan(&order.ID, &order.UserID, &order.TotalAmount, &order.PromoCode, &order.Discount, &order.Status, &order.Paid,
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
	query := `UPDATE orders SET status = $1 WHERE id = $1`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	result, err := m.db.ExecContext(ctx, query, orderID)

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
