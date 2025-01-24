package store

import "time"

type Notification struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Message   string    `json:"message"`
	IsRead    bool      `json:"is_read"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
