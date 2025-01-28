package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/devphaseX/buyr-api.git/internal/db"
)

type Category struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	Description      string    `json:"description"`
	Visible          bool      `json:"visible"`
	CreatedByAdminID string    `json:"created_by_admin_id,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type CategoryStore interface {
	Create(ctx context.Context, category *Category) error
	RemoveByID(ctx context.Context, categoryID string) error
	GetPublicCategorie(ctx context.Context, filter PaginateQueryFilter) ([]*Category, Metadata, error)
	GetAdminCategoryView(ctx context.Context, filter PaginateQueryFilter) ([]*AdminCategoryView, Metadata, error)
}

type CategoryModel struct {
	db *sql.DB
}

func NewCategoryModel(db *sql.DB) CategoryStore {
	return &CategoryModel{db}
}

func (m *CategoryModel) Create(ctx context.Context, category *Category) error {

	query := `INSERT INTO category(id, name, description,visible, created_by_admin_id)
			 VALUES($1, $2, $3, $4, $5)
			  RETURNING id, created_at, updated_at
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	id := db.GenerateULID()

	args := []any{id, category.Name, category.Description, category.Visible, category.CreatedByAdminID}

	err := m.db.QueryRowContext(ctx, query, args...).Scan(&category.ID, &category.CreatedAt, &category.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create category: %w", err)
	}

	return nil
}

func (m *CategoryModel) GetPublicCategorie(ctx context.Context, filter PaginateQueryFilter) ([]*Category, Metadata, error) {
	query := fmt.Sprintf(
		`
			SELECT count(id) over(), id, name, description,visible, created_at, updated_at FROM category
		    WHERE visible = true
			ORDER BY %s %s
			LIMIT $1 OFFSET $2
		`, filter.SortColumn(), filter.SortDirection())

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)

	defer cancel()

	var (
		categories  = []*Category{}
		totalRecord int
	)

	rows, err := m.db.QueryContext(ctx, query, filter.Limit(), filter.Offset())

	if err != nil {
		return nil, Metadata{}, err
	}

	for rows.Next() {
		category := &Category{}

		err := rows.Scan(
			&totalRecord,
			&category.ID,
			&category.Name,
			&category.Description,
			&category.Visible,
			&category.CreatedAt,
			&category.UpdatedAt)

		if err != nil {
			return nil, Metadata{}, err
		}

		categories = append(categories, category)
	}

	metadata := calculateMetadata(totalRecord, filter.Page, filter.PageSize)

	return categories, metadata, nil
}

type AdminCategoryView struct {
	Category
	CreatedBy *AdminUser `json:"created_by,omitempty"`
}

func (m *CategoryModel) GetAdminCategoryView(ctx context.Context, filter PaginateQueryFilter) ([]*AdminCategoryView, Metadata, error) {
	query := fmt.Sprintf(
		`
        SELECT
            count(c.id) OVER(),
            c.id,
            c.name,
            c.description,
            c.visible,
            c.created_at,
            c.updated_at,
            au.id AS admin_user_id,
            au.first_name,
            au.last_name,
            au.user_id,
            au.admin_level,
            au.created_at AS admin_user_created_at,
            au.updated_at AS admin_user_updated_at,
            u.id AS user_id,
            u.email,
            u.avatar_url,
            u.role,
            u.email_verified_at,
            u.force_password_change,
            u.is_active,
            u.created_at AS user_created_at,
            u.updated_at AS user_updated_at
        FROM
            category c
        LEFT JOIN
            admin_users au ON c.created_by_admin_id = au.id
        LEFT JOIN
            users u ON au.user_id = u.id
        WHERE
            c.visible = true
        ORDER BY
            %s %s
        LIMIT $1 OFFSET $2
        `, filter.SortColumn(), filter.SortDirection())

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	var (
		categories  = []*AdminCategoryView{}
		totalRecord int
	)

	rows, err := m.db.QueryContext(ctx, query, filter.Limit(), filter.Offset())
	if err != nil {
		return nil, Metadata{}, err
	}
	defer rows.Close()

	for rows.Next() {
		category := &AdminCategoryView{
			CreatedBy: &AdminUser{},
		}

		var (
			adminUserID             *string
			adminUserFirstName      *string
			adminUserLastName       *string
			adminUserUserID         *string
			adminUserAdminLevel     *AdminLevel
			adminUserCreatedAt      *time.Time
			adminUserUpdatedAt      *time.Time
			userID                  *string
			userEmail               *string
			userAvatarURL           *string
			userRole                *Role
			userEmailVerifiedAt     *time.Time
			userForcePasswordChange *bool
			userIsActive            *bool
			userCreatedAt           *time.Time
			userUpdatedAt           *time.Time
		)

		err := rows.Scan(
			&totalRecord,
			&category.ID,
			&category.Name,
			&category.Description,
			&category.Visible,
			&category.CreatedAt,
			&category.UpdatedAt,
			&adminUserID,
			&adminUserFirstName,
			&adminUserLastName,
			&adminUserUserID,
			&adminUserAdminLevel,
			&adminUserCreatedAt,
			&adminUserUpdatedAt,
			&userID,
			&userEmail,
			&userAvatarURL,
			&userRole,
			&userEmailVerifiedAt,
			&userForcePasswordChange,
			&userIsActive,
			&userCreatedAt,
			&userUpdatedAt,
		)

		if err != nil {
			return nil, Metadata{}, err
		}

		// If adminUserID is nil, set CreatedBy to nil
		if adminUserID == nil {
			category.CreatedBy = nil
		} else {
			category.CreatedBy = &AdminUser{
				ID:         *adminUserID,
				FirstName:  *adminUserFirstName,
				LastName:   *adminUserLastName,
				UserID:     *adminUserUserID,
				AdminLevel: *adminUserAdminLevel,
				CreatedAt:  *adminUserCreatedAt,
				UpdatedAt:  *adminUserUpdatedAt,
				User: User{
					ID:                  *userID,
					Email:               *userEmail,
					Role:                *userRole,
					EmailVerifiedAt:     userEmailVerifiedAt,
					ForcePasswordChange: *userForcePasswordChange,
					IsActive:            *userIsActive,
					CreatedAt:           *userCreatedAt,
					UpdatedAt:           *userUpdatedAt,
				},
			}

			if userAvatarURL != nil {
				category.CreatedBy.User.AvatarURL = *userAvatarURL
			}
		}

		categories = append(categories, category)
	}

	metadata := calculateMetadata(totalRecord, filter.Page, filter.PageSize)
	return categories, metadata, nil
}

func (m *CategoryModel) RemoveByID(ctx context.Context, categoryID string) error {
	query := `DELETE FROM category WHERE id = $1`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)

	defer cancel()

	res, err := m.db.ExecContext(ctx, query, categoryID)

	if err != nil {
		return err
	}

	rows, err := res.RowsAffected()

	if err != nil {
		return err
	}

	if rows == 0 {
		return ErrRecordNotFound
	}

	return nil
}
