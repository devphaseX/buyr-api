package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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
	ID          string         `json:"id"`
	EventType   AuditEventType `json:"event_type"`
	AccountID   string         `json:"account_id"`
	PerformedBy string         `json:"performed_by"`
	Reason      string         `json:"reason"`
	Details     []byte         `json:"details"`
	Timestamp   time.Time      `json:"timestamp"`
	AccessLevel int            `json:"admin_level_access"`
	IPAddress   string         `json:"ip_address"`
	UserAgent   string         `json:"user_agent"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type AuditEventStore interface {
	LogEvent(ctx context.Context, event AuditEvent) error
	GetAuditLogs(ctx context.Context, filter PaginateQueryFilter, adminLevel AdminLevel) ([]*AuditEventWithAdmin, Metadata, error)
	GetAuditLogByID(ctx context.Context, id string) (*AuditEventWithAdmin, error)
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
		INSERT INTO audit_events (id, event_type, performed_by, reason, details, timestamp, ip_address, user_agent)
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

type AuditEventWithAdmin struct {
	AuditEvent
	AdminUser AdminUser `json:"admin_user"`
}

func (s *AuditEventModel) GetAuditLogs(ctx context.Context, filter PaginateQueryFilter, adminLevel AdminLevel) ([]*AuditEventWithAdmin, Metadata, error) {
	query := fmt.Sprintf(`
		SELECT
			count(a.id) OVER(),
			a.id, a.event_type, a.account_id, a.performed_by, a.reason, a.details, a.timestamp,
			a.admin_level_access, a.ip_address, a.user_agent, a.created_at, a.updated_at,
			au.first_name, au.last_name, au.admin_level,
			u.id, u.email, u.avatar_url, u.role, u.email_verified_at, u.is_active, u.created_at, u.updated_at
		FROM audit_events a
		JOIN admin_users au ON a.performed_by = au.id
		JOIN users u ON au.user_id = u.id
		WHERE a.admin_level_access = $1
		ORDER BY au.%s %s
		LIMIT $2 OFFSET $3
	`, filter.SortColumn(), filter.SortDirection())

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	rows, err := s.db.QueryContext(ctx, query, adminLevel.GetRank(), filter.Limit(), filter.Offset())
	if err != nil {
		return nil, Metadata{}, fmt.Errorf("failed to query audit logs: %w", err)
	}
	defer rows.Close()

	var (
		auditLogs    = []*AuditEventWithAdmin{}
		totalRecords int
	)

	for rows.Next() {
		var (
			event           AuditEvent
			details         []byte
			createdAt       time.Time
			updatedAt       time.Time
			timestamp       time.Time
			adminUser       AdminUser
			user            User
			emailVerifiedAt sql.NullTime
			avatarURL       sql.NullString
			isActive        sql.NullBool
		)

		err := rows.Scan(
			&totalRecords,
			&event.ID,
			&event.EventType,
			&event.AccountID,
			&event.PerformedBy,
			&event.Reason,
			&details,
			&timestamp,
			&event.AccessLevel,
			&event.IPAddress,
			&event.UserAgent,
			&createdAt,
			&updatedAt,
			&adminUser.FirstName,
			&adminUser.LastName,
			&adminUser.AdminLevel,
			&user.ID,
			&user.Email,
			&avatarURL,
			&user.Role,
			&emailVerifiedAt,
			&isActive,
			&user.CreatedAt,
			&user.UpdatedAt,
		)

		if err != nil {
			return nil, Metadata{}, fmt.Errorf("failed to scan audit log row: %w", err)
		}

		// Convert binary details to a readable format (if needed).
		event.Details = details
		event.Timestamp = timestamp
		event.CreatedAt = createdAt
		event.UpdatedAt = updatedAt

		// Populate the User and AdminUser fields.
		if emailVerifiedAt.Valid {
			user.EmailVerifiedAt = &emailVerifiedAt.Time
		}
		if avatarURL.Valid {
			user.AvatarURL = avatarURL.String
		}
		if isActive.Valid {
			user.IsActive = isActive.Bool
		}

		adminUser.User = user

		// Combine the audit event with the admin user details.
		auditLogs = append(auditLogs, &AuditEventWithAdmin{
			AuditEvent: event,
			AdminUser:  adminUser,
		})
	}

	if err = rows.Err(); err != nil {
		return nil, Metadata{}, fmt.Errorf("error after iterating over audit log rows: %w", err)
	}

	// Calculate metadata for pagination.
	metadata := calculateMetadata(totalRecords, filter.Page, filter.PageSize)

	return auditLogs, metadata, nil
}

func (s *AuditEventModel) GetAuditLogByID(ctx context.Context, id string) (*AuditEventWithAdmin, error) {
	query := `
		SELECT
			a.id, a.event_type, a.account_id, a.performed_by, a.reason, a.details, a.timestamp,
			a.admin_level_access, a.ip_address, a.user_agent, a.created_at, a.updated_at,
			au.first_name, au.last_name, au.admin_level,
			u.id, u.email, u.avatar_url, u.role, u.email_verified_at, u.is_active, u.created_at, u.updated_at
		FROM audit_logs a
		JOIN admin_users au ON a.performed_by = au.id
		JOIN users u ON au.user_id = u.id
		WHERE a.id = $1
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	var (
		event           AuditEvent
		details         []byte
		createdAt       time.Time
		updatedAt       time.Time
		timestamp       time.Time
		adminUser       AdminUser
		user            User
		emailVerifiedAt sql.NullTime
		avatarURL       sql.NullString
		isActive        sql.NullBool
	)

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&event.ID,
		&event.EventType,
		&event.AccountID,
		&event.PerformedBy,
		&event.Reason,
		&details,
		&timestamp,
		&event.AccessLevel,
		&event.IPAddress,
		&event.UserAgent,
		&createdAt,
		&updatedAt,
		&adminUser.FirstName,
		&adminUser.LastName,
		&adminUser.AdminLevel,
		&user.ID,
		&user.Email,
		&avatarURL,
		&user.Role,
		&emailVerifiedAt,
		&isActive,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrRecordNotFound
		}
		return nil, fmt.Errorf("failed to fetch audit log: %w", err)
	}

	// Convert binary details to a readable format (if needed).
	event.Details = details
	event.Timestamp = timestamp
	event.CreatedAt = createdAt
	event.UpdatedAt = updatedAt

	// Populate the User and AdminUser fields.
	if emailVerifiedAt.Valid {
		user.EmailVerifiedAt = &emailVerifiedAt.Time
	}
	if avatarURL.Valid {
		user.AvatarURL = avatarURL.String
	}
	if isActive.Valid {
		user.IsActive = isActive.Bool
	}

	adminUser.User = user

	// Combine the audit event with the admin user details.
	auditLogWithAdmin := &AuditEventWithAdmin{
		AuditEvent: event,
		AdminUser:  adminUser,
	}

	return auditLogWithAdmin, nil
}
