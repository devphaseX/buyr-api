package cache

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisTokenModel struct {
	client *redis.Client
}

func NewRedisTokenModel(client *redis.Client) *RedisTokenModel {
	return &RedisTokenModel{client: client}
}

func createTokenKey(scope, userId string) string {
	return fmt.Sprintf("%s:%s", scope, userId)
}

func (m *RedisTokenModel) New(userID string, ttl time.Duration, scope string) (*Token, error) {
	token, err := generateToken(userID, ttl, scope)
	if err != nil {
		return nil, err
	}
	return token, err
}

func (m *RedisTokenModel) Insert(ctx context.Context, token *Token) error {
	// Create a Redis key using the scope and user ID
	key := createTokenKey(token.Scope, token.UserID)

	byte, err := json.Marshal(token)
	if err != nil {
		return err
	}
	// Store the token hash and expiry in Redis
	err = m.client.Set(ctx, key, byte, time.Until(token.Expiry).Abs()).Err()
	if err != nil {
		return fmt.Errorf("failed to insert token into Redis: %w", err)
	}

	return nil
}

func (s *RedisTokenModel) Get(ctx context.Context, scope, userID, tokenKey string) (*Token, error) {
	key := createTokenKey(scope, userID)
	data, err := s.client.Get(ctx, key).Result()

	if err == redis.Nil {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	token := &Token{}
	if err := json.Unmarshal([]byte(data), token); err != nil {
		return nil, err
	}

	hash := sha256.Sum256([]byte(tokenKey))

	if !bytes.Equal(token.Hash, hash[:]) {
		return nil, nil
	}

	return token, nil
}

func (m *RedisTokenModel) DeleteAllForUser(ctx context.Context, scope string, userID string) error {
	// Create a Redis key using the scope and user ID
	key := fmt.Sprintf("%s:%s", scope, userID)

	// Delete the token from Redis
	err := m.client.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to delete token from Redis: %w", err)
	}

	return nil
}
