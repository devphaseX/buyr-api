package cache

import (
	"context"
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

func (m *RedisTokenModel) New(userID string, ttl time.Duration, scope string) (*Token, error) {
	token, err := generateToken(userID, ttl, scope)
	if err != nil {
		return nil, err
	}

	err = m.Insert(token)
	if err != nil {
		return nil, err
	}
	return token, err
}

func (m *RedisTokenModel) Insert(token *Token) error {
	// Create a Redis key using the scope and user ID
	key := fmt.Sprintf("%s:%s", token.Scope, token.UserID)

	// Store the token hash and expiry in Redis
	err := m.client.Set(context.Background(), key, token.Hash, time.Until(token.Expiry)).Err()
	if err != nil {
		return fmt.Errorf("failed to insert token into Redis: %w", err)
	}

	return nil
}

func (m *RedisTokenModel) DeleteAllForUser(scope string, userID int64) error {
	// Create a Redis key using the scope and user ID
	key := fmt.Sprintf("%s:%d", scope, userID)

	// Delete the token from Redis
	err := m.client.Del(context.Background(), key).Err()
	if err != nil {
		return fmt.Errorf("failed to delete token from Redis: %w", err)
	}

	return nil
}
