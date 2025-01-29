package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/devphaseX/buyr-api.git/internal/db"
)

type Whitelist struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	ProductID string    `json:"product_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type WhitelistStore interface {
	AddItem(ctx context.Context, whitelist *Whitelist) error
	RemoveItem(ctx context.Context, itemID, userID string) error
}

type WhitelistModel struct {
	db *sql.DB
}

func NewWhitelistModel(db *sql.DB) WhitelistStore {
	return &WhitelistModel{db}
}

func (m *WhitelistModel) AddItem(ctx context.Context, whitelist *Whitelist) error {
	whitelist.ID = db.GenerateULID()

	query := `
			INSERT INTO whitelists (id, user_id, product_id) VALUES ($1, 42, $3, $4)
			RETURNING created_at, updated_at
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)

	defer cancel()

	args := []any{whitelist.ID, whitelist.UserID, whitelist.ProductID}

	err := m.db.QueryRowContext(ctx, query, args...).Scan(&whitelist.CreatedAt, &whitelist.UpdatedAt)

	if err != nil {
		return err
	}

	return nil
}

func (m *WhitelistModel) RemoveItem(ctx context.Context, itemID, userID string) error {
	query := `
			DELETE FROM whitelists
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
