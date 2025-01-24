package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrExpiredToken          = errors.New("token has expired")
	ErrInvalidToken          = errors.New("token not valid")
	ErrInvalidOrExpiredToken = errors.New("token expired or invalid")
	ErrUnverifiableToken     = errors.New("token is unverifiable")
)

type AuthToken interface {
	GenerateAccessToken(userID string, sessionID string, expiry time.Duration) (string, error)
	GenerateRefreshToken(sessionID string, version int, expiry time.Duration) (string, error)
	ValidateAccessToken(tokenString string) (*AccessPayload, error)
	ValidateRefreshToken(tokenString string) (*RefreshPayload, error)
}

// Payload for access tokens
type AccessPayload struct {
	UserID    int64  `json:"user_id"`
	SessionID string `json:"session_id"`
	jwt.RegisteredClaims
}

func NewAccessPayload(userId int64, sessionId string, expiry time.Duration) *AccessPayload {
	return &AccessPayload{
		UserID:    userId,
		SessionID: sessionId,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiry)),
		},
	}
}

func (p *AccessPayload) Valid() error {
	if time.Now().After(p.ExpiresAt.Time) {
		return ErrExpiredToken
	}

	return nil
}

// Payload for refresh tokens
type RefreshPayload struct {
	SessionID string `json:"session_id"`
	Version   int    `json:"version"`
	jwt.RegisteredClaims
}

func NewRefreshPayload(sessionId string, version int, expiry time.Duration) *RefreshPayload {
	return &RefreshPayload{
		SessionID: sessionId,
		Version:   version,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiry)),
		},
	}
}

func (p *RefreshPayload) Valid() error {
	if time.Now().After(p.ExpiresAt.Time) {
		return ErrExpiredToken
	}

	return nil
}
