package store

import "time"

type Cart struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	IsActive  bool      `json:"is_active"`
}

type CartItem struct {
	ID        string    `json:"id"`
	CartID    string    `json:"cart_id"`
	ProductID string    `json:"product_id"`
	AddedAt   time.Time `json:"added_at"`
	Quantity  int       `json:"quantity"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
