package storage

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStorage implements storage using Redis
type RedisStorage struct {
	client *redis.Client
}

// NewRedisStorage creates a new Redis storage
func NewRedisStorage(client *redis.Client) *RedisStorage {
	return &RedisStorage{client: client}
}

// Store stores a value with key
func (r *RedisStorage) Store(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return r.client.Set(ctx, key, data, ttl).Err()
}

// Get retrieves a value by key
func (r *RedisStorage) Get(ctx context.Context, key string, dest interface{}) error {
	data, err := r.client.Get(ctx, key).Result()
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(data), dest)
}

// Delete deletes a key
func (r *RedisStorage) Delete(ctx context.Context, key string) error {
	return r.client.Del(ctx, key).Err()
}
