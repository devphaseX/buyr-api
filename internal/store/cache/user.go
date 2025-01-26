package cache

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/devphaseX/buyr-api.git/internal/store"
	"github.com/redis/go-redis/v9"
)

type RedisUserModel struct {
	client *redis.Client
}

func NewRedisUserModel(client *redis.Client) UserStore {
	return &RedisUserModel{client}
}

func createUserCacheKey(userID string) string {
	return fmt.Sprintf("user-%v", userID)
}

func (s *RedisUserModel) Get(ctx context.Context, userID string) (*store.User, error) {
	cacheKey := createUserCacheKey(userID)
	data, err := s.client.Get(ctx, cacheKey).Result()

	if err == redis.Nil {
		return nil, store.ErrRecordNotFound
	}

	if err != nil {
		return nil, err
	}

	user := &store.User{}
	if err := json.Unmarshal([]byte(data), user); err != nil {
		return nil, err
	}

	return user, nil
}

func (s *RedisUserModel) Set(ctx context.Context, user *store.User) error {
	cacheKey := createUserCacheKey(user.ID)

	json, err := json.Marshal(user)

	if err != nil {
		return err
	}

	return s.client.SetEx(ctx, cacheKey, json, UserExpTime).Err()
}
