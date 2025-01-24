package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/devphaseX/buyr-api.git/internal/db"
	"github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

// User represents a user in the system.
type User struct {
	ID              string     `json:"id"`
	Email           string     `json:"email"`
	Password        password   `json:"-"`
	AvatarURL       string     `json:"avatar_url"`
	Role            string     `json:"role"`
	EmailVerifiedAt *time.Time `json:"email_verified_at"`
	IsActive        bool       `json:"is_active"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type password struct {
	plaintext *string
	hash      []byte
}

func (p *password) Set(plantextPassword string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(plantextPassword), 12)

	if err != nil {
		return err
	}

	p.plaintext = &plantextPassword
	p.hash = hash

	return nil
}

func (p *password) Matches(plaintextPassword string) (bool, error) {
	err := bcrypt.CompareHashAndPassword(p.hash, []byte(plaintextPassword))

	if err != nil {
		switch {
		case errors.Is(err, bcrypt.ErrMismatchedHashAndPassword):
			return false, nil
		default:
			return false, err
		}
	}

	return true, nil
}

// NormalUser represents a normal user in the system.
type NormalUser struct {
	ID          string    `json:"id"`
	FirstName   string    `json:"first_name"`
	LastName    string    `json:"last_name"`
	PhoneNumber string    `json:"phone_number"`
	UserID      string    `json:"user_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	User        User      `json:"user"`
}

// VendorUser represents a vendor user in the system.
type VendorUser struct {
	ID               string     `json:"id"`
	BusinessName     string     `json:"business_name"`
	BusinessAddress  string     `json:"business_address"`
	ContactNumber    string     `json:"contact_number"`
	UserID           string     `json:"user_id"`
	ApprovedAt       *time.Time `json:"approved_at"`
	SuspendedAt      *time.Time `json:"suspended_at"`
	CreatedByAdminID string     `json:"created_by_admin_id"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	User             User       `json:"user"`
}

// UserModel represents the database model for users.
type UserModel struct {
	db *sql.DB
}

// NewUserModel creates a new UserModel instance.
func NewUserModel(db *sql.DB) *UserModel {
	return &UserModel{db}
}

// createUser inserts a new user into the database.
func createUser(ctx context.Context, tx *sql.Tx, user *User) error {
	query := `
		INSERT INTO users(id, email, password, role)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, updated_at
	`

	id := db.GenerateULID()
	args := []any{id, user.Email, pq.Array(user.Password.hash), user.Role}

	ctx, cancel := context.WithTimeout(ctx, QueryDurationTimeout)
	defer cancel()

	err := tx.QueryRowContext(ctx, query, args).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		var pgErr *pq.Error
		if errors.As(err, &pgErr) {
			if pgErr.Code == "23505" && pgErr.Constraint == "unique_email" {
				return ErrDuplicateEmail
			}
		}
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

// createNormalUser inserts a new normal user into the database.
func createNormalUser(ctx context.Context, tx *sql.Tx, user *NormalUser) error {
	query := `
		INSERT INTO normal_users(id, first_name, last_name, phone_number, user_id)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, user_id, created_at, updated_at
	`

	id := db.GenerateULID()
	args := []any{id, user.FirstName, user.LastName, user.PhoneNumber, user.User.ID}

	ctx, cancel := context.WithTimeout(ctx, QueryDurationTimeout)
	defer cancel()

	err := tx.QueryRowContext(ctx, query, args).Scan(&user.ID, &user.UserID, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create normal user: %w", err)
	}

	return nil
}

// createVendorUser inserts a new vendor user into the database.
func createVendorUser(ctx context.Context, tx *sql.Tx, user *VendorUser) error {
	query := `
		INSERT INTO vendor_users(id, business_name, business_address, contact_number, user_id, created_by_admin_id)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, user_id, created_at, updated_at
	`

	id := db.GenerateULID()
	args := []any{id, user.BusinessName, user.BusinessAddress, user.ContactNumber, user.User.ID, user.CreatedByAdminID}

	ctx, cancel := context.WithTimeout(ctx, QueryDurationTimeout)
	defer cancel()

	err := tx.QueryRowContext(ctx, query, args).Scan(&user.ID, &user.UserID, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create vendor user: %w", err)
	}

	return nil
}

// CreateNormalUser creates a new normal user and associated user record.
func (s *UserModel) CreateNormalUser(ctx context.Context, user *NormalUser) error {
	return withTrx(s.db, ctx, func(tx *sql.Tx) error {
		if err := createUser(ctx, tx, &user.User); err != nil {
			return err
		}

		if err := createNormalUser(ctx, tx, user); err != nil {
			return err
		}

		return nil
	})
}

// CreateVendorUser creates a new vendor user and associated user record.
func (s *UserModel) CreateVendorUser(ctx context.Context, user *VendorUser) error {
	return withTrx(s.db, ctx, func(tx *sql.Tx) error {
		if err := createUser(ctx, tx, &user.User); err != nil {
			return err
		}

		if err := createVendorUser(ctx, tx, user); err != nil {
			return err
		}

		return nil
	})
}
