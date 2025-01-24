package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/devphaseX/buyr-api.git/internal/db"
	"github.com/lib/pq"
)

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
	plaintext string
	hash      []byte
}

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

type AdminUser struct {
	ID                   string    `json:"id"`
	FirstName            string    `json:"first_name"`
	LastName             string    `json:"last_name"`
	UserID               string    `json:"user_id"`
	AuthSecret           string    `json:"auth_secret"`
	TwoFactorAuthEnabled bool      `json:"two_factor_auth_enabled"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
	User                 User      `json:"user"`
}

type UserModel struct {
	db *sql.DB
}

func NewUserModel(db *sql.DB) *UserModel {
	return &UserModel{db}
}

func createUser(ctx context.Context, tx *sql.Tx, user *User) error {
	query := `
		INSERT INTO users(id, email,password, role)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, updated_at
	`

	id := db.GenerateULID()
	args := []any{id, user.Email, pq.Array(user.Password), user.Role}

	ctx, cancel := context.WithTimeout(ctx, QueryDurationTimeout)
	defer cancel()
	err := tx.QueryRowContext(ctx, query, args).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		// Check for unique constraint violation (duplicate email)
		var pgErr *pq.Error

		switch {
		case errors.As(err, &pgErr):
			if pgErr.Code == "23505" && pgErr.Constraint == "unique_email" {
				return ErrDuplicateEmail
			}

		case errors.Is(err, sql.ErrNoRows):
			return ErrRecordNotFound
		}
	}

	return nil
}
