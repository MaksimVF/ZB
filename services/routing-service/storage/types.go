package storage

import (
	"context"
	"sync"
	"time"

	"github.com/go-redis/redis/v9"
	
	routing "github.com/MaksimVF/ZB/services/routing-service/routing"
)

// RedisClient interface for Redis operations
type RedisClient interface {
	Ping(ctx context.Context) *redis.StatusCmd
	HSet(ctx context.Context, key string, values ...interface{}) *redis.IntCmd
	HGet(ctx context.Context, key, field string) *redis.StringCmd
	SAdd(ctx context.Context, key string, members ...interface{}) *redis.IntCmd
	SMembers(ctx context.Context, key string) *redis.StringSliceCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
	Exists(ctx context.Context, keys ...string) *redis.IntCmd
	Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd
}

// RedisStorage implements storage interface using Redis
type RedisStorage struct {
	client RedisClient
}

// NewRedisStorage creates a new Redis storage instance
func NewRedisStorage(client RedisClient) *RedisStorage {
	return &RedisStorage{
		client: client,
	}
}

// Cache interface for caching routing decisions
type Cache interface {
	Get(key string) (*routing.RoutingResponse, bool)
	Set(key string, response *routing.RoutingResponse)
	Delete(key string)
	Clear()
}

// LRUCache implements a simple LRU cache
type LRUCache struct {
	data   map[string]*CacheEntry
	order  []string
	maxSize int
	ttl    time.Duration
mutex   sync.RWMutex
}

// CacheEntry represents a cache entry
type CacheEntry struct {
	Value       *routing.RoutingResponse
	CreatedAt   time.Time
	AccessCount int
}

// NewLRUCache creates a new LRU cache
func NewLRUCache(maxSize int, ttl time.Duration) *LRUCache {
	return &LRUCache{
		data:   make(map[string]*CacheEntry),
		order:  make([]string, 0),
		maxSize: maxSize,
		ttl:    ttl,
	}
}

// Get retrieves a value from the cache
func (c *LRUCache) Get(key string) (*routing.RoutingResponse, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	entry, found := c.data[key]
	if !found {
		return nil, false
	}

	// Check if expired
	if time.Since(entry.CreatedAt) > c.ttl {
		delete(c.data, key)
		c.removeFromOrder(key)
		return nil, false
	}

	// Update access count and move to end (most recently used)
	entry.AccessCount++
	c.moveToEnd(key)

	return entry.Value, true
}

// Set stores a value in the cache
func (c *LRUCache) Set(key string, response *routing.RoutingResponse) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Remove existing entry if present
	if _, exists := c.data[key]; exists {
		c.removeFromOrder(key)
	}

	// Check if cache is full
	if len(c.data) >= c.maxSize {
		c.removeLRU()
	}

	// Add new entry
	c.data[key] = &CacheEntry{
		Value:       response,
		CreatedAt:   time.Now(),
		AccessCount: 1,
	}
	c.order = append(c.order, key)
}

// Delete removes a key from the cache
func (c *LRUCache) Delete(key string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	delete(c.data, key)
	c.removeFromOrder(key)
}

// Clear removes all entries from the cache
func (c *LRUCache) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.data = make(map[string]*CacheEntry)
	c.order = make([]string, 0)
}

// removeFromOrder removes a key from the order slice
func (c *LRUCache) removeFromOrder(key string) {
	for i, k := range c.order {
		if k == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			break
		}
	}
}

// moveToEnd moves a key to the end of the order slice (most recently used)
func (c *LRUCache) moveToEnd(key string) {
	c.removeFromOrder(key)
	c.order = append(c.order, key)
}

// removeLRU removes the least recently used entry
func (c *LRUCache) removeLRU() {
	if len(c.order) > 0 {
		lruKey := c.order[0]
		delete(c.data, lruKey)
		c.order = c.order[1:]
	}
}