package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v9"
	"go.uber.org/zap"
)

// RedisService provides Redis operations for network configuration
type RedisService struct {
	client *redis.Client
	logger *zap.Logger
}

// NewRedisService creates a new Redis service
func NewRedisService(client *redis.Client, logger *zap.Logger) *RedisService {
	return &RedisService{
		client: client,
		logger: logger,
	}
}

// StoreConfig stores network configuration in Redis
func (rs *RedisService) StoreConfig(ctx context.Context, key string, config interface{}, ttl time.Duration) error {
	data, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	err = rs.client.Set(ctx, key, data, ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to store config in Redis: %w", err)
	}

	rs.logger.Info("Config stored in Redis",
		zap.String("key", key),
		zap.Duration("ttl", ttl))
	return nil
}

// GetConfig retrieves network configuration from Redis
func (rs *RedisService) GetConfig(ctx context.Context, key string, config interface{}) error {
	data, err := rs.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return fmt.Errorf("config not found: %s", key)
		}
		return fmt.Errorf("failed to get config from Redis: %w", err)
	}

	err = json.Unmarshal([]byte(data), config)
	if err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return nil
}

// DeleteConfig removes network configuration from Redis
func (rs *RedisService) DeleteConfig(ctx context.Context, key string) error {
	err := rs.client.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to delete config from Redis: %w", err)
	}

	rs.logger.Info("Config deleted from Redis", zap.String("key", key))
	return nil
}

// StoreConfigHistory stores configuration change history
func (rs *RedisService) StoreConfigHistory(ctx context.Context, configID string, history interface{}) error {
	key := fmt.Sprintf("config_history:%s", configID)

	data, err := json.Marshal(history)
	if err != nil {
		return fmt.Errorf("failed to marshal history: %w", err)
	}

	// Add to list with timestamp as score
	score := float64(time.Now().UnixNano()) / float64(time.Second)
	err = rs.client.ZAdd(ctx, key, redis.Z{
		Score:  score,
		Member: string(data),
	}).Err()
	if err != nil {
		return fmt.Errorf("failed to store history in Redis: %w", err)
	}

	// Keep only last 100 entries
	err = rs.client.ZRemRangeByRank(ctx, key, 0, -101).Err()
	if err != nil {
		rs.logger.Warn("Failed to trim history", zap.Error(err))
	}

	return nil
}

// GetConfigHistory retrieves configuration history
func (rs *RedisService) GetConfigHistory(ctx context.Context, configID string, limit int) ([]string, error) {
	key := fmt.Sprintf("config_history:%s", configID)

	// Get entries sorted by score (newest first)
	entries, err := rs.client.ZRevRange(ctx, key, 0, int64(limit-1)).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get history from Redis: %w", err)
	}

	return entries, nil
}

// StoreNetworkStatus stores network status information
func (rs *RedisService) StoreNetworkStatus(ctx context.Context, status interface{}) error {
	key := "network_status:current"

	data, err := json.Marshal(status)
	if err != nil {
		return fmt.Errorf("failed to marshal status: %w", err)
	}

	err = rs.client.Set(ctx, key, data, 5*time.Minute).Err()
	if err != nil {
		return fmt.Errorf("failed to store status in Redis: %w", err)
	}

	return nil
}

// GetNetworkStatus retrieves current network status
func (rs *RedisService) GetNetworkStatus(ctx context.Context, status interface{}) error {
	key := "network_status:current"

	data, err := rs.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return fmt.Errorf("network status not found")
		}
		return fmt.Errorf("failed to get status from Redis: %w", err)
	}

	err = json.Unmarshal([]byte(data), status)
	if err != nil {
		return fmt.Errorf("failed to unmarshal status: %w", err)
	}

	return nil
}

// AcquireLock acquires a distributed lock
func (rs *RedisService) AcquireLock(ctx context.Context, key, value string, ttl time.Duration) (bool, error) {
	err := rs.client.SetNX(ctx, key, value, ttl).Err()
	if err != nil {
		return false, fmt.Errorf("failed to acquire lock: %w", err)
	}
	return err == nil, nil
}

// ReleaseLock releases a distributed lock
func (rs *RedisService) ReleaseLock(ctx context.Context, key, value string) error {
	// Use Lua script to ensure we only delete the lock if it has the same value
	script := `
		if redis.call("GET", KEYS[1]) == ARGV[1] then
			return redis.call("DEL", KEYS[1])
		else
			return 0
		end
	`

	err := rs.client.Eval(ctx, script, []string{key}, value).Err()
	if err != nil {
		return fmt.Errorf("failed to release lock: %w", err)
	}

	return nil
}

// HealthCheck performs Redis health check
func (rs *RedisService) HealthCheck(ctx context.Context) error {
	err := rs.client.Ping(ctx).Err()
	if err != nil {
		return fmt.Errorf("Redis health check failed: %w", err)
	}
	return nil
}

// BatchStore performs batch operations
func (rs *RedisService) BatchStore(ctx context.Context, items map[string]interface{}) error {
	pipe := rs.client.Pipeline()

	for key, value := range items {
		data, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("failed to marshal value for key %s: %w", key, err)
		}
		pipe.Set(ctx, key, data, 0)
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to execute batch operations: %w", err)
	}

	return nil
}

// GetKeysByPattern retrieves keys matching a pattern
func (rs *RedisService) GetKeysByPattern(ctx context.Context, pattern string) ([]string, error) {
	iter := rs.client.Scan(ctx, 0, pattern, 100).Iterator()
	var keys []string

	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}

	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan keys: %w", err)
	}

	return keys, nil
}

// CleanupOldData removes old configuration data
func (rs *RedisService) CleanupOldData(ctx context.Context, maxAge time.Duration) error {
	patterns := []string{
		"config_history:*",
		"network_status:*",
		"config_lock:*",
	}

	for _, pattern := range patterns {
		keys, err := rs.GetKeysByPattern(ctx, pattern)
		if err != nil {
			rs.logger.Warn("Failed to get keys for cleanup",
				zap.String("pattern", pattern),
				zap.Error(err))
			continue
		}

		// Check TTL and remove expired keys
		pipe := rs.client.Pipeline()
		for _, key := range keys {
			pipe.PTTL(ctx, key)
		}

		cmds, err := pipe.Exec(ctx)
		if err != nil {
			rs.logger.Warn("Failed to check TTL for keys", zap.Error(err))
			continue
		}

		for i, key := range keys {
			if ttlCmd, ok := cmds[i].(*redis.DurationCmd); ok {
				if ttl, err := ttlCmd.Result(); err == nil && ttl < 0 {
					pipe.Del(ctx, key)
				}
			}
		}

		_, err = pipe.Exec(ctx)
		if err != nil {
			rs.logger.Warn("Failed to cleanup expired keys", zap.Error(err))
		}
	}

	return nil
}
