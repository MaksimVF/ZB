package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Simple configuration structure
type Config struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Mode         string    `json:"mode"`
	Version      int       `json:"version"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	HeadEndpoint string    `json:"head_endpoint"`
}

// In-memory storage (replace with Redis in production)
var configs = make(map[string]Config)
var configMutex = make(chan struct{}, 1)

func main() {
	// Initialize basic configuration
	configMutex <- struct{}{}
	configs["default"] = Config{
		ID:           "default",
		Name:         "Default Configuration",
		Mode:         "direct",
		Version:      1,
		Status:       "active",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		HeadEndpoint: "grpc://localhost:50055",
	}
	<-configMutex

	// Setup router
	r := chi.NewRouter()

	// Global middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))

	// Health check
	r.Get("/health", healthHandler)
	r.Get("/", infoHandler)

	// API routes
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/configs", getConfigsHandler)
		r.Get("/configs/{id}", getConfigHandler)
		r.Post("/configs", createConfigHandler)
		r.Put("/configs/{id}", updateConfigHandler)
		r.Delete("/configs/{id}", deleteConfigHandler)
	})

	// Start server
	port := getEnv("PORT", "50060")
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Printf("Network Config Service v2.0 starting on port %s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down Network Config Service...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	log.Println("Network Config Service stopped")
}

// Handler functions
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
		"service":   "network-config",
		"version":   "2.0.0",
		"mode":      "simple",
	})
}

func infoHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	info := map[string]interface{}{
		"name":        "Network Config Service",
		"version":     "2.0.0",
		"description": "Centralized network configuration management (Simple Version)",
		"mode":        "simplified",
		"endpoints": []string{
			"GET /health - Health check",
			"GET /api/v1/configs - List configurations",
			"GET /api/v1/configs/{id} - Get configuration",
			"POST /api/v1/configs - Create configuration",
			"PUT /api/v1/configs/{id} - Update configuration",
			"DELETE /api/v1/configs/{id} - Delete configuration",
		},
		"documentation": "https://github.com/MaksimVF/ZB/services/network-config",
	}

	json.NewEncoder(w).Encode(info)
}

func getConfigsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	configMutex <- struct{}{}
	defer func() { <-configMutex }()

	// Convert map to slice
	configList := make([]Config, 0, len(configs))
	for _, config := range configs {
		configList = append(configList, config)
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    configList,
		"count":   len(configList),
	})
}

func getConfigHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	configID := chi.URLParam(r, "id")
	if configID == "" {
		http.Error(w, "Config ID is required", http.StatusBadRequest)
		return
	}

	configMutex <- struct{}{}
	defer func() { <-configMutex }()

	config, exists := configs[configID]
	if !exists {
		http.Error(w, "Configuration not found", http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    config,
	})
}

func createConfigHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var newConfig Config
	if err := json.NewDecoder(r.Body).Decode(&newConfig); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if newConfig.Name == "" || newConfig.Mode == "" {
		http.Error(w, "Name and Mode are required", http.StatusBadRequest)
		return
	}

	// Set metadata
	now := time.Now()
	newConfig.ID = "config_" + strconv.FormatInt(now.UnixNano(), 36)
	newConfig.CreatedAt = now
	newConfig.UpdatedAt = now
	newConfig.Version = 1
	newConfig.Status = "active"

	configMutex <- struct{}{}
	configs[newConfig.ID] = newConfig
	<-configMutex

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    newConfig,
		"message": "Configuration created successfully",
	})
}

func updateConfigHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	configID := chi.URLParam(r, "id")
	if configID == "" {
		http.Error(w, "Config ID is required", http.StatusBadRequest)
		return
	}

	var updatedConfig Config
	if err := json.NewDecoder(r.Body).Decode(&updatedConfig); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	configMutex <- struct{}{}
	defer func() { <-configMutex }()

	existingConfig, exists := configs[configID]
	if !exists {
		http.Error(w, "Configuration not found", http.StatusNotFound)
		return
	}

	// Update metadata
	updatedConfig.ID = configID
	updatedConfig.CreatedAt = existingConfig.CreatedAt
	updatedConfig.UpdatedAt = time.Now()
	updatedConfig.Version = existingConfig.Version + 1
	updatedConfig.Status = "active"

	configs[configID] = updatedConfig

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    updatedConfig,
		"message": "Configuration updated successfully",
	})
}

func deleteConfigHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	configID := chi.URLParam(r, "id")
	if configID == "" {
		http.Error(w, "Config ID is required", http.StatusBadRequest)
		return
	}

	configMutex <- struct{}{}
	defer func() { <-configMutex }()

	_, exists := configs[configID]
	if !exists {
		http.Error(w, "Configuration not found", http.StatusNotFound)
		return
	}

	// Soft delete - mark as deprecated
	configs[configID].Status = "deprecated"
	configs[configID].UpdatedAt = time.Now()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Configuration deprecated successfully",
	})
}

// Helper functions
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
