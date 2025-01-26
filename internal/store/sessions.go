package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/devphaseX/buyr-api.git/internal/auth"
	"github.com/devphaseX/buyr-api.git/internal/db"
)

type Session struct {
	ID                 string     `json:"id"`
	UserID             string     `json:"user_id"`
	UserAgent          string     `json:"user_agent"`
	IP                 string     `json:"ip"`
	Version            int        `json:"version"`
	ExpiresAt          time.Time  `json:"expires_at"`
	LastUsed           *time.Time `json:"last_used"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	RememberMe         bool       `json:"remember_me"`          // Whether the session should be extended
	MaxRenewalDuration int64      `json:"max_renewal_duration"` // Maximum duration for session renewal (in seconds)
}

type SessionStore interface {
	Create(ctx context.Context, session *Session) error
	ValidateSession(ctx context.Context, sessionID string, version int) (*Session, *User, bool, error)
	InvalidateSession(ctx context.Context, sessionID string) error
	GetSessionByID(ctx context.Context, sessionID string) (*Session, *User, error)
	UpdateLastUsed(ctx context.Context, sessionID string) error
	GetSessionsByUserID(
		ctx context.Context,
		userID string,
		isAdmin bool,
		paginateQuery PaginateQueryFilter,
	) ([]Session, Metadata, error)

	ExtendSessionAndGenerateRefreshToken(
		ctx context.Context,
		session *Session,
		tokenMaker auth.AuthToken,
		rememberPeriod time.Duration,
	) (string, error)
}

type SessionModel struct {
	db *sql.DB
}

func NewSessionModel(db *sql.DB) SessionStore {
	return &SessionModel{db: db}
}

func (s *SessionModel) Create(ctx context.Context, session *Session) error {
	query := `INSERT INTO sessions (id, user_id, user_agent, ip,
			  expires_at, remember_me, max_renewal_duration )
	          VALUES ($1, $2, $3 , $4, $5, $6, $7)`
	if session.ID == "" {
		session.ID = db.GenerateULID()
	}

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()
	_, err := s.db.ExecContext(ctx,
		query,
		session.ID,
		session.UserID,
		session.UserAgent,
		session.IP,
		session.ExpiresAt,
		session.RememberMe,
		session.MaxRenewalDuration,
	)

	if err != nil {
		return err
	}
	return nil
}

func (s *SessionModel) ValidateSession(ctx context.Context, sessionID string, version int) (*Session, *User, bool, error) {
	session, user, err := s.GetSessionByID(ctx, sessionID)

	if err != nil {
		return nil, nil, false, nil
	}

	if session.Version != version {
		return nil, nil, false, nil
	}
	// Check if the session is expired
	now := time.Now()

	if now.After(session.ExpiresAt) {
		_ = s.InvalidateSession(ctx, sessionID)
		return nil, nil, false, nil
	}

	// Check if the session can be extended (Remember Me is enabled)
	canExtend := false
	if session.RememberMe {
		// Calculate the midpoint of the session's lifetime
		sessionDuration := session.ExpiresAt.Sub(session.UpdatedAt)
		midpoint := session.UpdatedAt.Add(sessionDuration / 2)

		// If the current time is past the midpoint, the session can be extended
		if now.After(midpoint) {
			canExtend = true
		}

		// Check if the session has exceeded the maximum renewal duration
		maxRenewalTime := time.Unix(session.MaxRenewalDuration, 0)
		if session.ExpiresAt.After(maxRenewalTime) {
			// The session has exceeded the maximum renewal duration; force the user to log in again
			_ = s.InvalidateSession(ctx, sessionID)
			return nil, nil, false, nil
		}
	}

	return session, user, canExtend, nil
}

func (s *SessionModel) InvalidateSession(ctx context.Context, sessionID string) error {
	query := `DELETE FROM sessions WHERE id = $1`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()
	_, err := s.db.ExecContext(ctx, query, sessionID)
	return err
}

func (s *SessionModel) GetSessionByID(ctx context.Context, sessionID string) (*Session, *User, error) {
	var session Session
	var user User

	query := `
		SELECT
		 s.id, s.user_id, s.user_agent,
		 s.ip, s.expires_at, s.last_used, s.version,
		 s.created_at,s.updated_at, s.remember_me, s.max_renewal_duration,
		 u.id, u.email,
		 u.avatar_url, u.role,
	 	 u.email_verified_at, u.is_active,
		 u.created_at, u.updated_at
		FROM sessions s
		INNER JOIN users  u ON u.id = s.user_id
		WHERE s.id = $1
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	var (
		emailVerifiedAt    sql.NullTime
		maxRenewalDuration sql.NullInt64
		avatarURL          sql.NullString
	)

	row := s.db.QueryRowContext(ctx, query, sessionID)
	err := row.Scan(
		&session.ID,
		&session.UserID,
		&session.UserAgent,
		&session.IP,
		&session.ExpiresAt,
		&session.LastUsed,
		&session.Version,
		&session.CreatedAt,
		&session.UpdatedAt,
		&session.RememberMe,
		&maxRenewalDuration,
		&user.ID,
		&user.Email,
		&avatarURL,
		&user.Role,
		&emailVerifiedAt,
		&user.IsActive,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil, err
		}
		return nil, nil, err
	}

	// Handle nullable fields
	if emailVerifiedAt.Valid {
		user.EmailVerifiedAt = &emailVerifiedAt.Time
	}

	if maxRenewalDuration.Valid {
		session.MaxRenewalDuration = maxRenewalDuration.Int64
	}

	if avatarURL.Valid {
		user.AvatarURL = avatarURL.String
	}

	return &session, &user, nil
}

func (s *SessionModel) GetSessionsByUserID(
	ctx context.Context,
	userID string,
	isAdmin bool,
	paginateQuery PaginateQueryFilter,
) ([]Session, Metadata, error) {
	var sessions []Session
	var totalRecords int

	query := `
		SELECT count(*) OVER(), id, user_id, user_agent, ip, expires_at, remember_me, last_used, created_at,updated_at
		FROM sessions
	`
	if !isAdmin {
		query += ` WHERE user_id = $1`
	}

	query += fmt.Sprintf(` ORDER BY %s %s LIMIT $2 OFFSET $3`, paginateQuery.SortColumn(), paginateQuery.SortDirection())

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	var rows *sql.Rows
	var err error
	if !isAdmin {
		rows, err = s.db.QueryContext(ctx, query, userID, paginateQuery.Limit(), paginateQuery.Offset())
	} else {
		rows, err = s.db.QueryContext(ctx, query, paginateQuery.Limit(), paginateQuery.Offset())
	}
	if err != nil {
		return nil, Metadata{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var session Session
		err := rows.Scan(
			&totalRecords,
			&session.ID,
			&session.UserID,
			&session.UserAgent,
			&session.IP,
			&session.ExpiresAt,
			&session.RememberMe,
			&session.LastUsed,
			&session.CreatedAt,
			&session.UpdatedAt,
		)
		if err != nil {
			return nil, Metadata{}, err
		}
		sessions = append(sessions, session)
	}

	if err = rows.Err(); err != nil {
		return nil, Metadata{}, err
	}

	metadata := calculateMetadata(totalRecords, paginateQuery.Page, paginateQuery.PageSize)

	return sessions, metadata, nil
}

func (s *SessionModel) ExtendSessionAndGenerateRefreshToken(
	ctx context.Context,
	session *Session,
	tokenMaker auth.AuthToken,
	rememberPeriod time.Duration,
) (string, error) {
	// Check if RememberMe is enabled
	if !session.RememberMe {
		return "", ErrSessionCannotBeExtends
	}

	// Calculate the maximum allowed expiration time
	maxRenewalTime := session.CreatedAt.Add(time.Duration(session.MaxRenewalDuration) * time.Second)

	// Calculate the new expiration time (current time + remember period)
	newExpiresAt := time.Now().Add(rememberPeriod)

	// Ensure the new expiration time does not exceed the maximum renewal duration
	if newExpiresAt.After(maxRenewalTime) {
		newExpiresAt = maxRenewalTime
	}

	var version int
	// Update the session expiration time and refresh token hash in the database
	updateQuery := `UPDATE sessions SET expires_at = $1, version = version + 1 WHERE id = $2 AND version = $3 RETURNING version`
	err := s.db.QueryRowContext(ctx, updateQuery, newExpiresAt, session.ID, session.Version).Scan(&version)
	if err != nil {
		return "", fmt.Errorf("failed to extend session: %w", err)
	}

	// Generate a new refresh token
	newRefreshToken, err := tokenMaker.GenerateRefreshToken(session.ID, version, rememberPeriod)
	if err != nil {
		return "", fmt.Errorf("failed to generate refresh token: %w", err)
	}

	// Update the session's ExpiresAt field in memory
	session.ExpiresAt = newExpiresAt

	// Return the new refresh token (unhashed) to the client
	return newRefreshToken, nil
}

func (s *SessionModel) UpdateLastUsed(ctx context.Context, sessionID string) error {
	query := `UPDATE sessions SET last_used = NOW() WHERE id = $1`
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()
	_, err := s.db.ExecContext(ctx, query, sessionID)
	return err
}
