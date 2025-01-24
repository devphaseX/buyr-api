package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var (
	QueryDurationTimeout = time.Second * 5
	ErrRecordNotFound    = errors.New("record not found")
	ErrDuplicateEmail    = errors.New("email already exists")
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
