package cache

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"time"

	"github.com/devphaseX/buyr-api.git/internal/store"
	"github.com/redis/go-redis/v9"
)

type TokenScope string

const (
	ActivationTokenScope        TokenScope = "activation"
	ForgetPasswordTokenScope    TokenScope = "forget_password"
	Login2faTokenScope          TokenScope = "login_2fa"
	ChangePassword2faTokenScope TokenScope = "change_password_2fa"
	ChangeEmail2faTokenScope    TokenScope = "change_email_2fa"
	ChangeEmailTokenScope       TokenScope = "change_email"
)

var (
	UserExpTime = time.Minute
)

type Token struct {
	Plaintext string     `json:"-"`
	Hash      []byte     `json:"hash"`
	UserID    string     `json:"user_id"`
	Expiry    time.Time  `json:"expiry"`
	Scope     TokenScope `json:"scope"`
	Data      []byte     `json:"data"`
}

type TokenStore interface {
	New(userID string, ttl time.Duration, scope TokenScope, data []byte) (*Token, error)
	Insert(ctx context.Context, token *Token) error
	Get(ctx context.Context, scope TokenScope, tokenKey string) (*Token, error)
	DeleteAllForUser(ctx context.Context, scope TokenScope, userID string) error
}

type UserStore interface {
	Get(ctx context.Context, userID string) (*store.User, error)
	Set(ctx context.Context, user *store.User) error
}

type Storage struct {
	Tokens TokenStore
	Users  UserStore
}

func NewRedisStorage(rdb *redis.Client) *Storage {
	return &Storage{
		Tokens: NewRedisTokenModel(rdb),
		Users:  NewRedisUserModel(rdb),
	}
}

func generateToken(userID string, ttl time.Duration, scope TokenScope, data []byte) (*Token, error) {
	token := &Token{
		UserID: userID,
		Expiry: time.Now().Add(ttl),
		Scope:  scope,
		Data:   data,
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
