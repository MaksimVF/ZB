package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"balance-service/internal/config"
	"balance-service/internal/handlers"
	"balance-service/internal/repository"
	"balance-service/internal/service"

	"github.com/redis/go-redis/v9"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize logger
	config.SetupLogging(cfg.Environment)

	// Initialize Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	// Test Redis connection
	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redisClient.Close()

	// Initialize repository
	balanceRepo := repository.NewRedisBalanceRepository(redisClient)

	// Initialize service
	balanceService := service.NewBalanceService(balanceRepo)
	balanceService.SetAdminKey(cfg.AdminKey)

	// Initialize handlers
	balanceHandlers := handlers.NewBalanceHandlers(balanceService)

	// Create HTTP server with routing
	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/health", balanceHandlers.HealthHandler)

	// Balance endpoints
	mux.HandleFunc("/api/v1/balance", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			balanceHandlers.GetBalanceHandler(w, r)
		case http.MethodPost:
			balanceHandlers.AdjustBalanceHandler(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Reservation endpoints
	mux.HandleFunc("/api/v1/balance/reserve", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			balanceHandlers.ReserveBalanceHandler(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/v1/balance/commit", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			balanceHandlers.CommitReservationHandler(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/v1/balance/cancel", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			balanceHandlers.CancelReservationHandler(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Admin endpoints
	mux.HandleFunc("/api/v1/admin/balances", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			balanceHandlers.GetUserBalancesHandler(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Start HTTP server
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler: mux,
	}

	// Start server in goroutine
	go func() {
		log.Printf("Balance Service starting on port %d", cfg.HTTPPort)
		log.Printf("Health check: http://localhost:%d/health", cfg.HTTPPort)
		log.Printf("API endpoints:")
		log.Printf("  GET    /api/v1/balance?user_id=<id>")
		log.Printf("  POST   /api/v1/balance")
		log.Printf("  POST   /api/v1/balance/reserve")
		log.Printf("  POST   /api/v1/balance/commit")
		log.Printf("  POST   /api/v1/balance/cancel")
		log.Printf("  GET    /api/v1/admin/balances?limit=<n>")

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Setup graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit
	log.Println("Shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}
