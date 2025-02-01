package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/devphaseX/buyr-api.git/internal/db"
)

type PaymentStatus string

var (
	CompletedPaymentStatus PaymentStatus = "completed"
	FailedPaymentStatus    PaymentStatus = "failed"
)

type Payment struct {
	ID            string        `json:"id"`
	OrderID       string        `json:"order_id"`
	PaymentMethod string        `json:"payment_method"`
	Amount        float64       `json:"amount"`
	Status        PaymentStatus `json:"status"`
	TransactionID string        `json:"transaction_id"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
}

type PaymentStore interface {
	Create(ctx context.Context, payment *Payment) error
}

type PaymentModel struct {
	db *sql.DB
}

func NewPaymentModel(db *sql.DB) PaymentStore {
	return &PaymentModel{db}
}

func createPayment(ctx context.Context, tx *sql.Tx, payment *Payment) error {
	payment.ID = db.GenerateULID()
	query := `INSERT INTO payments (id, order_id, payment_method, amount, status, transaction_id)
			  VALUES ($1, $2, $3, $4, $5, $6) RETURNING created_at, updated_at
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)

	defer cancel()

	args := []any{payment.ID, payment.OrderID, payment.PaymentMethod,
		payment.Amount, payment.Status, payment.TransactionID}
	err := tx.QueryRowContext(ctx, query, args...).Scan(&payment.CreatedAt, &payment.UpdatedAt)

	if err != nil {
		return err
	}

	return nil
}

func setProcessingOrder(ctx context.Context, tx *sql.Tx, orderID string, paid bool) error {
	query := `UPDATE orders SET status = $2, paid = $3 WHERE id = $1`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	status := CancelledOrderStatus

	if paid {
		status = ProcessingOrderStatus
	}

	result, err := tx.ExecContext(ctx, query, orderID, status, paid)

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

func (m *PaymentModel) Create(ctx context.Context, payment *Payment) error {
	return withTrx(m.db, ctx, func(tx *sql.Tx) error {
		if err := createPayment(ctx, tx, payment); err != nil {
			return err
		}

		if err := setProcessingOrder(ctx, tx, payment.OrderID, payment.Status == CompletedPaymentStatus); err != nil {
			return err
		}

		if payment.Status == CompletedPaymentStatus {
			err := clearOrderedCartItems(ctx, tx, payment.OrderID)

			if err != nil {
				return err
			}
		}

		return nil
	})
}
