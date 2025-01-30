package store

import (
	"context"
	"database/sql"
	"errors"
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

func (m *Wishlist) GetWishlistItems(ctx context.Context) {

}
