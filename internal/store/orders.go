package store

import "time"

type Order struct {
	ID            string    `json:"id"`
	UserID        string    `json:"user_id"`
	TotalAmount   string    `json:"total_amount"`
	PromoCode     string    `json:"promo_code"`
	Discount      float64   `json:"discount"`
	Status        string    `json:"status"`
	Paid          bool      `json:"paid"`
	PaymentMethod string    `json:"payment_method"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type OrderItem struct {
	ID        string  `json:"id"`
	OrderID   string  `json:"order_id"`
	ProductID string  `json:"product_id"`
	Quantity  int     `json:"quantity"`
	Price     float64 `json:"price"`
}
