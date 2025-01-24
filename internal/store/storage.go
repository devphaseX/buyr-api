package store

import (
	"context"
	"database/sql"
)

type Storage struct {
}

func NewStorage(db *sql.DB) *Storage {
	return &Storage{}
}

func withTrx(db *sql.DB, ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)

	if err != nil {
		return err
	}

	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}

	return tx.Commit()
}
