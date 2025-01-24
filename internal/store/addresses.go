package store

import "time"

type Address struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	AddressType  string    `json:"address_type"`
	StreetAddress string    `json:"street_address"`
	City         string    `json:"city"`
	State        string    `json:"state"`
	PostalCode   string    `json:"postal_code"`
	Country      string    `json:"country"`
	IsDefault    bool      `json:"is_default"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
