package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var (
	ErrPromoNotFound          = errors.New("promo code not found")
	ErrPromoExpired           = errors.New("promo code has expired")
	ErrPromoUsageLimitReached = errors.New("promo code has reached its usage limit")
	ErrMinPurchaseNotMet      = errors.New("minimum purchase amount not met")
	ErrUserNotAllowed         = errors.New("promo code is not valid for this user")
	ErrDatabase               = errors.New("database error")
)

type DiscountType string

var (
	PercentDiscountType      DiscountType = "percent"
	FixedDiscountType        DiscountType = "fixed"
	FreeShippingDiscountType DiscountType = "free_shipping"
)

type Promo struct {
	ID                string       `json:"id"`
	Code              string       `json:"code"`
	DiscountType      DiscountType `json:"discount_type"`
	DiscountValue     float64      `json:"discount_value"`
	MinPurchaseAmount float64      `json:"min_purchase_amount"`
	MaxUses           int          `json:"max_uses"` //0 for unlimited
	UsedCount         int          `json:"used_count"`
	UserSpecific      bool         `json:"user_specific"` // Whether the promo is restricted to specific users
	ExpiredAt         time.Time    `json:"expired_at"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type UserPromoUsage struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	PromoID   string `json:"promo_id"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

type PromoUserRestriction struct {
	ID        string `json:"id"`
	PromoID   string `json:"promo_id"`
	UserID    string `json:"user_id"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

type PromoStore interface {
	FindByCode(ctx context.Context, code string) (*Promo, error)
	IncrementUsage(ctx context.Context, promoID string) error
	ReleaseUsage(ctx context.Context, code string) error
	ValidatePromoCode(ctx context.Context, code string, userID string, orderTotal float64) (*Promo, float64, error)
	IsUserAllowed(ctx context.Context, promoID string, userID string) (bool, error)
}

type PromoModel struct {
	db *sql.DB
}

func NewPromoModel(db *sql.DB) PromoStore {
	return &PromoModel{db}
}

func (m *PromoModel) FindByCode(ctx context.Context, code string) (*Promo, error) {
	query := `
		SELECT id, code, discount_type, discount_value, min_purchase_amount, max_uses, used_count, expired_at, user_specific, created_at, updated_at
		FROM promos
		WHERE code = $1`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	var p Promo

	err := m.db.QueryRowContext(ctx, query, code).Scan(&p.ID, &p.Code, &p.DiscountType, &p.MinPurchaseAmount, &p.MaxUses,
		&p.UsedCount, &p.ExpiredAt, &p.UserSpecific, &p.CreatedAt, &p.UpdatedAt)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, fmt.Errorf("failed to retrieve promo: %w", err)
		}
	}

	return &p, nil
}

func (s *PromoModel) ReleaseUsage(ctx context.Context, code string) error {
	query := `
		UPDATE promos
		SET used_count = used_count - 1
		WHERE code = $1 AND used_count > 0
	`
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()
	_, err := s.db.ExecContext(ctx, query, code)
	return err
}

func (s *PromoModel) ValidatePromoCode(ctx context.Context, code string, userID string, orderTotal float64) (*Promo, float64, error) {
	// Step 1: Find the promo code
	promo, err := s.FindByCode(ctx, code)
	if err != nil {
		if errors.Is(err, ErrPromoNotFound) {
			return nil, 0, ErrPromoNotFound
		}
		return nil, 0, err
	}

	// Step 2: Validate the promo code
	if time.Now().After(promo.ExpiredAt) {
		return nil, 0, ErrPromoExpired
	}
	if promo.MaxUses > 0 && promo.UsedCount >= promo.MaxUses {
		return nil, 0, ErrPromoUsageLimitReached
	}
	if promo.MinPurchaseAmount > 0 && orderTotal < promo.MinPurchaseAmount {
		return nil, 0, fmt.Errorf("%w: minimum purchase amount of %.2f required", ErrMinPurchaseNotMet, promo.MinPurchaseAmount)
	}
	if promo.UserSpecific {
		allowed, err := s.IsUserAllowed(ctx, promo.ID, userID)
		if err != nil {
			return nil, 0, err
		}
		if !allowed {
			return nil, 0, ErrUserNotAllowed
		}
	}

	// Step 3: Calculate the discounted total
	discountedTotal := orderTotal
	switch promo.DiscountType {
	case "percent":
		discountedTotal -= orderTotal * (promo.DiscountValue / 100)
	case "fixed":
		discountedTotal -= promo.DiscountValue
	case "free_shipping":
		// Handle free shipping logic (if applicable)
	}

	return promo, discountedTotal, nil
}

func (m *PromoModel) IncrementUsage(ctx context.Context, promoID string) error {
	query := `
			UPDATE promos
			SET used_count = used_count + 1
			WHERE id = $1
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	_, err := m.db.ExecContext(ctx, query, promoID)

	return err
}

func (m *PromoModel) IsUserAllowed(ctx context.Context, promoID string, userID string) (bool, error) {
	query := `
			SELECT EXISTS (
				SELECT 1
				FROM promo_user_restrictions
				WHERE promo_id = $1 AND user_id = $2
			)
		`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)

	defer cancel()
	var exists bool

	err := m.db.QueryRowContext(ctx, query, promoID, userID).Scan(&exists)

	if err != nil {
		return false, fmt.Errorf("failed to check user restriction: %w", err)
	}

	return exists, nil
}

func recordUsage(ctx context.Context, tx *sql.Tx, promoID string, userID string, orderID string) error {
	query := `
		INSERT INTO user_promo_usage (user_id, promo_id, order_id)
		VALUES ($1, $2, $3)
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	_, err := tx.ExecContext(ctx, query, userID, promoID, orderID)
	return err
}

func (r *PromoModel) ReservePromo(ctx context.Context, promoID string) error {
	query := `
		UPDATE promos
		SET reserved_count = reserved_count + 1
		WHERE id = $1 AND (max_uses IS NULL OR max_uses = 0 OR used_count + reserved_count < max_uses)
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	result, err := r.db.ExecContext(ctx, query, promoID)
	if err != nil {
		return wrapDatabaseError(err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return wrapDatabaseError(err)
	}
	if rowsAffected == 0 {
		return ErrPromoUsageLimitReached
	}

	return nil
}

func (r *PromoModel) FinalizePromoUsage(ctx context.Context, promoID string) error {
	query := `
		UPDATE promos
		SET used_count = used_count + 1, reserved_count = reserved_count - 1
		WHERE id = $1
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)

	defer cancel()
	_, err := r.db.ExecContext(ctx, query, promoID)
	return wrapDatabaseError(err)
}

func (r *PromoModel) ReleasePromoReservation(ctx context.Context, promoID string) error {
	query := `
		UPDATE promos
		SET reserved_count = reserved_count - 1
		WHERE id = $1
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)

	defer cancel()
	_, err := r.db.ExecContext(ctx, query, promoID)
	return wrapDatabaseError(err)
}
