



package config

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
)

// NetworkConfig represents the network configuration structure
type NetworkConfig struct {
	HeadEndpoint    string            `json:"head_endpoint"`
	NetworkMode    string            `json:"network_mode"`
	WGPeerPublic   string            `json:"wg_peer_public,omitempty"`
	WGAllowedIPs   string            `json:"wg_allowed_ips,omitempty"`
	SecurityToken  string            `json:"security_token"`
	RetryPolicy    RetryPolicy       `json:"retry_policy"`
	RateLimits     RateLimits        `json:"rate_limits"`
	LoadBalancing LoadBalancingConfig `json:"load_balancing"`
}

type RetryPolicy struct {
	Retries   int `json:"retries"`
	BackoffMs int `json:"backoff_ms"`
}

type RateLimits struct {
	MaxRequestsPerUser int `json:"max_requests_per_user"`
	MaxRequestsPerIP  int `json:"max_requests_per_ip"`
	WindowSeconds     int `json:"window_seconds"`
}

type LoadBalancingConfig struct {
	Mode           string   `json:"mode"`
	HeadEndpoints []string `json:"head_endpoints,omitempty"`
}

// NetworkConfigManager manages dynamic network configuration
type NetworkConfigManager struct {
	redisClient *redis.Client
	currentConfig NetworkConfig
	mutex        sync.RWMutex
}

// NewNetworkConfigManager creates a new config manager
func NewNetworkConfigManager(redisAddr string) *NetworkConfigManager {
	redisClient := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	return &NetworkConfigManager{
		redisClient: redisClient,
	}
}

// LoadConfig loads the current configuration from Redis
func (m *NetworkConfigManager) LoadConfig() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	ctx := context.Background()
	result, err := m.redisClient.Get(ctx, "network_config").Result()
	if err == redis.Nil {
		// Default config if not found
		m.currentConfig = NetworkConfig{
			HeadEndpoint: "grpc://head:50055",
			NetworkMode: "direct",
			SecurityToken: "default-token",
			RetryPolicy: RetryPolicy{
				Retries:   3,
				BackoffMs: 200,
			},
			RateLimits: RateLimits{
				MaxRequestsPerUser: 100,
				MaxRequestsPerIP:   1000,
				WindowSeconds:      60,
			},
			LoadBalancing: LoadBalancingConfig{
				Mode: "single",
			},
		}
		return nil
	} else if err != nil {
		return err
	}

	err = json.Unmarshal([]byte(result), &m.currentConfig)
	if err != nil {
		return err
	}

	return nil
}

// GetConfig returns the current configuration
func (m *NetworkConfigManager) GetConfig() NetworkConfig {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.currentConfig
}

// StartAutoReload starts the auto-reload goroutine
func (m *NetworkConfigManager) StartAutoReload(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			err := m.LoadConfig()
			if err != nil {
				log.Printf("Failed to reload config: %v", err)
			} else {
				log.Printf("Config reloaded successfully")
			}
		}
	}()
}


