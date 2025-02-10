package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/devphaseX/buyr-api.git/internal/db"
	"github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

type Role string

var (
	UserRole   Role = "user"
	VendorRole Role = "vendor"
	AdminRole  Role = "admin"
)

type AdminLevel string

var (
	AdminLevelSuper   AdminLevel = "super"
	AdminLevelManager AdminLevel = "manager"
	AdminLevelSupport AdminLevel = "support"
	AdminLevelNone    AdminLevel = "none"
)

var adminLevelWeights = map[AdminLevel]int{
	AdminLevelSuper:   3,
	AdminLevelManager: 2,
	AdminLevelSupport: 1,
	AdminLevelNone:    0,
}

func (a AdminLevel) HasAccessTo(required AdminLevel) bool {
	return adminLevelWeights[a] >= adminLevelWeights[required]
}

func (a AdminLevel) CanModifyAdminLevel(b AdminLevel) bool {
	return a.HasAccessTo(b)
}

func (a AdminLevel) GetRank() int {
	return adminLevelWeights[a]
}

type TwoFactorType string

const (
	TotpFactorType TwoFactorType = "totp"
)

var AnonymousUser = &User{}

// User represents a user in the system.
type User struct {
	ID                   string     `json:"id"`
	Email                string     `json:"email"`
	Password             password   `json:"-"`
	AvatarURL            string     `json:"avatar_url"`
	Role                 Role       `json:"role"`
	EmailVerifiedAt      *time.Time `json:"email_verified_at"`
	RecoveryCodes        []string   `json:"-"`
	AuthSecret           string     `json:"-"`
	ForcePasswordChange  bool       `json:"force_password_change"`
	TwoFactorAuthEnabled bool       `json:"-"`
	Version              int        `json:"-"`
	IsActive             bool       `json:"is_active"`
	Disabled             bool       `json:"-"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

func (u *User) IsAnonymous() bool {
	return u == AnonymousUser
}

type Account struct {
	UserID            string `json:"user_id"`
	Type              string `json:"type"`
	Provider          string `json:"provider"`
	ProviderAccountID string `json:"-"`
	RefreshToken      string `json:"-"`
	AccessToken       string `json:"-"`
	ExpiresAt         int64  `json:"-"`
	TokenType         string `json:"-"`
	Scope             string `json:"-"`
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

func (u *NormalUser) MarshalJSON() ([]byte, error) {
	return json.Marshal(MarshalNormalUser(*u))
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
	City             string     `json:"city"`
	Country          string     `json:"country"`
	CreatedByAdminID string     `json:"created_by_admin_id"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	User             User       `json:"user"`
}

func (u *VendorUser) MarshalJSON() ([]byte, error) {
	return json.Marshal(MarshalVendorUser(*u))
}

// NormalUser represents a normal user in the system.
type AdminUser struct {
	ID         string     `json:"id"`
	FirstName  string     `json:"first_name"`
	LastName   string     `json:"last_name"`
	UserID     string     `json:"user_id"`
	AdminLevel AdminLevel `json:"admin_level"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	User       User       `json:"user"`
}

func (u *AdminUser) MarshalJSON() ([]byte, error) {
	return json.Marshal(MarshalAdminUser(*u))
}

// FlattenedUser combines all user types into a single struct
type FlattenedUser struct {
	// Base User fields
	ID                   string     `json:"id"`
	Email                string     `json:"email"`
	AvatarURL            string     `json:"avatar_url"`
	Role                 Role       `json:"role"`
	EmailVerifiedAt      *time.Time `json:"email_verified_at"`
	AuthSecret           string     `json:"-"`
	TwoFactorAuthEnabled bool       `json:"-"`
	IsActive             bool       `json:"is_active"`
	UserCreatedAt        time.Time  `json:"user_created_at"`
	UserUpdatedAt        time.Time  `json:"user_updated_at"`

	OriginalUserID string `json:"original_user_id"`

	AdminLevel AdminLevel `json:"admin_level,omitempty"`
	// Normal User fields
	FirstName   string `json:"first_name,omitempty"`
	LastName    string `json:"last_name,omitempty"`
	PhoneNumber string `json:"phone_number,omitempty"`

	// Vendor User fields
	BusinessName     string     `json:"business_name,omitempty"`
	BusinessAddress  string     `json:"business_address,omitempty"`
	ContactNumber    string     `json:"contact_number,omitempty"`
	ApprovedAt       *time.Time `json:"approved_at,omitempty"`
	SuspendedAt      *time.Time `json:"suspended_at,omitempty"`
	CreatedByAdminID string     `json:"created_by_admin_id,omitempty"`
}

// Marshal functions for each user type
func MarshalNormalUser(normalUser NormalUser) *FlattenedUser {
	return &FlattenedUser{
		// Base User fields
		ID:                   normalUser.User.ID,
		Email:                normalUser.User.Email,
		AvatarURL:            normalUser.User.AvatarURL,
		Role:                 normalUser.User.Role,
		EmailVerifiedAt:      normalUser.User.EmailVerifiedAt,
		AuthSecret:           normalUser.User.AuthSecret,
		TwoFactorAuthEnabled: normalUser.User.TwoFactorAuthEnabled,
		IsActive:             normalUser.User.IsActive,
		UserCreatedAt:        normalUser.User.CreatedAt,
		UserUpdatedAt:        normalUser.User.UpdatedAt,

		// User Type fields
		OriginalUserID: normalUser.ID,

		// Normal User specific fields
		FirstName:   normalUser.FirstName,
		LastName:    normalUser.LastName,
		PhoneNumber: normalUser.PhoneNumber,
	}
}

func MarshalVendorUser(vendorUser VendorUser) *FlattenedUser {
	return &FlattenedUser{
		// Base User fields
		ID:                   vendorUser.User.ID,
		Email:                vendorUser.User.Email,
		AvatarURL:            vendorUser.User.AvatarURL,
		Role:                 vendorUser.User.Role,
		EmailVerifiedAt:      vendorUser.User.EmailVerifiedAt,
		AuthSecret:           vendorUser.User.AuthSecret,
		TwoFactorAuthEnabled: vendorUser.User.TwoFactorAuthEnabled,
		IsActive:             vendorUser.User.IsActive,
		UserCreatedAt:        vendorUser.User.CreatedAt,
		UserUpdatedAt:        vendorUser.User.UpdatedAt,

		// User Type fields
		OriginalUserID: vendorUser.ID,

		// Vendor User specific fields
		BusinessName:     vendorUser.BusinessName,
		BusinessAddress:  vendorUser.BusinessAddress,
		ContactNumber:    vendorUser.ContactNumber,
		ApprovedAt:       vendorUser.ApprovedAt,
		SuspendedAt:      vendorUser.SuspendedAt,
		CreatedByAdminID: vendorUser.CreatedByAdminID,
	}
}

func MarshalAdminUser(adminUser AdminUser) *FlattenedUser {
	return &FlattenedUser{
		// Base User fields
		ID:                   adminUser.User.ID,
		Email:                adminUser.User.Email,
		AvatarURL:            adminUser.User.AvatarURL,
		Role:                 adminUser.User.Role,
		EmailVerifiedAt:      adminUser.User.EmailVerifiedAt,
		AuthSecret:           adminUser.User.AuthSecret,
		TwoFactorAuthEnabled: adminUser.User.TwoFactorAuthEnabled,
		IsActive:             adminUser.User.IsActive,
		UserCreatedAt:        adminUser.User.CreatedAt,
		UserUpdatedAt:        adminUser.User.UpdatedAt,

		// User Type fields
		OriginalUserID: adminUser.ID,
		AdminLevel:     adminUser.AdminLevel,

		// Admin User specific fields
		FirstName: adminUser.FirstName,
		LastName:  adminUser.LastName,
	}
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

func (p *password) IsSetPasswordEmpty() bool {
	return len(p.hash) == 0
}

type UserStorage interface {
	CreateNormalUser(context.Context, *NormalUser) error
	CreateVendorUser(ctx context.Context, user *VendorUser) error
	CreateAdminUser(ctx context.Context, user *AdminUser) error
	SetUserAccountAsActivate(ctx context.Context, user *User) error
	GetByID(ctx context.Context, userID string) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	UpdatePassword(ctx context.Context, user *User, password string) error
	EnableTwoFactorAuth(ctx context.Context, userID, authSecret string, recoveryCodes []string) error
	DisableTwoFactorAuth(ctx context.Context, userID string) error
	GetVendorUserByID(ctx context.Context, userID string) (*VendorUser, error)
	GetAdminUserByID(ctx context.Context, userID string) (*AdminUser, error)
	GetAdminByID(ctx context.Context, userID string) (*AdminUser, error)
	GetNormalUserByID(ctx context.Context, userID string) (*NormalUser, error)
	ChangeAdminLevel(ctx context.Context, AdminID string, NewLevel AdminLevel) error
	DisableUser(ctx context.Context, userID string) error
	EnableUser(ctx context.Context, userID string) error
	GetVendorByID(ctx context.Context, ID string) (*VendorUser, error)
	UpsertAccount(ctx context.Context, account *Account) error

	FlattenUser(ctx context.Context, user *User) (*FlattenedUser, error)
	ResetRecoveryCodes(context.Context, string, []string) error
	GetNormalUsers(ctx context.Context, filter PaginateQueryFilter) ([]*NormalUser, Metadata, error)
	GetVendorUsers(ctx context.Context, filter PaginateQueryFilter) ([]*VendorUser, Metadata, error)
	GetAdminUsers(ctx context.Context, filter PaginateQueryFilter) ([]*AdminUser, Metadata, error)
	GetUserAccountByUserID(ctx context.Context, userID string) (*Account, error)
	ChangePassword(ctx context.Context, user *User) error
	UpdateEmail(ctx context.Context, userID string, newEmail string) error
}

// UserModel represents the database model for users.
type UserModel struct {
	db *sql.DB
}

// NewUserModel creates a new UserModel instance.
func NewUserModel(db *sql.DB) UserStorage {
	return &UserModel{db}
}

// createUser inserts a new user into the database.
func createUser(ctx context.Context, tx *sql.Tx, user *User) error {
	query := `
		INSERT INTO users(id, email, avatar_url, password_hash, is_active, email_verified_at role, force_password_change, version)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at
	`

	if len(user.Password.hash) == 0 {
		user.Password.hash = []byte{}
	}

	id := db.GenerateULID()
	args := []any{id, user.Email, user.AvatarURL, user.Password.hash, user.IsActive, user.EmailVerifiedAt, user.Role, user.ForcePasswordChange, user.Version}

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	err := tx.QueryRowContext(ctx, query, args...).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)

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

func (s *UserModel) UpsertAccount(ctx context.Context, account *Account) error {
	query := `INSERT INTO accounts (
	user_id, type, provider, provider_account_id,
    access_token, refresh_token, expires_at, token_type, scope
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	ON CONFLICT (provider, provider_account_id)
	DO UPDATE SET
		access_token = EXCLUDED.access_token,
		expires_at = EXCLUDED.expires_at,
		token_type = EXCLUDED.token_type,
		scope = EXCLUDED.scope
		`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	args := []any{account.UserID, account.Type, account.Provider,
		account.ProviderAccountID, account.AccessToken, account.RefreshToken,
		account.ExpiresAt, account.TokenType, account.Scope}

	_, err := s.db.ExecContext(ctx, query, args...)

	if err != nil {
		return err
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

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	err := tx.QueryRowContext(ctx, query, args...).Scan(&user.ID, &user.UserID, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create normal user: %w", err)
	}

	return nil
}

// createVendorUser inserts a new vendor user into the database.
func createVendorUser(ctx context.Context, tx *sql.Tx, user *VendorUser) error {
	query := `
		INSERT INTO vendor_users(id, business_name, business_address, contact_number, user_id,city, country, created_by_admin_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, user_id, created_at, updated_at
	`

	id := db.GenerateULID()
	args := []any{id, user.BusinessName, user.BusinessAddress, user.ContactNumber, user.User.ID, user.City, user.Country, user.CreatedByAdminID}

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	err := tx.QueryRowContext(ctx, query, args...).Scan(&user.ID, &user.UserID, &user.CreatedAt, &user.UpdatedAt)
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

func createAdminUser(ctx context.Context, tx *sql.Tx, user *AdminUser) error {
	query := `
		INSERT INTO admin_users(id, first_name, last_name,admin_level, user_id)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, user_id, created_at, updated_at
	`

	id := db.GenerateULID()
	args := []any{id, user.FirstName, user.LastName, user.AdminLevel, user.User.ID}

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	err := tx.QueryRowContext(ctx, query, args...).Scan(&user.ID, &user.UserID, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create an admin user: %w", err)
	}

	return nil
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

func (s *UserModel) CreateAdminUser(ctx context.Context, user *AdminUser) error {
	return withTrx(s.db, ctx, func(tx *sql.Tx) error {
		if err := createUser(ctx, tx, &user.User); err != nil {
			return err
		}

		if err := createAdminUser(ctx, tx, user); err != nil {
			return err
		}

		return nil
	})
}

func (s *UserModel) GetByID(ctx context.Context, userID string) (*User, error) {
	query := `SELECT id, email, password_hash,force_password_change,
			  avatar_url, role, email_verified_at, version,
			  is_active, two_factor_auth_enabled, auth_secret,recovery_codes, created_at, updated_at FROM users
			  WHERE id = $1
	`

	user := &User{}

	var emailVerifiedAt sql.NullTime
	var avatarURL sql.NullString
	var isActive sql.NullBool
	var authSecret sql.NullString

	err := s.db.QueryRowContext(ctx, query, userID).Scan(
		&user.ID,
		&user.Email,
		&user.Password.hash,
		&user.ForcePasswordChange,
		&avatarURL,
		&user.Role,
		&emailVerifiedAt,
		&user.Version,
		&isActive,
		&user.TwoFactorAuthEnabled,
		&authSecret,
		pq.Array(&user.RecoveryCodes),
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	if emailVerifiedAt.Valid {
		user.EmailVerifiedAt = &emailVerifiedAt.Time
	}

	if avatarURL.Valid {
		user.AvatarURL = avatarURL.String
	}

	if isActive.Valid {
		user.IsActive = isActive.Bool
	}

	if authSecret.Valid {
		user.AuthSecret = authSecret.String
	}

	return user, nil
}

func (s *UserModel) GetByEmail(ctx context.Context, email string) (*User, error) {
	query := `SELECT id, email, password_hash,force_password_change,
			  avatar_url, role, email_verified_at, version,
			  is_active,  two_factor_auth_enabled, auth_secret, recovery_codes, created_at, updated_at FROM users
			  WHERE email ilike $1
	`

	user := &User{}

	var emailVerifiedAt sql.NullTime
	var avatarURL sql.NullString
	var isActive sql.NullBool
	var authSecret sql.NullString

	err := s.db.QueryRowContext(ctx, query, email).Scan(
		&user.ID,
		&user.Email,
		&user.Password.hash,
		&user.ForcePasswordChange,
		&avatarURL,
		&user.Role,
		&emailVerifiedAt,
		&user.Version,
		&isActive,
		&user.TwoFactorAuthEnabled,
		&authSecret,
		pq.Array(&user.RecoveryCodes),
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	if emailVerifiedAt.Valid {
		user.EmailVerifiedAt = &emailVerifiedAt.Time
	}

	if avatarURL.Valid {
		user.AvatarURL = avatarURL.String
	}

	if isActive.Valid {
		user.IsActive = isActive.Bool
	}

	if authSecret.Valid {
		user.AuthSecret = authSecret.String
	}

	return user, nil
}

func (s *UserModel) SetUserAccountAsActivate(ctx context.Context, user *User) error {
	query := `UPDATE users
			SET email_verified_at = now(), is_active = true
			WHERE id = $1
			RETURNING email_verified_at, updated_at`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)

	defer cancel()

	err := s.db.QueryRowContext(ctx, query, user.ID).Scan(
		&user.EmailVerifiedAt,
		&user.UpdatedAt,
	)

	return err
}

func (s *UserModel) UpdatePassword(ctx context.Context, user *User, password string) error {
	query := `UPDATE users SET password_hash = $1 WHERE id = $2`

	err := user.Password.Set(password)

	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)

	defer cancel()
	_, err = s.db.ExecContext(ctx, query, user.Password.hash, user.ID)
	return err
}

// EnableTwoFactorAuth enables 2FA for a user.
func (s *UserModel) EnableTwoFactorAuth(ctx context.Context, userID, authSecret string, recoveryCodes []string) error {
	query := `
		UPDATE users
		SET auth_secret = $1, two_factor_auth_enabled = TRUE
		WHERE id = $2
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	_, err := s.db.ExecContext(ctx, query, authSecret, userID)
	if err != nil {
		return fmt.Errorf("failed to enable 2FA: %w", err)
	}

	return nil
}

// DisableTwoFactorAuth disables 2FA for a user.
func (s *UserModel) DisableTwoFactorAuth(ctx context.Context, userID string) error {
	query := `
		UPDATE users
		SET auth_secret = NULL, two_factor_auth_enabled = FALSE, recovery_codes = null
		WHERE id = $1
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	_, err := s.db.ExecContext(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to disable 2FA: %w", err)
	}

	return nil
}

func (s *UserModel) GetNormalUserByID(ctx context.Context, userID string) (*NormalUser, error) {
	query := `
		SELECT n.id, n.first_name, n.last_name, n.phone_number, n.user_id, n.created_at, n.updated_at,
			   u.id, u.email, u.password_hash, u.avatar_url, u.role, u.email_verified_at,
			   u.is_active, u.two_factor_auth_enabled, u.auth_secret, u.created_at, u.updated_at
		FROM normal_users n
		JOIN users u ON n.user_id = u.id
		WHERE u.id = $1
	`

	normalUser := &NormalUser{}
	user := &User{}

	var emailVerifiedAt sql.NullTime
	var avatarURL sql.NullString
	var isActive sql.NullBool
	var authSecret sql.NullString

	err := s.db.QueryRowContext(ctx, query, userID).Scan(
		&normalUser.ID,
		&normalUser.FirstName,
		&normalUser.LastName,
		&normalUser.PhoneNumber,
		&normalUser.UserID,
		&normalUser.CreatedAt,
		&normalUser.UpdatedAt,
		&user.ID,
		&user.Email,
		&user.Password.hash,
		&avatarURL,
		&user.Role,
		&emailVerifiedAt,
		&isActive,
		&user.TwoFactorAuthEnabled,
		&authSecret,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	if emailVerifiedAt.Valid {
		user.EmailVerifiedAt = &emailVerifiedAt.Time
	}

	if avatarURL.Valid {
		user.AvatarURL = avatarURL.String
	}

	if isActive.Valid {
		user.IsActive = isActive.Bool
	}

	if authSecret.Valid {
		user.AuthSecret = authSecret.String
	}

	normalUser.User = *user

	return normalUser, nil
}

func (s *UserModel) GetAdminUserByID(ctx context.Context, userID string) (*AdminUser, error) {
	query := `
		SELECT a.id, a.first_name, a.last_name, a.user_id,a.admin_level, a.created_at, a.updated_at,
			   u.id, u.email, u.password_hash, u.avatar_url, u.role, u.email_verified_at,
			   u.is_active, u.two_factor_auth_enabled, u.auth_secret, u.created_at, u.updated_at
		FROM admin_users a
		JOIN users u ON a.user_id = u.id
		WHERE u.id = $1
	`

	adminUser := &AdminUser{}
	user := &adminUser.User

	var emailVerifiedAt sql.NullTime
	var avatarURL sql.NullString
	var isActive sql.NullBool
	var authSecret sql.NullString

	err := s.db.QueryRowContext(ctx, query, userID).Scan(
		&adminUser.ID,
		&adminUser.FirstName,
		&adminUser.LastName,
		&adminUser.UserID,
		&adminUser.AdminLevel,
		&adminUser.CreatedAt,
		&adminUser.UpdatedAt,
		&user.ID,
		&user.Email,
		&user.Password.hash,
		&avatarURL,
		&user.Role,
		&emailVerifiedAt,
		&isActive,
		&user.TwoFactorAuthEnabled,
		&authSecret,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	if emailVerifiedAt.Valid {
		user.EmailVerifiedAt = &emailVerifiedAt.Time
	}

	if avatarURL.Valid {
		user.AvatarURL = avatarURL.String
	}

	if isActive.Valid {
		user.IsActive = isActive.Bool
	}

	if authSecret.Valid {
		user.AuthSecret = authSecret.String
	}

	return adminUser, nil
}

func (s *UserModel) GetAdminByID(ctx context.Context, ID string) (*AdminUser, error) {
	query := `
		SELECT a.id, a.first_name, a.last_name, a.user_id,a.admin_level, a.created_at, a.updated_at,
			   u.id, u.email, u.password_hash, u.avatar_url, u.role, u.email_verified_at,
			   u.is_active, u.two_factor_auth_enabled, u.auth_secret, u.created_at, u.updated_at
		FROM admin_users a
		JOIN users u ON a.user_id = u.id
		WHERE a.id = $1
	`

	adminUser := &AdminUser{}
	user := &adminUser.User

	var emailVerifiedAt sql.NullTime
	var avatarURL sql.NullString
	var isActive sql.NullBool
	var authSecret sql.NullString

	err := s.db.QueryRowContext(ctx, query, ID).Scan(
		&adminUser.ID,
		&adminUser.FirstName,
		&adminUser.LastName,
		&adminUser.UserID,
		&adminUser.AdminLevel,
		&adminUser.CreatedAt,
		&adminUser.UpdatedAt,
		&user.ID,
		&user.Email,
		&user.Password.hash,
		&avatarURL,
		&user.Role,
		&emailVerifiedAt,
		&isActive,
		&user.TwoFactorAuthEnabled,
		&authSecret,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	if emailVerifiedAt.Valid {
		user.EmailVerifiedAt = &emailVerifiedAt.Time
	}

	if avatarURL.Valid {
		user.AvatarURL = avatarURL.String
	}

	if isActive.Valid {
		user.IsActive = isActive.Bool
	}

	if authSecret.Valid {
		user.AuthSecret = authSecret.String
	}

	return adminUser, nil
}

func (s *UserModel) GetVendorUserByID(ctx context.Context, userID string) (*VendorUser, error) {
	query := `
		SELECT v.id, v.business_name, v.business_address, v.contact_number, v.city, v.country, v.user_id, v.created_by_admin_id,
			   v.approved_at, v.suspended_at, v.created_at, v.updated_at,
			   u.id, u.email, u.password_hash, u.avatar_url, u.role, u.email_verified_at,
			   u.is_active, u.two_factor_auth_enabled, u.auth_secret, u.created_at, u.updated_at
		FROM vendor_users v
		JOIN users u ON v.user_id = u.id
		WHERE u.id = $1
	`

	vendorUser := &VendorUser{}
	user := &User{}

	var emailVerifiedAt sql.NullTime
	var avatarURL sql.NullString
	var isActive sql.NullBool
	var authSecret sql.NullString
	var approvedAt sql.NullTime
	var suspendedAt sql.NullTime

	err := s.db.QueryRowContext(ctx, query, userID).Scan(
		&vendorUser.ID,
		&vendorUser.BusinessName,
		&vendorUser.BusinessAddress,
		&vendorUser.ContactNumber,
		&vendorUser.City,
		&vendorUser.Country,
		&vendorUser.UserID,
		&vendorUser.CreatedByAdminID,
		&approvedAt,
		&suspendedAt,
		&vendorUser.CreatedAt,
		&vendorUser.UpdatedAt,
		&user.ID,
		&user.Email,
		&user.Password.hash,
		&avatarURL,
		&user.Role,
		&emailVerifiedAt,
		&isActive,
		&user.TwoFactorAuthEnabled,
		&authSecret,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	if emailVerifiedAt.Valid {
		user.EmailVerifiedAt = &emailVerifiedAt.Time
	}

	if avatarURL.Valid {
		user.AvatarURL = avatarURL.String
	}

	if isActive.Valid {
		user.IsActive = isActive.Bool
	}

	if authSecret.Valid {
		user.AuthSecret = authSecret.String
	}

	if approvedAt.Valid {
		vendorUser.ApprovedAt = &approvedAt.Time
	}

	if suspendedAt.Valid {
		vendorUser.SuspendedAt = &suspendedAt.Time
	}

	vendorUser.User = *user

	return vendorUser, nil
}

func (u *UserModel) GetVendorByID(ctx context.Context, ID string) (*VendorUser, error) {
	query := `
		SELECT v.id, v.business_name, v.business_address, v.contact_number, v.city, v.country, v.user_id, v.created_by_admin_id,
			   v.approved_at, v.suspended_at, v.created_at, v.updated_at,
			   u.id, u.email, u.password_hash, u.avatar_url, u.role, u.email_verified_at,
			   u.is_active, u.two_factor_auth_enabled, u.auth_secret, u.created_at, u.updated_at
		FROM vendor_users v
		JOIN users u ON u.id = v.user_id
		WHERE v.id = $1
	`

	vendorUser := &VendorUser{}
	user := &User{}

	var emailVerifiedAt sql.NullTime
	var avatarURL sql.NullString
	var isActive sql.NullBool
	var authSecret sql.NullString
	var approvedAt sql.NullTime
	var suspendedAt sql.NullTime

	err := u.db.QueryRowContext(ctx, query, ID).Scan(
		&vendorUser.ID,
		&vendorUser.BusinessName,
		&vendorUser.BusinessAddress,
		&vendorUser.ContactNumber,
		&vendorUser.City,
		&vendorUser.Country,
		&vendorUser.UserID,
		&vendorUser.CreatedByAdminID,
		&approvedAt,
		&suspendedAt,
		&vendorUser.CreatedAt,
		&vendorUser.UpdatedAt,
		&user.ID,
		&user.Email,
		&user.Password.hash,
		&avatarURL,
		&user.Role,
		&emailVerifiedAt,
		&isActive,
		&user.TwoFactorAuthEnabled,
		&authSecret,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	if emailVerifiedAt.Valid {
		user.EmailVerifiedAt = &emailVerifiedAt.Time
	}

	if avatarURL.Valid {
		user.AvatarURL = avatarURL.String
	}

	if isActive.Valid {
		user.IsActive = isActive.Bool
	}

	if authSecret.Valid {
		user.AuthSecret = authSecret.String
	}

	if approvedAt.Valid {
		vendorUser.ApprovedAt = &approvedAt.Time
	}

	if suspendedAt.Valid {
		vendorUser.SuspendedAt = &suspendedAt.Time
	}

	vendorUser.User = *user

	return vendorUser, nil
}

func (u *UserModel) FlattenUser(ctx context.Context, user *User) (*FlattenedUser, error) {
	var flattenUser *FlattenedUser

	switch user.Role {
	case UserRole:
		normalUser, err := u.GetNormalUserByID(ctx, user.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch normal user: %w", err)
		}
		flattenUser = MarshalNormalUser(*normalUser)

	case VendorRole:
		vendorUser, err := u.GetVendorUserByID(ctx, user.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch vendor user: %w", err)
		}
		flattenUser = MarshalVendorUser(*vendorUser)

	case AdminRole:
		adminUser, err := u.GetAdminUserByID(ctx, user.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch admin user: %w", err)
		}
		flattenUser = MarshalAdminUser(*adminUser)

	default:
		return nil, ErrUnknownUserRole
	}

	return flattenUser, nil
}

func (u *UserModel) ResetRecoveryCodes(ctx context.Context, userID string, recoveryCodes []string) error {
	query := `UPDATE users SET recovery_codes = $1 WHERE id = $2`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)

	defer cancel()

	_, err := u.db.ExecContext(ctx, query, pq.Array(recoveryCodes), userID)

	if err != nil {
		return fmt.Errorf("failed to reset recovery codes: %w", err)
	}

	return err
}

func (u *UserModel) GetNormalUsers(ctx context.Context, filter PaginateQueryFilter) ([]*NormalUser, Metadata, error) {
	query := fmt.Sprintf(`
		SELECT count(n.id) over(),  n.id, n.first_name, n.last_name, n.phone_number, n.user_id, n.created_at, n.updated_at,
			   u.id, u.email, u.avatar_url, u.role, u.email_verified_at,
			   u.is_active, u.two_factor_auth_enabled, u.created_at, u.updated_at
		FROM normal_users n
		JOIN users u ON n.user_id = u.id
		ORDER BY u.%s %s
		LIMIT $1 OFFSET $2
	`, filter.SortColumn(), filter.SortDirection())

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	rows, err := u.db.QueryContext(ctx, query, filter.Limit(), filter.Offset())

	if err != nil {
		return nil, Metadata{}, err
	}

	var (
		users        = []*NormalUser{}
		totalRecords int
	)

	defer rows.Close()

	for rows.Next() {

		normalUser := &NormalUser{}
		user := &normalUser.User

		var emailVerifiedAt sql.NullTime
		var avatarURL sql.NullString
		var isActive sql.NullBool

		err := rows.Scan(
			&totalRecords,
			&normalUser.ID,
			&normalUser.FirstName,
			&normalUser.LastName,
			&normalUser.PhoneNumber,
			&normalUser.UserID,
			&normalUser.CreatedAt,
			&normalUser.UpdatedAt,
			&user.ID,
			&user.Email,
			&avatarURL,
			&user.Role,
			&emailVerifiedAt,
			&isActive,
			&user.TwoFactorAuthEnabled,
			&user.CreatedAt,
			&user.UpdatedAt,
		)

		if err != nil {
			return nil, Metadata{}, err
		}

		if emailVerifiedAt.Valid {
			user.EmailVerifiedAt = &emailVerifiedAt.Time
		}

		if avatarURL.Valid {
			user.AvatarURL = avatarURL.String
		}

		if isActive.Valid {
			user.IsActive = isActive.Bool
		}

		users = append(users, normalUser)
	}

	if err = rows.Err(); err != nil {
		return nil, Metadata{}, err
	}

	metadata := calculateMetadata(totalRecords, filter.Page, filter.PageSize)

	return users, metadata, nil
}

func (u *UserModel) GetVendorUsers(ctx context.Context, filter PaginateQueryFilter) ([]*VendorUser, Metadata, error) {
	query := fmt.Sprintf(`
		SELECT count(v.id) over (), v.id, v.business_name, v.business_address, v.contact_number, v.user_id, v.created_by_admin_id,
			   v.approved_at, v.suspended_at, v.created_at, v.updated_at,
			   u.id, u.email, u.password_hash, u.avatar_url, u.role, u.email_verified_at,
			   u.is_active, u.two_factor_auth_enabled, u.auth_secret, u.created_at, u.updated_at
		FROM vendor_users v
		JOIN users u ON v.user_id = u.id
		ORDER BY u.%s %s
		LIMIT $1 OFFSET $2
	`, filter.SortColumn(), filter.SortDirection())

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	rows, err := u.db.QueryContext(ctx, query, filter.Limit(), filter.Offset())

	if err != nil {
		return nil, Metadata{}, err
	}

	var (
		users        = []*VendorUser{}
		totalRecords int
	)

	defer rows.Close()

	for rows.Next() {
		vendorUser := &VendorUser{}
		user := &vendorUser.User

		var emailVerifiedAt sql.NullTime
		var avatarURL sql.NullString
		var isActive sql.NullBool
		var authSecret sql.NullString
		var approvedAt sql.NullTime
		var suspendedAt sql.NullTime

		err := rows.Scan(
			&totalRecords,
			&vendorUser.ID,
			&vendorUser.BusinessName,
			&vendorUser.BusinessAddress,
			&vendorUser.ContactNumber,
			&vendorUser.UserID,
			&vendorUser.CreatedByAdminID,
			&approvedAt,
			&suspendedAt,
			&vendorUser.CreatedAt,
			&vendorUser.UpdatedAt,
			&user.ID,
			&user.Email,
			&user.Password.hash,
			&avatarURL,
			&user.Role,
			&emailVerifiedAt,
			&isActive,
			&user.TwoFactorAuthEnabled,
			&authSecret,
			&user.CreatedAt,
			&user.UpdatedAt,
		)

		if err != nil {
			return nil, Metadata{}, err
		}

		if emailVerifiedAt.Valid {
			user.EmailVerifiedAt = &emailVerifiedAt.Time
		}

		if avatarURL.Valid {
			user.AvatarURL = avatarURL.String
		}

		if isActive.Valid {
			user.IsActive = isActive.Bool
		}

		if authSecret.Valid {
			user.AuthSecret = authSecret.String
		}

		if approvedAt.Valid {
			vendorUser.ApprovedAt = &approvedAt.Time
		}

		if suspendedAt.Valid {
			vendorUser.SuspendedAt = &suspendedAt.Time
		}

		users = append(users, vendorUser)
	}

	if err = rows.Err(); err != nil {
		return nil, Metadata{}, err
	}

	metadata := calculateMetadata(totalRecords, filter.Page, filter.PageSize)
	return users, metadata, nil
}

func (u *UserModel) GetAdminUsers(ctx context.Context, filter PaginateQueryFilter) ([]*AdminUser, Metadata, error) {
	query := fmt.Sprintf(`
		SELECT count(a.id) over(), a.id, a.first_name, a.last_name,a.admin_level, a.user_id, a.created_at, a.updated_at,
			   u.id, u.email, u.password_hash, u.avatar_url, u.role, u.email_verified_at,
			   u.is_active, u.two_factor_auth_enabled, u.auth_secret, u.created_at, u.updated_at
		FROM admin_users a
		JOIN users u ON a.user_id = u.id
		ORDER BY u.%s %s
		LIMIT $1 OFFSET $2
	`, filter.SortColumn(), filter.SortDirection())

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	rows, err := u.db.QueryContext(ctx, query, filter.Limit(), filter.Offset())

	if err != nil {
		return nil, Metadata{}, err
	}

	var (
		users        = []*AdminUser{}
		totalRecords int
	)

	defer rows.Close()

	for rows.Next() {
		adminUser := &AdminUser{}
		user := &adminUser.User

		var emailVerifiedAt sql.NullTime
		var avatarURL sql.NullString
		var isActive sql.NullBool
		var authSecret sql.NullString

		err := rows.Scan(
			&totalRecords,
			&adminUser.ID,
			&adminUser.FirstName,
			&adminUser.LastName,
			&adminUser.AdminLevel,
			&adminUser.UserID,
			&adminUser.CreatedAt,
			&adminUser.UpdatedAt,
			&user.ID,
			&user.Email,
			&user.Password.hash,
			&avatarURL,
			&user.Role,
			&emailVerifiedAt,
			&isActive,
			&user.TwoFactorAuthEnabled,
			&authSecret,
			&user.CreatedAt,
			&user.UpdatedAt,
		)

		if err != nil {
			return nil, Metadata{}, ErrRecordNotFound
		}

		if emailVerifiedAt.Valid {
			user.EmailVerifiedAt = &emailVerifiedAt.Time
		}

		if avatarURL.Valid {
			user.AvatarURL = avatarURL.String
		}

		if isActive.Valid {
			user.IsActive = isActive.Bool
		}

		if authSecret.Valid {
			user.AuthSecret = authSecret.String
		}
		users = append(users, adminUser)
	}

	if err = rows.Err(); err != nil {
		return nil, Metadata{}, err
	}

	metadata := calculateMetadata(totalRecords, filter.Page, filter.PageSize)
	return users, metadata, nil
}

func (m *UserModel) ChangeAdminLevel(ctx context.Context, AdminID string, NewLevel AdminLevel) error {
	return nil
}

func (m *UserModel) DisableUser(ctx context.Context, userID string) error {
	query := `UPDATE users SET disabled = true WHERE id = $1`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)

	defer cancel()

	result, err := m.db.ExecContext(ctx, query, userID)

	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()

	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrRecordNotFound
	}

	return nil
}

func (m *UserModel) EnableUser(ctx context.Context, userID string) error {
	query := `UPDATE users SET disabled = false WHERE id = $1`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)

	defer cancel()

	result, err := m.db.ExecContext(ctx, query, userID)

	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()

	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrRecordNotFound
	}

	return nil
}

func (m *UserModel) GetUserAccountByUserID(ctx context.Context, userID string) (*Account, error) {
	query := `
		SELECT user_id, type, provider, provider_account_id,access_token,
			refresh_token,expires_at,token_type, scope
		FROM accounts
		WHERE user_id = $1
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	account := &Account{}
	err := m.db.QueryRowContext(ctx, query, userID).Scan(&account.UserID, &account.Type,
		&account.Provider, &account.ProviderAccountID, &account.AccessToken,
		&account.RefreshToken, &account.ExpiresAt, &account.TokenType, &account.Scope,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound

		default:
			return nil, err
		}
	}

	return account, nil
}

func (m *UserModel) ChangePassword(ctx context.Context, user *User) error {
	query := `UPDATE users SET password_hash = $1 WHERE id = $2`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	result, err := m.db.ExecContext(ctx, query, user.Password.hash, user.ID)

	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()

	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrRecordNotFound
	}

	return nil
}

func (m *UserModel) UpdateEmail(ctx context.Context, userID string, newEmail string) error {
	query := `UPDATE users SET email = $1 WHERE id = $2`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)

	defer cancel()

	result, err := m.db.ExecContext(ctx, query, newEmail, userID)

	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()

	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrRecordNotFound
	}

	return nil
}
