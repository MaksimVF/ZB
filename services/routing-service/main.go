package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/MaksimVF/ZB/services/routing-service/config"
	grpcLocal "github.com/MaksimVF/ZB/services/routing-service/grpc"
	"github.com/MaksimVF/ZB/services/routing-service/http/handlers"
	httpMiddleware "github.com/MaksimVF/ZB/services/routing-service/http/middleware"
	"github.com/MaksimVF/ZB/services/routing-service/monitoring"
	pb "github.com/MaksimVF/ZB/services/routing-service/proto"
	routing "github.com/MaksimVF/ZB/services/routing-service/routing"
	"github.com/MaksimVF/ZB/services/routing-service/storage"
)

func main() {
	// Initialize configuration
	cfg := config.DefaultConfig()

	// Initialize logger
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatal(err)
	}
	defer logger.Sync()

	// Initialize Redis client
	redisClient := redis.NewClient(cfg.Redis)
	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		logger.Fatal("Failed to connect to Redis", zap.Error(err))
	}

	// Initialize storage layer
	cache := storage.NewLRUCache(cfg.Cache.MaxSize, time.Duration(cfg.Cache.TTL)*time.Second)

	// Initialize routing components
	policy := &routing.RoutingPolicy{
		DefaultStrategy:     "adaptive",
		EnableGeoRouting:    true,
		EnableLoadBalancing: true,
		EnableModelSpecific: true,
		EnablePredictive:    true,
		EnableAdaptive:      true,
		StrategyConfig:      make(map[string]string),
		PredictionWindow:    15,
		LoadGrowthFactor:    1.1,
		CapacityThreshold:   80.0,
	}

	// Initialize registry (simplified in-memory implementation)
	registry := NewInMemoryRegistry()

	// Initialize routing engine
	routingEngine := routing.NewRoutingEngine(policy, registry, cache, nil)

	// Initialize metrics
	metrics := monitoring.NewMetrics()
	metrics.Register()

	// Initialize rate limiter
	rateLimiter := httpMiddleware.NewRateLimiter(cfg)

	// Initialize NATS connection
	natsConn, err := nats.Connect(cfg.NATSURL)
	if err != nil {
		logger.Fatal("Failed to connect to NATS", zap.Error(err))
	}
	defer natsConn.Close()

	// Initialize handlers
	adminHandlers := handlers.NewAdminHandlers(routingEngine, registry, &InMemoryPolicyManager{policy: policy})
	webhookHandlers := handlers.NewWebhookHandlers(routingEngine, registry)
	websocketHandlers := handlers.NewWebSocketHandlers(routingEngine, registry)
	sseHandlers := handlers.NewSSEHandlers(routingEngine, registry)
	grpcHandlers := grpcLocal.NewGRPCHandlers(routingEngine, registry, &InMemoryPolicyManager{policy: policy})

	// Start background services
	go startBackgroundServices(sseHandlers, metrics, registry)

	// Start gRPC server
	go startGRPCServer(grpcHandlers, cfg, logger)

	// Start HTTP server
	startHTTPServer(adminHandlers, webhookHandlers, websocketHandlers, sseHandlers, metrics, rateLimiter, cfg, logger)
}

// startGRPCServer starts the gRPC server
func startGRPCServer(grpcHandlers *grpcLocal.GRPCHandlers, cfg *config.Config, logger *zap.Logger) {
	lis, err := net.Listen("tcp", cfg.GRPCPort)
	if err != nil {
		logger.Fatal("Failed to listen", zap.Error(err))
	}

	// Load TLS certificates
	serverCert, err := tls.LoadX509KeyPair(cfg.TLS.CertFile, cfg.TLS.KeyFile)
	if err != nil {
		logger.Fatal("Failed to load server certificates", zap.Error(err))
	}

	// Load CA certificate
	caCert, err := os.ReadFile(cfg.TLS.CAFile)
	if err != nil {
		logger.Fatal("Failed to read CA certificate", zap.Error(err))
	}

	// Create certificate pool
	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(caCert) {
		logger.Fatal("Failed to add CA certificate to pool")
	}

	// Create TLS configuration
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    certPool,
	}

	// Create gRPC server with TLS
	grpcServer := grpc.NewServer(
		grpc.Creds(credentials.NewTLS(tlsConfig)),
	)
	pb.RegisterRoutingServiceServer(grpcServer, grpcHandlers)

	logger.Info("Starting gRPC server with mTLS", zap.String("port", cfg.GRPCPort))
	if err := grpcServer.Serve(lis); err != nil {
		logger.Fatal("gRPC server failed", zap.Error(err))
	}
}

// startHTTPServer starts the HTTP server
func startHTTPServer(
	adminHandlers *handlers.AdminHandlers,
	webhookHandlers *handlers.WebhookHandlers,
	websocketHandlers *handlers.WebSocketHandlers,
	sseHandlers *handlers.SSEHandlers,
	metrics *monitoring.Metrics,
	rateLimiter *httpMiddleware.RateLimiter,
	cfg *config.Config,
	logger *zap.Logger,
) {
	router := chi.NewRouter()

	// Apply middlewares
	router.Use(httpMiddleware.SecurityHeadersMiddleware)
	router.Use(httpMiddleware.RequestIDMiddleware)
	router.Use(httpMiddleware.RealIPMiddleware)
	router.Use(httpMiddleware.LoggerMiddleware)
	router.Use(httpMiddleware.RecoverMiddleware)
	router.Use(httpMiddleware.CORSMiddleware([]string{"http://localhost:3000", "http://localhost:3001"}))
	router.Use(httpMiddleware.JWTAuthMiddleware(cfg))

	// Admin API endpoints
	router.Get("/api/routing/policy", adminHandlers.GetRoutingPolicy)
	router.Handle("PUT /api/routing/policy", adminHandlers.RequireRole(string(httpMiddleware.RoleAdmin))(http.HandlerFunc(adminHandlers.UpdateRoutingPolicy)))
	router.Get("/api/routing/heads", adminHandlers.GetAllHeads)
	router.Handle("POST /api/routing/heads", adminHandlers.RequireRole(string(httpMiddleware.RoleOperator))(http.HandlerFunc(adminHandlers.RegisterHead)))

	// Webhook endpoints
	router.With(httpMiddleware.WebhookSecurityMiddleware(cfg), httpMiddleware.RateLimitMiddleware(rateLimiter)).Post("/webhook/head-status", webhookHandlers.HeadStatusWebhook)
	router.With(httpMiddleware.WebhookSecurityMiddleware(cfg), httpMiddleware.RateLimitMiddleware(rateLimiter)).Post("/webhook/routing-decision", webhookHandlers.RoutingDecisionWebhook)

	// SSE endpoints
	router.Get("/events/head-status", sseHandlers.HeadStatusEvents)
	router.Get("/events/routing-decisions", sseHandlers.RoutingDecisionEvents)

	// WebSocket endpoints
	router.Get("/ws/head-management", websocketHandlers.HeadManagementWebSocket)
	router.Get("/ws/routing-decisions", websocketHandlers.RoutingDecisionsWebSocket)

	// Health check and metrics
	router.Get("/health", adminHandlers.HealthCheck)
	router.Handle("/metrics", metrics.MetricsHandler())

	httpServer := &http.Server{
		Addr:    cfg.HTTPPort,
		Handler: router,
	}

	logger.Info("Starting HTTP server", zap.String("port", cfg.HTTPPort))
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatal("HTTP server failed", zap.Error(err))
	}
}

// startBackgroundServices starts background services with graceful shutdown
func startBackgroundServices(sseHandlers *handlers.SSEHandlers, metrics *monitoring.Metrics, registry routing.HeadRegistry) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		// Use defer to recover from any panics and log them
		defer func() {
			if r := recover(); r != nil {
				// Log the panic but don't crash the background service
				log.Printf("Panic in background service: %v", r)
			}
		}()

		// Update active heads count
		activeHeads := registry.GetActive()
		metrics.SetActiveHeads(float64(len(activeHeads)))

		// Send heartbeat to SSE clients
		sseHandlers.SendHeartbeat()
	}
}

// InMemoryRegistry implements HeadRegistry interface using in-memory storage
type InMemoryRegistry struct {
	heads map[string]routing.HeadService
	mutex sync.RWMutex
}

func NewInMemoryRegistry() *InMemoryRegistry {
	return &InMemoryRegistry{
		heads: make(map[string]routing.HeadService),
	}
}

func (r *InMemoryRegistry) Register(head routing.HeadService) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.heads[head.HeadID] = head
	return nil
}

func (r *InMemoryRegistry) UpdateStatus(headID, status string, load int32, timestamp int64) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if head, exists := r.heads[headID]; exists {
		head.Status = status
		head.CurrentLoad = load
		head.LastHeartbeat = timestamp
		r.heads[headID] = head
	}
	return nil
}

func (r *InMemoryRegistry) GetAll() []routing.HeadService {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	heads := make([]routing.HeadService, 0, len(r.heads))
	for _, head := range r.heads {
		heads = append(heads, head)
	}
	return heads
}

func (r *InMemoryRegistry) GetByModelType(modelType string) []routing.HeadService {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	var heads []routing.HeadService
	for _, head := range r.heads {
		if head.ModelType == modelType {
			heads = append(heads, head)
		}
	}
	return heads
}

func (r *InMemoryRegistry) GetByRegion(region string) []routing.HeadService {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	var heads []routing.HeadService
	for _, head := range r.heads {
		if head.Region == region {
			heads = append(heads, head)
		}
	}
	return heads
}

func (r *InMemoryRegistry) GetActive() []routing.HeadService {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	var heads []routing.HeadService
	for _, head := range r.heads {
		if head.Status == "active" {
			heads = append(heads, head)
		}
	}
	return heads
}

// InMemoryPolicyManager implements PolicyManager interface
type InMemoryPolicyManager struct {
	policy *routing.RoutingPolicy
	mutex  sync.RWMutex
}

func (pm *InMemoryPolicyManager) Get() *routing.RoutingPolicy {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()
	return pm.policy
}

func (pm *InMemoryPolicyManager) Update(policy *routing.RoutingPolicy) error {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()
	pm.policy = policy
	return nil
}
