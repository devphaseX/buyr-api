package store

import "time"

type Audit struct {
	ID        string    `json:"id"`
	AdminID   string    `json:"admin_id"`
	Action    string    `json:"action"`
	Details   string    `json:"details"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
