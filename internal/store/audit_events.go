package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/devphaseX/buyr-api.git/internal/db"
)

type AuditEventType string

var (
	AccountDisableAuditEventType AuditEventType = "account_disabled"
	AccountEnabledAuditEventType AuditEventType = "account_enabled"
	ChangeRoleAuditEventType     AuditEventType = "change_role"
)

type AuditEvent struct {
	ID               string         `json:"id"`
	EventType        AuditEventType `json:"event_type"`
	AccountID        string         `json:"account_id"`
	PerformedBy      string         `json:"performed_by"`
	Reason           string         `json:"reason"`
	Details          []byte         `json:"details"`
	Timestamp        time.Time      `json:"timestamp"`
	AdminLevelAccess AdminLevel     `json:"admin_level_access"`
	IPAddress        string         `json:"ip_address"`
	UserAgent        string         `json:"user_agent"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}

type AuditEventStore interface {
	LogEvent(ctx context.Context, event AuditEvent) error
}

type AuditEventModel struct {
	db *sql.DB
}

func NewAuditEventModel(db *sql.DB) AuditEventStore {
	return &AuditEventModel{db}
}

func (s *AuditEventModel) LogEvent(ctx context.Context, event AuditEvent) error {
	event.ID = db.GenerateULID()
	query := `
		INSERT INTO audit_logs (id, event_type, performed_by, reason, details, timestamp, ip_address, user_agent)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := s.db.ExecContext(ctx, query,
		event.ID,
		event.EventType,
		event.PerformedBy,
		event.Reason,
		event.Details,
		event.Timestamp,
		event.IPAddress,
		event.UserAgent,
	)
	return err
}
