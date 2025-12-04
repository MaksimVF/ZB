


package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

var (
	redisClient *redis.Client
	logger       *zap.Logger
	configMutex  sync.RWMutex
	currentConfig NetworkConfig
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

func init() {
	// Initialize logger
	var err error
	logger, err = zap.NewProduction()
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}

	// Initialize Redis client
	redisClient = redis.NewClient(&redis.Options{
		Addr: "redis:6379",
	})

	// Load initial config
	loadConfig()
}

func main() {
	router := mux.NewRouter()

	// API endpoints
	router.HandleFunc("/api/config", getConfig).Methods("GET")
	router.HandleFunc("/api/config", updateConfig).Methods("PUT")
	router.HandleFunc("/api/config/history", getConfigHistory).Methods("GET")

	// Health check
	router.HandleFunc("/health", healthCheck).Methods("GET")

	srv := &http.Server{
		Addr:    ":50060",
		Handler: router,
	}

	// Start auto-reload goroutine
	go autoReloadConfig()

	// Start HTTP server
	go func() {
		logger.Info("Starting Network Config Service on :50060")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Server failed", zap.Error(err))
		}
	}()

	// Graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	<-c

	logger.Info("Shutting down Network Config Service...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	logger.Info("Network Config Service stopped")
}

// loadConfig loads the current configuration from Redis
func loadConfig() {
	configMutex.Lock()
	defer configMutex.Unlock()

	ctx := context.Background()
	result, err := redisClient.Get(ctx, "network_config").Result()
	if err == redis.Nil {
		// Default config if not found
		currentConfig = NetworkConfig{
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
		return
	} else if err != nil {
		logger.Error("Failed to load config from Redis", zap.Error(err))
		return
	}

	err = json.Unmarshal([]byte(result), &currentConfig)
	if err != nil {
		logger.Error("Failed to parse config", zap.Error(err))
	}
}

// saveConfig saves the configuration to Redis
func saveConfig(cfg NetworkConfig) error {
	configMutex.Lock()
	defer configMutex.Unlock()

	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}

	ctx := context.Background()
	return redisClient.Set(ctx, "network_config", data, 0).Err()
}

// autoReloadConfig periodically checks for config updates
func autoReloadConfig() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		loadConfig()
	}
}

// getConfig returns the current configuration
func getConfig(w http.ResponseWriter, r *http.Request) {
	configMutex.RLock()
	defer configMutex.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(currentConfig)
}

// updateConfig updates the configuration
func updateConfig(w http.ResponseWriter, r *http.Request) {
	var newConfig NetworkConfig
	err := json.NewDecoder(r.Body).Decode(&newConfig)
	if err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if newConfig.HeadEndpoint == "" || newConfig.NetworkMode == "" || newConfig.SecurityToken == "" {
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	err = saveConfig(newConfig)
	if err != nil {
		http.Error(w, "Failed to save config", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// healthCheck returns the health status
func healthCheck(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	err := redisClient.Ping(ctx).Err()
	if err != nil {
		http.Error(w, "Redis unavailable", http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// getConfigHistory returns the configuration history (stub for now)
func getConfigHistory(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement config history from database
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode([]NetworkConfig{currentConfig})
}

