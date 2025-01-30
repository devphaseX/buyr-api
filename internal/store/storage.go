package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var (
	QueryTimeoutDuration      = time.Second * 5
	ErrRecordNotFound         = errors.New("record not found")
	ErrDuplicateEmail         = errors.New("email already exists")
	ErrSessionCannotBeExtends = errors.New("session cannot be extended")
	ErrUnknownUserRole        = fmt.Errorf("unknown user role")
)

type Storage struct {
	Users     UserStorage
	Sessions  SessionStore
	Category  CategoryStore
	Products  ProductStore
	Reviews   ReviewStore
	Carts     CartStore
	CartItems CartItemStore
	Wishlists WhitelistStore
}

func NewStorage(db *sql.DB) *Storage {
	return &Storage{
		Users:     NewUserModel(db),
		Sessions:  NewSessionModel(db),
		Category:  NewCategoryModel(db),
		Products:  NewProductModel(db),
		Reviews:   NewReviewModel(db),
		Carts:     NewCartModel(db),
		CartItems: NewCartItemModel(db),
		Wishlists: NewWishlistModel(db),
	}
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
