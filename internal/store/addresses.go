package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/devphaseX/buyr-api.git/internal/db"
)

type AddressType string

const (
	ShippingAddressType AddressType = "shipping"
	BillingAddressType  AddressType = "billing"
)

type Address struct {
	ID            string      `json:"id"`
	FirstName     string      `json:"first_name"`
	LastName      string      `json:"last_name"`
	PhoneNumber   string      `json:"phone_number"`
	UserID        string      `json:"user_id"`
	AddressType   AddressType `json:"address_type"`
	StreetAddress string      `json:"street_address"`
	City          string      `json:"city"`
	State         string      `json:"state"`
	PostalCode    string      `json:"postal_code"`
	Country       string      `json:"country"`
	IsDefault     bool        `json:"is_default"`
	CreatedAt     time.Time   `json:"created_at"`
	UpdatedAt     time.Time   `json:"updated_at"`
}

type AddressStore interface {
	Create(ctx context.Context, addr *Address) error
	GetByID(ctx context.Context, ID string) (*Address, error)
	GetByUserID(ctx context.Context, userID string) ([]*Address, error)
	SetDefault(ctx context.Context, userID string, addressID string, addrType AddressType) error
}

type AddressModel struct {
	db *sql.DB
}

func NewAddressModel(db *sql.DB) AddressStore {
	return &AddressModel{db}
}

func clearAddressTypeDefault(ctx context.Context, tx *sql.Tx, userID string, addrType AddressType) error {
	query := `UPDATE address
			  SET is_default = false
			  WHERE user_id = $1 AND address_type = $2 AND is_default = true`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)

	defer cancel()
	_, err := tx.ExecContext(ctx, query, userID, addrType)

	if err != nil {
		return err
	}

	return nil
}

func createAddress(ctx context.Context, tx *sql.Tx, address *Address) error {
	address.ID = db.GenerateULID()

	query := `INSERT INTO address(id, first_name, last_name, phone_number,
			user_id, address_type, street_address,
			city, state, postal_code, country, is_default)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)

	defer cancel()
	_, err := tx.ExecContext(ctx, query,
		address.ID,
		address.FirstName,
		address.LastName,
		address.PhoneNumber,
		address.UserID,
		address.AddressType,
		address.StreetAddress,
		address.City,
		address.State,
		address.PostalCode,
		address.Country,
		address.IsDefault,
	)
	if err != nil {
		return err
	}

	return err
}

func (m *AddressModel) Create(ctx context.Context, addr *Address) error {
	return withTrx(m.db, ctx, func(tx *sql.Tx) error {
		if err := createAddress(ctx, tx, addr); err != nil {
			return err
		}

		if addr.IsDefault {
			if err := clearAddressTypeDefault(ctx, tx, addr.UserID, addr.AddressType); err != nil {
				return err
			}
		}
		return nil
	})
}

func (m *AddressModel) GetByID(ctx context.Context, ID string) (*Address, error) {
	query := `SELECT id, first_name, last_name, phone_number,
			  user_id, address_type, street_address,
			  city, state, postal_code, country,
	 	      is_default, created_at, updated_at
			  FROM address WHERE id = $1`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	address := &Address{}
	err := m.db.QueryRowContext(ctx, query, ID).Scan(&address.ID, &address.FirstName, &address.LastName,
		&address.PhoneNumber, &address.UserID, &address.AddressType, &address.StreetAddress,
		&address.City, &address.State, &address.PostalCode, &address.Country,
		&address.IsDefault, &address.CreatedAt, &address.UpdatedAt)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound

		default:
			return nil, err
		}
	}

	return address, nil
}

func (m *AddressModel) GetByUserID(ctx context.Context, userID string) ([]*Address, error) {
	query := `SELECT id, first_name, last_name, phone_number,
			  user_id, address_type, street_address,
			  city, state, postal_code, country,
	 	      is_default, created_at, updated_at
			  FROM address WHERE user_id = $1`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	rows, err := m.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var addresses []*Address
	for rows.Next() {
		address := &Address{}
		err := rows.Scan(
			&address.ID,
			&address.FirstName,
			&address.LastName,
			&address.PhoneNumber,
			&address.UserID,
			&address.AddressType,
			&address.StreetAddress,
			&address.City,
			&address.State,
			&address.PostalCode,
			&address.Country,
			&address.IsDefault,
			&address.CreatedAt,
			&address.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		addresses = append(addresses, address)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	if len(addresses) == 0 {
		return nil, ErrRecordNotFound
	}

	return addresses, nil
}

func setAddressAsDefault(ctx context.Context, tx *sql.Tx, userID, addressID string, addrType AddressType) error {
	query := `UPDATE address SET is_default = true WHERE id = $1 AND user_id = $2 AND address_type = $3`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)

	defer cancel()

	result, err := tx.ExecContext(ctx, query, addressID, userID, addrType)

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

func (m *AddressModel) SetDefault(ctx context.Context, userID string, addressID string, addrType AddressType) error {
	return withTrx(m.db, ctx, func(tx *sql.Tx) error {
		err := clearAddressTypeDefault(ctx, tx, userID, addrType)

		if err != nil {
			return err
		}

		err = setAddressAsDefault(ctx, tx, userID, addressID, addrType)

		if err != nil {
			return err
		}
		return nil
	})
}
