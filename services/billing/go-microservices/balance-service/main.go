package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"balance-service/internal/handlers"
	"balance-service/internal/service"
)

var (
	// Metrics
	totalRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status"},
	)

	balanceGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "user_balance",
			Help: "User balance",
		},
		[]string{"user_id"},
	)
)

func init() {
	// Register metrics
	prometheus.MustRegister(totalRequests)
	prometheus.MustRegister(balanceGauge)
}

func main() {
	// Initialize logger
	logger, _ := zap.NewProduction()
	defer logger.Sync()
	sugar := logger.Sugar()

	// Load configuration
	config := loadConfig()

	// Initialize database
	db, err := initDB(config.DatabaseURL)
	if err != nil {
		sugar.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Initialize Redis
	redisClient := initRedis(config.RedisURL)
	if redisClient == nil {
		sugar.Fatal("Failed to connect to Redis")
	}
	defer redisClient.Close()

	// Initialize balance service
	balanceService := service.NewBalanceService(db, redisClient, logger)

	// Initialize HTTP server
	r := gin.New()

	// Add middleware
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(prometheusMiddleware())

	// Initialize handlers
	balanceHandler := handlers.NewBalanceHandler(balanceService, logger)

	// Define routes
	r.GET("/health", healthCheckHandler)
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	api := r.Group("/api/v1")
	{
		// Balance endpoints
		api.GET("/balance/:userId", balanceHandler.GetBalance)
		api.POST("/balance/adjust", balanceHandler.AdjustBalance)

		// Statistics endpoints
		api.GET("/stats/balance", balanceHandler.GetBalanceStats)
	}

	// Start server
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", config.HTTPPort),
		Handler: r,
	}

	go func() {
		sugar.Infof("Balance service starting on port %d", config.HTTPPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			sugar.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	sugar.Info("Shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		sugar.Fatalf("Server forced to shutdown: %v", err)
	}

	sugar.Info("Server exited")
}

func loadConfig() *Config {
	return &Config{
		DatabaseURL: getEnvOrDefault("DATABASE_URL", "postgres://postgres:password@localhost:5432/balance_service"),
		RedisURL:    getEnvOrDefault("REDIS_URL", "redis://localhost:6379"),
		HTTPPort:    getEnvAsInt("HTTP_PORT", 8081),
	}
}

func initDB(databaseURL string) (*sql.DB, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

func initRedis(redisURL string) *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:     redisURL,
		Password: "",
		DB:       0,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Printf("Failed to connect to Redis: %v", err)
		return nil
	}

	return rdb
}

func healthCheckHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"service":   "balance-service",
		"timestamp": time.Now().Unix(),
	})
}

func prometheusMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		duration := time.Since(start).Seconds()
		totalRequests.WithLabelValues(c.Request.Method, c.FullPath(), fmt.Sprintf("%d", c.Writer.Status())).Inc()

		// Custom metric for balance operations
		if c.FullPath() == "/api/v1/balance/:userId" && c.Request.Method == "GET" {
			userID := c.Param("userId")
			if userID != "" {
				balanceGauge.WithLabelValues(userID).Set(0) // This would be updated with actual balance
			}
		}
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intValue, err := parseInt(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func parseInt(s string) (int, error) {
	var result int
	if _, err := fmt.Sscanf(s, "%d", &result); err != nil {
		return 0, err
	}
	return result, nil
}

// Config represents the application configuration
type Config struct {
	DatabaseURL string
	RedisURL    string
	HTTPPort    int
}
