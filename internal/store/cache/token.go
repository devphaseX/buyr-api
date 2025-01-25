package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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

func createTokenKey(scope, identifier string) string {
	return fmt.Sprintf("%s:%s", scope, identifier)
}

func createUserTokenSetKey(scope, userID string) string {
	return fmt.Sprintf("%s:user_tokens:%s", scope, userID)
}

func (m *RedisTokenModel) New(userID string, ttl time.Duration, scope string, data []byte) (*Token, error) {
	token, err := generateToken(userID, ttl, scope, data)
	if err != nil {
		return nil, err
	}
	return token, err
}

func (m *RedisTokenModel) Insert(ctx context.Context, token *Token, key ...string) error {
	// Create a Redis key using the scope and the token's hash
	redisKey := createTokenKey(token.Scope, hex.EncodeToString(token.Hash))

	byte, err := json.Marshal(token)
	if err != nil {
		return err
	}
	// Store the token hash and expiry in Redis
	err = m.client.Set(ctx, redisKey, byte, time.Until(token.Expiry).Abs()).Err()
	if err != nil {
		return fmt.Errorf("failed to insert token into Redis: %w", err)
	}

	// Store the token hash in a set associated with the userID
	userTokenSetKey := createUserTokenSetKey(token.Scope, token.UserID)
	err = m.client.SAdd(ctx, userTokenSetKey, hex.EncodeToString(token.Hash)).Err()
	if err != nil {
		return fmt.Errorf("failed to add token hash to user token set: %w", err)
	}

	// Set an expiration on the user's token set
	err = m.client.Expire(ctx, userTokenSetKey, time.Until(token.Expiry).Abs()).Err()
	if err != nil {
		return fmt.Errorf("failed to set expiration on user token set: %w", err)
	}

	return nil
}

func (s *RedisTokenModel) Get(ctx context.Context, scope, tokenKey string) (*Token, error) {
	// Hash the tokenKey
	hash := sha256.Sum256([]byte(tokenKey))
	hashedKey := hex.EncodeToString(hash[:]) //
	key := createTokenKey(scope, hashedKey)
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

	return token, nil
}

func (m *RedisTokenModel) DeleteAllForUser(ctx context.Context, scope string, userID string) error {
	// Create the key for the user's token set
	userTokenSetKey := createUserTokenSetKey(scope, userID)

	// Fetch all token hashes associated with the user
	tokenHashes, err := m.client.SMembers(ctx, userTokenSetKey).Result()
	if err != nil {
		return fmt.Errorf("failed to fetch token hashes for user: %w", err)
	}

	// Delete each token using its hashed key
	for _, tokenHash := range tokenHashes {
		redisKey := createTokenKey(scope, tokenHash)
		err := m.client.Del(ctx, redisKey).Err()
		if err != nil {
			return fmt.Errorf("failed to delete token with hash %s: %w", tokenHash, err)
		}
	}

	// Delete the user's token set
	err = m.client.Del(ctx, userTokenSetKey).Err()
	if err != nil {
		return fmt.Errorf("failed to delete user token set: %w", err)
	}

	return nil
}

func (m *RedisTokenModel) CleanupOrphanedSets(ctx context.Context) error {
	// Fetch all user token set keys
	keys, err := m.client.Keys(ctx, "*:user_tokens:*").Result()
	if err != nil {
		return fmt.Errorf("failed to fetch user token set keys: %w", err)
	}

	// Check each set and delete it if it's empty
	for _, key := range keys {
		count, err := m.client.SCard(ctx, key).Result()
		if err != nil {
			return fmt.Errorf("failed to check set size for key %s: %w", key, err)
		}

		if count == 0 {
			err := m.client.Del(ctx, key).Err()
			if err != nil {
				return fmt.Errorf("failed to delete empty set %s: %w", key, err)
			}
		}
	}

	return nil
}
