package store

import (
	"database/sql"
	"time"
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
