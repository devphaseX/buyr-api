package auth

import (
	"encoding/base64"
	"fmt"
	"time"

	"github.com/aead/chacha20poly1305"
	"github.com/o1egl/paseto"
)

type PasetoToken struct {
	paseto     *paseto.V2
	accessKey  []byte
	refreshKey []byte
}

func NewPasetoToken(accessSecret, refreshSecret string) (*PasetoToken, error) {
	accessSecretByte, err := base64.StdEncoding.DecodeString(accessSecret)

	if err != nil {
		fmt.Println("Failed to decode access secret base64 key:", err)
		return nil, err
	}

	refreshSecretByte, err := base64.StdEncoding.DecodeString(refreshSecret)

	if err != nil {
		fmt.Println("Failed to decode refresh secret base64 key:", err)
		return nil, err
	}

	// Verify access key length
	if len(accessSecretByte) != chacha20poly1305.KeySize {
		return nil, fmt.Errorf("invalid access key size: must be exactly %d bytes", chacha20poly1305.KeySize)
	}

	// Verify refresh key length
	if len(refreshSecretByte) != chacha20poly1305.KeySize {
		return nil, fmt.Errorf("invalid refresh key size: must be exactly %d bytes", chacha20poly1305.KeySize)
	}

	return &PasetoToken{
		paseto:     paseto.NewV2(),
		accessKey:  accessSecretByte,
		refreshKey: refreshSecretByte,
	}, nil
}

// GenerateAccessToken creates a PASETO token for access
func (t *PasetoToken) GenerateAccessToken(userID string, sessionID string, accessExpiry time.Duration) (string, error) {
	payload := NewAccessPayload(userID, sessionID, accessExpiry)

	// Set token expiration

	// Create the token
	token, err := t.paseto.Encrypt(t.accessKey, payload, nil)
	if err != nil {
		return "", err
	}

	return token, nil
}

// GenerateRefreshToken creates a PASETO token for refresh
func (t *PasetoToken) GenerateRefreshToken(sessionID string, version int, refreshExpiry time.Duration) (string, error) {
	payload := NewRefreshPayload(sessionID, version, refreshExpiry)
	// Set token expiration

	// Create the token
	token, err := t.paseto.Encrypt(t.refreshKey, payload, nil)
	if err != nil {
		return "", err
	}

	return token, nil
}

// ValidateAccessToken validates a PASETO access token
func (t *PasetoToken) ValidateAccessToken(tokenString string) (*AccessPayload, error) {
	var payload AccessPayload

	// Decrypt and validate the token
	err := t.paseto.Decrypt(tokenString, t.accessKey, &payload, nil)
	if err != nil {
		return nil, ErrInvalidToken
	}

	return &payload, nil
}

// ValidateRefreshToken validates a PASETO refresh token
func (t *PasetoToken) ValidateRefreshToken(tokenString string) (*RefreshPayload, error) {
	var payload RefreshPayload

	// Decrypt and validate the token
	err := t.paseto.Decrypt(tokenString, t.refreshKey, &payload, nil)
	if err != nil {
		return nil, ErrInvalidToken
	}

	return &payload, nil
}
