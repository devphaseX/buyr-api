package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	QueryTimeoutDuration      = time.Second * 5
	ErrRecordNotFound         = errors.New("record not found")
	ErrDuplicateEmail         = errors.New("the email address is already in use. Please use a different email.")
	ErrSessionCannotBeExtends = errors.New("session cannot be extended")
	ErrUnknownUserRole        = fmt.Errorf("unknown user role")
)

type Storage struct {
	Users      UserStorage
	Sessions   SessionStore
	Category   CategoryStore
	Products   ProductStore
	Reviews    ReviewStore
	Carts      CartStore
	CartItems  CartItemStore
	Wishlists  WhitelistStore
	AuditLogs  AuditEventStore
	Orders     OrderStore
	OrderItems OrderItemStore
	Payments   PaymentStore
	Promos     PromoStore
	Address    AddressStore
	OptionType OptionTypeStore
}

func NewStorage(db *sql.DB) *Storage {
	return &Storage{
		Users:      NewUserModel(db),
		Sessions:   NewSessionModel(db),
		Category:   NewCategoryModel(db),
		Products:   NewProductModel(db),
		Reviews:    NewReviewModel(db),
		Carts:      NewCartModel(db),
		CartItems:  NewCartItemModel(db),
		Wishlists:  NewWishlistModel(db),
		AuditLogs:  NewAuditEventModel(db),
		Orders:     NewOrderModel(db),
		OrderItems: NewOrderItemModel(db),
		Payments:   NewPaymentModel(db),
		Promos:     NewPromoModel(db),
		Address:    NewAddressModel(db),
		OptionType: NewOptionTypeModel(db),
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

func buildPlaceholders(n int) string {
	placeholders := make([]string, n)
	for i := 0; i < n; i++ {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}
	return strings.Join(placeholders, ", ")
}

func wrapDatabaseError(err error) error {
	return fmt.Errorf("%w: %v", ErrDatabase, err)
}
