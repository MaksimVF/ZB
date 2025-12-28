package storage

import (
	"sync"
	"time"
)

// Cache interface for caching routing decisions
type Cache interface {
	Get(key string) (interface{}, bool)
	Set(key string, value interface{}, ttl time.Duration)
	Delete(key string)
	Clear()
}

// LRUCache implements a simple LRU cache
type LRUCache struct {
	data    map[string]*CacheEntry
	order   []string
	maxSize int
	ttl     time.Duration
	mutex   sync.RWMutex
}

// CacheEntry represents a cache entry
type CacheEntry struct {
	Value       interface{}
	CreatedAt   time.Time
	AccessCount int
}

// NewLRUCache creates a new LRU cache
func NewLRUCache(maxSize int, ttl time.Duration) *LRUCache {
	return &LRUCache{
		data:    make(map[string]*CacheEntry),
		order:   make([]string, 0),
		maxSize: maxSize,
		ttl:     ttl,
	}
}

// Get retrieves a value from the cache
func (c *LRUCache) Get(key string) (interface{}, bool) {
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
func (c *LRUCache) Set(key string, value interface{}, ttl time.Duration) {
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
		Value:       value,
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
