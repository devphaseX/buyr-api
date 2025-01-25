package cache

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	ScopeActivation = "activation"
)

type Token struct {
	Plaintext string    `json:"-"`
	Hash      []byte    `json:"hash"`
	UserID    string    `json:"user_id"`
	Expiry    time.Time `json:"expiry"`
	Scope     string    `json:"scope"`
	Data      []byte    `json:"data"`
}

type TokenStore interface {
	New(userID string, ttl time.Duration, scope string) (*Token, error)
	Insert(ctx context.Context, token *Token) error
	Get(ctx context.Context, scope, userID, tokenKey string) (*Token, error)
	DeleteAllForUser(ctx context.Context, scope string, userID string) error
}

type Storage struct {
	Tokens TokenStore
}

func NewRedisStorage(rdb *redis.Client) *Storage {
	return &Storage{
		Tokens: NewRedisTokenModel(rdb),
	}
}

func generateToken(userID string, ttl time.Duration, scope string) (*Token, error) {
	token := &Token{
		UserID: userID,
		Expiry: time.Now().Add(ttl),
		Scope:  scope,
	}

	randomBytes := make([]byte, 16)

	_, err := rand.Read(randomBytes)
	if err != nil {
		return nil, err
	}

	token.Plaintext = base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(randomBytes)

	hash := sha256.Sum256([]byte(token.Plaintext))
	token.Hash = hash[:]

	return token, nil
}
