





package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/mux"
)

var redisClient = redis.NewClient(&redis.Options{
	Addr: "redis:6379",
})

type SecurityConfig struct {
	ContentFilteringEnabled bool `json:"content_filtering_enabled"`
	AuditLoggingEnabled    bool `json:"audit_logging_enabled"`
	DataIsolationEnabled   bool `json:"data_isolation_enabled"`
}

func GetSecurityConfig(w http.ResponseWriter, r *http.Request) {
	// Get client ID from context
	clientID := r.Context().Value("client_id").(string)
	if clientID == "" {
		http.Error(w, "Client ID required", http.StatusUnauthorized)
		return
	}

	// Get security config from Redis
	ctx := r.Context()
	configKey := "client:" + clientID + ":security_config"

	val, err := redisClient.Get(ctx, configKey).Result()
	if err == redis.Nil {
		// Return default config if not found
		defaultConfig := SecurityConfig{
			ContentFilteringEnabled: true,
			AuditLoggingEnabled:    true,
			DataIsolationEnabled:   true,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(defaultConfig)
		return
	} else if err != nil {
		http.Error(w, "Failed to get security config", http.StatusInternalServerError)
		return
	}

	// Parse and return config
	var config SecurityConfig
	err = json.Unmarshal([]byte(val), &config)
	if err != nil {
		http.Error(w, "Failed to parse security config", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

func UpdateSecurityConfig(w http.ResponseWriter, r *http.Request) {
	// Get client ID from context
	clientID := r.Context().Value("client_id").(string)
	if clientID == "" {
		http.Error(w, "Client ID required", http.StatusUnauthorized)
		return
	}

	// Parse request body
	var config SecurityConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate config
	if config.ContentFilteringEnabled && config.AuditLoggingEnabled && config.DataIsolationEnabled {
		// At least one security feature must be enabled
		http.Error(w, "At least one security feature must be enabled", http.StatusBadRequest)
		return
	}

	// Store config in Redis
	ctx := r.Context()
	configKey := "client:" + clientID + ":security_config"

	configData, err := json.Marshal(config)
	if err != nil {
		http.Error(w, "Failed to save security config", http.StatusInternalServerError)
		return
	}

	err = redisClient.Set(ctx, configKey, configData, 0).Err()
	if err != nil {
		http.Error(w, "Failed to save security config", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}





