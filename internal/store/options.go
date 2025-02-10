package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/devphaseX/buyr-api.git/internal/db"
)

type OptionType struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	DisplayName string         `json:"display_name"`
	CreatedByID string         `json:"created_by_id"`
	Values      []*OptionValue `json:"values,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type OptionValue struct {
	ID           string    `json:"id"`
	OptionTypeID string    `json:"option_type_id"`
	Value        string    `json:"value"`
	DisplayValue string    `json:"display_value"`
	CreatedByID  string    `json:"created_by_id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type OptionTypeModel struct {
	db *sql.DB
}

type OptionTypeStore interface {
	Create(ctx context.Context, option *OptionType) error
	GetByID(ctx context.Context, id string) (*OptionType, error)
	CreateOptionValues(ctx context.Context, values []*OptionValue) error
	GetAll(ctx context.Context, fq PaginateQueryFilter) ([]*OptionType, Metadata, error)
	Update(ctx context.Context, option *OptionType) error
	UpdateOptionValue(ctx context.Context, value *OptionValue) error
	DeleteOptionValue(ctx context.Context, valueID string) error
}

func NewOptionTypeModel(db *sql.DB) OptionTypeStore {
	return &OptionTypeModel{db}
}

func createOptionType(ctx context.Context, tx *sql.Tx, option *OptionType) error {
	option.ID = db.GenerateULID()

	query := `INSERT INTO option_types(id, name, display_name, created_by_id)
			  VALUES ($1, $2, $3, $4) RETURNING created_at, updated_at`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)

	defer cancel()

	args := []any{option.ID, option.Name, option.DisplayName, option.CreatedByID}

	err := tx.QueryRowContext(ctx, query, args...).Scan(&option.CreatedAt, &option.UpdatedAt)

	if err != nil {
		return err
	}

	return nil
}

func createOptionValues(ctx context.Context, tx *sql.Tx, optionTypeID string, optValues []*OptionValue) error {

	query := `INSERT INTO option_values (id, display_value, value, option_type_id, created_by_id)
			 VALUES ($1, $2, $3, $4, $5) RETURNING created_at, updated_at`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	var wg sync.WaitGroup

	errCh := make(chan error, len(optValues))

	for _, value := range optValues {
		wg.Add(1)

		go func(optValue *OptionValue) {
			defer wg.Done()

			optValue.ID = db.GenerateULID()
			optValue.OptionTypeID = optionTypeID

			args := []any{optValue.ID, optValue.DisplayValue, optValue.Value, optValue.OptionTypeID, optValue.CreatedByID}

			err := tx.QueryRowContext(ctx, query, args...).Scan(&optValue.CreatedAt, &optValue.UpdatedAt)

			if err != nil {
				errCh <- err
				return
			}
		}(value)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *OptionTypeModel) Create(ctx context.Context, option *OptionType) error {
	return withTrx(m.db, ctx, func(tx *sql.Tx) error {
		if err := createOptionType(ctx, tx, option); err != nil {
			return err
		}

		if len(option.Values) == 0 {
			return nil
		}

		if err := createOptionValues(ctx, tx, option.ID, option.Values); err != nil {
			return err
		}
		return nil
	})
}

func (m *OptionTypeModel) GetByID(ctx context.Context, id string) (*OptionType, error) {
	query := `
		SELECT
			ot.id,
			ot.name,
			ot.display_name,
			ot.created_by_id,
			ot.created_at,
			ot.updated_at,
			COALESCE(json_agg(row_to_json(ov)) FILTER (WHERE ov.id IS NOT NULL), '[]') AS values
		FROM
			option_types ot
		LEFT JOIN
			option_values ov ON ov.option_type_id = ot.id
		WHERE
			ot.id = $1
		GROUP BY
			ot.id
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	var (
		option  = &OptionType{}
		rawJSON []byte
	)

	err := m.db.QueryRowContext(ctx, query, id).Scan(
		&option.ID,
		&option.Name,
		&option.DisplayName,
		&option.CreatedByID,
		&option.CreatedAt,
		&option.UpdatedAt,
		&rawJSON,
	)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound

		default:
			return nil, fmt.Errorf("failed to fetch option type: %w", err)
		}
	}

	if err := json.Unmarshal(rawJSON, &option.Values); err != nil {
		return nil, fmt.Errorf("failed to unmarshal option values: %w", err)
	}

	return option, nil
}

func (m *OptionTypeModel) CreateOptionValues(ctx context.Context, values []*OptionValue) error {
	return withTrx(m.db, ctx, func(tx *sql.Tx) error {
		return m.CreateOptionValues(ctx, values)
	})
}

func (m *OptionTypeModel) GetAll(ctx context.Context, fq PaginateQueryFilter) ([]*OptionType, Metadata, error) {
	query := `SELECT count(id) over(), id, name, display_name, created_by_id, created_at, updated_at FROM option_types
			  LIMIT $1 OFFSET $2`
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)

	defer cancel()

	rows, err := m.db.QueryContext(ctx, query, fq.Limit(), fq.Offset())

	if err != nil {
		return nil, Metadata{}, err
	}

	var (
		options     = []*OptionType{}
		totalRecord int
	)
	defer rows.Close()

	for rows.Next() {
		option := &OptionType{}

		var createdByID sql.NullString

		err := rows.Scan(&totalRecord, &option.ID, &option.Name, &option.DisplayName,
			&createdByID, &option.CreatedAt, &option.UpdatedAt)

		if err != nil {
			return nil, Metadata{}, fmt.Errorf("failed to scan option: %w", err)
		}

		if createdByID.Valid {
			option.CreatedByID = createdByID.String
		}

		options = append(options, option)
	}

	if err := rows.Err(); err != nil {
		return nil, Metadata{}, fmt.Errorf("failed to get options: %w", err)
	}

	metadata := calculateMetadata(totalRecord, fq.Page, fq.PageSize)
	return options, metadata, nil
}

func (m *OptionTypeModel) Update(ctx context.Context, option *OptionType) error {
	query := `
		UPDATE option_types
		SET name = $1, display_name = $2, updated_at = NOW()
		WHERE id = $3
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	result, err := m.db.ExecContext(ctx, query, option.Name, option.DisplayName, option.ID)
	if err != nil {
		return fmt.Errorf("failed to update option type: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if rowsAffected == 0 {
		return ErrRecordNotFound
	}

	return nil
}

func (m *OptionTypeModel) UpdateOptionValue(ctx context.Context, value *OptionValue) error {
	query := `
		UPDATE option_values
		SET display_value = $1, value = $2, updated_at = NOW()
		WHERE id = $3
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	result, err := m.db.ExecContext(ctx, query, value.DisplayValue, value.Value, value.ID)
	if err != nil {
		return fmt.Errorf("failed to update option value: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if rowsAffected == 0 {
		return ErrRecordNotFound
	}

	return nil
}

func (m *OptionTypeModel) DeleteOptionValue(ctx context.Context, valueID string) error {
	query := `
		DELETE FROM option_values
		WHERE id = $1
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	result, err := m.db.ExecContext(ctx, query, valueID)
	if err != nil {
		return fmt.Errorf("failed to delete option value: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if rowsAffected == 0 {
		return ErrRecordNotFound
	}

	return nil
}
