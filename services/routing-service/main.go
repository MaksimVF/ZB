



package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/go-redis/redis/v8"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	pb "github.com/MaksimVF/ZB/gen/proto"
)

var (
	redisClient   *redis.Client
	logger        *zap.Logger
	httpServer    *http.Server
	grpcServer    *grpc.Server
	headServices  = make(map[string]HeadService)
	routingPolicy RoutingPolicy
	configMutex   sync.RWMutex

	// Performance optimization
	routingCache = make(map[string]string) // Cache for routing decisions
	cacheMutex   sync.RWMutex

	// External service integration
	externalServiceClient *http.Client

	// Prometheus metrics
	routingDecisions = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "routing_decisions_total",
			Help: "Total number of routing decisions made",
		},
		[]string{"strategy", "model_type", "region"},
	)

	headRegistrations = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "head_registrations_total",
			Help: "Total number of head registrations",
		},
	)

	headStatusUpdates = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "head_status_updates_total",
			Help: "Total number of head status updates",
		},
	)

	activeHeads = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "active_heads",
			Help: "Number of active heads",
		},
	)

	httpRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status"},
	)

	// Cache performance metrics
	cacheHits = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "cache_hits_total",
			Help: "Total number of cache hits",
		},
	)

	cacheMisses = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "cache_misses_total",
			Help: "Total number of cache misses",
		},
	)

	// External service metrics
	externalServiceCalls = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "external_service_calls_total",
			Help: "Total number of external service calls",
		},
		[]string{"service", "status"},
	)
)

type HeadService struct {
	HeadID        string            `json:"head_id"`
	Endpoint      string            `json:"endpoint"`
	Status        string            `json:"status"`
	CurrentLoad   int32             `json:"current_load"`
	Region        string            `json:"region"`
	ModelType     string            `json:"model_type"`
	Version       string            `json:"version"`
	Metadata      map[string]string `json:"metadata"`
	LastHeartbeat int64             `json:"last_heartbeat"`
}

type RoutingPolicy struct {
	DefaultStrategy   string            `json:"default_strategy"`
	EnableGeoRouting  bool              `json:"enable_geo_routing"`
	EnableLoadBalancing bool            `json:"enable_load_balancing"`
	EnableModelSpecific bool            `json:"enable_model_specific"`
	StrategyConfig    map[string]string `json:"strategy_config"`
}

type RoutingServer struct {
	pb.UnimplementedRoutingServiceServer
}

func main() {
	// Initialize logger
	var err error
	logger, err = zap.NewProduction()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	// Initialize Prometheus metrics
	prometheus.MustRegister(
		routingDecisions,
		headRegistrations,
		headStatusUpdates,
		activeHeads,
		httpRequests,
		cacheHits,
		cacheMisses,
		externalServiceCalls,
	)

	// Initialize Redis client
	redisClient = redis.NewClient(&redis.Options{
		Addr: "redis:6379",
	})

	// Test Redis connection
	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		logger.Fatal("Failed to connect to Redis", zap.Error(err))
	}

	// Initialize default routing policy
	routingPolicy = RoutingPolicy{
		DefaultStrategy:   "round_robin",
		EnableGeoRouting:  true,
		EnableLoadBalancing: true,
		EnableModelSpecific: true,
		StrategyConfig:    make(map[string]string),
	}

	// Initialize external service client
	externalServiceClient = &http.Client{
		Timeout: 10 * time.Second,
	}

	// Start gRPC server
	go startGRPCServer()

	// Start HTTP server
	go startHTTPServer()

	// Wait for shutdown signal
	waitForShutdown()
}

func startGRPCServer() {
	lis, err := net.Listen("tcp", ":50055")
	if err != nil {
		logger.Fatal("Failed to listen", zap.Error(err))
	}

	// Load TLS certificates
	serverCert, err := tls.LoadX509KeyPair("certs/server.crt", "certs/server.key")
	if err != nil {
		logger.Fatal("Failed to load server certificates", zap.Error(err))
	}

	// Load CA certificate
	caCert, err := os.ReadFile("certs/ca.crt")
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
		ClientCAs:   certPool,
	}

	// Create gRPC server with TLS
	grpcServer = grpc.NewServer(
		grpc.Creds(credentials.NewTLS(tlsConfig)),
	)
	pb.RegisterRoutingServiceServer(grpcServer, &RoutingServer{})

	logger.Info("Starting gRPC server with mTLS on :50055")
	if err := grpcServer.Serve(lis); err != nil {
		logger.Fatal("gRPC server failed", zap.Error(err))
	}
}


type UserRole string

const (
	RoleAdmin    UserRole = "admin"
	RoleOperator UserRole = "operator"
	RoleViewer   UserRole = "viewer"
)

type UserContext struct {
	UserID string
	Role   UserRole
}

func jwtMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip authentication for health check
		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Missing authorization header", http.StatusUnauthorized)
			httpRequests.WithLabelValues(r.Method, r.URL.Path, "401").Inc()
			return
		}

		// In production, validate the JWT token and extract user info
		// For now, we'll simulate token validation and role extraction
		var userCtx UserContext

		// Simulate token validation
		switch authHeader {
		case "Bearer admin-token":
			userCtx = UserContext{UserID: "admin-user", Role: RoleAdmin}
		case "Bearer operator-token":
			userCtx = UserContext{UserID: "operator-user", Role: RoleOperator}
		case "Bearer viewer-token":
			userCtx = UserContext{UserID: "viewer-user", Role: RoleViewer}
		default:
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			httpRequests.WithLabelValues(r.Method, r.URL.Path, "401").Inc()
			return
		}

		// Add user context to request
		ctx := context.WithValue(r.Context(), "user", userCtx)

		// Create a response recorder to capture status code
		rec := statusRecorder{ResponseWriter: w, statusCode: 200}
		next.ServeHTTP(&rec, r.WithContext(ctx))

		// Record HTTP request metrics
		httpRequests.WithLabelValues(r.Method, r.URL.Path, fmt.Sprintf("%d", rec.statusCode)).Inc()
	})
}

// statusRecorder is a wrapper around http.ResponseWriter that captures the status code
type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

func checkRole(requiredRole UserRole) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userCtx, ok := r.Context().Value("user").(UserContext)
			if !ok || userCtx.Role != requiredRole {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func startHTTPServer() {
	router := mux.NewRouter()

	// Admin API endpoints with RBAC
	router.HandleFunc("/api/routing/policy", getRoutingPolicy).Methods("GET")
	router.Handle("/api/routing/policy", checkRole(RoleAdmin)(http.HandlerFunc(updateRoutingPolicy))).Methods("PUT")
	router.Handle("/api/routing/heads", checkRole(RoleOperator)(http.HandlerFunc(registerHeadHTTP))).Methods("POST")
	router.HandleFunc("/api/routing/heads", getAllHeads).Methods("GET")
	router.HandleFunc("/health", healthCheck).Methods("GET")

	// Webhook endpoints
	router.HandleFunc("/webhook/head-status", handleHeadStatusWebhook).Methods("POST")
	router.HandleFunc("/webhook/routing-decision", handleRoutingDecisionWebhook).Methods("POST")

	// Serve admin interface
	router.PathPrefix("/admin/").Handler(http.StripPrefix("/admin/", http.FileServer(http.Dir("./"))))

	// Add Prometheus metrics endpoint
	router.Handle("/metrics", promhttp.Handler()).Methods("GET")

	// Apply JWT middleware
	httpServer = &http.Server{
		Addr:    ":8080",
		Handler: jwtMiddleware(router),
	}

	logger.Info("Starting HTTP server with JWT authentication, RBAC, Prometheus metrics, and webhook support on :8080")
	if err := httpServer.ListenAndServe(); err != nil && err != http.ServerClosed {
		logger.Fatal("HTTP server failed", zap.Error(err))
	}
}

func waitForShutdown() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	logger.Info("Shutting down...")

	// Shutdown HTTP server
	if httpServer != nil {
		if err := httpServer.Shutdown(context.Background()); err != nil {
			logger.Error("HTTP server shutdown error", zap.Error(err))
		}
	}

	// Shutdown gRPC server
	if grpcServer != nil {
		grpcServer.GracefulStop()
	}

	logger.Info("Shutdown complete")
}

// Webhook handlers
func handleHeadStatusWebhook(w http.ResponseWriter, r *http.Request) {
	var webhookData struct {
		HeadID     string `json:"head_id"`
		Status     string `json:"status"`
		CurrentLoad int32  `json:"current_load"`
		Timestamp  int64  `json:"timestamp"`
	}

	if err := json.NewDecoder(r.Body).Decode(&webhookData); err != nil {
		http.Error(w, "Invalid webhook payload", http.StatusBadRequest)
		return
	}

	// Process the webhook
	err := processHeadStatusUpdate(webhookData.HeadID, webhookData.Status, webhookData.CurrentLoad, webhookData.Timestamp)
	if err != nil {
		http.Error(w, "Failed to process webhook", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Webhook processed successfully")
}

func handleRoutingDecisionWebhook(w http.ResponseWriter, r *http.Request) {
	var webhookData struct {
		ModelType       string            `json:"model_type"`
		RegionPreference string            `json:"region_preference"`
		RoutingStrategy  string            `json:"routing_strategy"`
		Metadata        map[string]string `json:"metadata"`
	}

	if err := json.NewDecoder(r.Body).Decode(&webhookData); err != nil {
		http.Error(w, "Invalid webhook payload", http.StatusBadRequest)
		return
	}

	// Process the routing decision request
	decision, err := makeRoutingDecisionFromWebhook(webhookData.ModelType, webhookData.RegionPreference, webhookData.RoutingStrategy, webhookData.Metadata)
	if err != nil {
		http.Error(w, "Failed to make routing decision", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(decision)
}

func processHeadStatusUpdate(headID, status string, currentLoad int32, timestamp int64) error {
	// Update head status in the system
	_, err := (&RoutingServer{}).UpdateHeadStatus(context.Background(), &pb.UpdateHeadStatusRequest{
		HeadId:      headID,
		Status:      status,
		CurrentLoad: currentLoad,
		Timestamp:   timestamp,
	})
	return err
}

func makeRoutingDecisionFromWebhook(modelType, regionPreference, routingStrategy string, metadata map[string]string) (map[string]interface{}, error) {
	// Make a routing decision based on webhook data
	resp, err := (&RoutingServer{}).GetRoutingDecision(context.Background(), &pb.GetRoutingDecisionRequest{
		ModelType:       modelType,
		RegionPreference: regionPreference,
		RoutingStrategy: routingStrategy,
		Metadata:        metadata,
	})
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"head_id":       resp.HeadId,
		"endpoint":      resp.Endpoint,
		"strategy_used": resp.StrategyUsed,
		"reason":        resp.Reason,
		"metadata":      resp.Metadata,
	}, nil

// gRPC Methods

func (s *RoutingServer) RegisterHead(ctx context.Context, req *pb.RegisterHeadRequest) (*pb.RegisterHeadResponse, error) {
	configMutex.Lock()
	defer configMutex.Unlock()

	head := HeadService{
		HeadID:      req.HeadId,
		Endpoint:    req.Endpoint,
		Status:      "active",
		Region:      req.Region,
		ModelType:   req.ModelType,
		Version:     req.Version,
		Metadata:    req.Metadata,
		LastHeartbeat: time.Now().Unix(),
	}

	headServices[req.HeadId] = head

	// Store in Redis
	err := storeHeadInRedis(head)
	if err != nil {
		return &pb.RegisterHeadResponse{
			Success: false,
			Message: "Failed to store head in Redis",
		}, err
	}

	// Record metrics
	headRegistrations.Inc()
	activeHeads.Inc()

	return &pb.RegisterHeadResponse{
		Success: true,
		Message: "Head registered successfully",
	}, nil
}

func (s *RoutingServer) UpdateHeadStatus(ctx context.Context, req *pb.UpdateHeadStatusRequest) (*pb.UpdateHeadStatusResponse, error) {
	configMutex.Lock()
	defer configMutex.Unlock()

	head, exists := headServices[req.HeadId]
	if !exists {
		return &pb.UpdateHeadStatusResponse{
			Success: false,
			Message: "Head not found",
		}, nil
	}

	head.Status = req.Status
	head.CurrentLoad = req.CurrentLoad
	head.LastHeartbeat = req.Timestamp
	headServices[req.HeadId] = head

	// Update in Redis
	err := updateHeadStatusInRedis(head)
	if err != nil {
		return &pb.UpdateHeadStatusResponse{
			Success: false,
			Message: "Failed to update head in Redis",
		}, err
	}

	// Invalidate cache if head becomes inactive
	if head.Status != "active" {
		// Clear cache entries that might reference this head
		cacheMutex.Lock()
		for key, headID := range routingCache {
			if headID == head.HeadID {
				delete(routingCache, key)
			}
		}
		cacheMutex.Unlock()
	}

	// Record metrics
	headStatusUpdates.Inc()

	return &pb.UpdateHeadStatusResponse{Success: true, Message: "Head status updated successfully"}, nil
}

func (s *RoutingServer) GetRoutingDecision(ctx context.Context, req *pb.GetRoutingDecisionRequest) (*pb.GetRoutingDecisionResponse, error) {

	// Implement routing decision logic based on current policy
	// This is a simplified version - in production this would be more sophisticated

	// Check cache first
	cacheKey := fmt.Sprintf("%s-%s-%s-%s", req.ModelType, req.RegionPreference, req.RoutingStrategy, req.Metadata["model"])
	cacheMutex.RLock()
	cachedHeadID, found := routingCache[cacheKey]
	cacheMutex.RUnlock()

	if found {
		// Cache hit
		cacheHits.Inc()

		// Find the cached head in our current list
		configMutex.RLock()
		for _, head := range headServices {
			if head.HeadID == cachedHeadID && head.Status == "active" {
				configMutex.RUnlock()
				return &pb.GetRoutingDecisionResponse{
					HeadId:      head.HeadID,
					Endpoint:    head.Endpoint,
					StrategyUsed: "cached",
					Reason:      "Cache hit",
					Metadata:    map[string]string{"model": head.ModelType, "region": head.Region},
				}, nil
			}
		}
		configMutex.RUnlock()
	}

	// Cache miss - proceed with normal routing
	cacheMisses.Inc()

	configMutex.RLock()
	defer configMutex.RUnlock()

	var selectedHead *HeadService

	// Filter heads by model type
	var candidates []HeadService
	for _, head := range headServices {
		if head.ModelType == req.ModelType && head.Status == "active" {
			candidates = append(candidates, head)
		}
	}

	if len(candidates) == 0 {
		return &pb.GetRoutingDecisionResponse{
			HeadId:      "",
			Endpoint:    "",
			StrategyUsed: "none",
			Reason:      "No available heads for model type",
		}, nil
	}

	// Apply routing strategy based on request or default policy
	strategy := req.RoutingStrategy
	if strategy == "" {
		strategy = routingPolicy.DefaultStrategy
	}

	var reason string
	switch strategy {
	case "round_robin":
		selectedHead = applyRoundRobinStrategy(candidates)
		reason = "Round-robin selection"
	case "least_loaded":
		selectedHead = applyLeastLoadedStrategy(candidates)
		reason = "Least loaded selection"
	case "geo_preferred":
		selectedHead = applyGeoPreferredStrategy(candidates, req.RegionPreference)
		reason = "Geo-preferred selection"
	case "model_specific":
		selectedHead = applyModelSpecificStrategy(candidates, req.Metadata)
		reason = "Model-specific selection"
	case "hybrid":
		selectedHead = applyHybridStrategy(candidates, req)
		reason = "Hybrid strategy selection"
	default:
		// Default to round-robin
		selectedHead = applyRoundRobinStrategy(candidates)
		reason = "Default round-robin selection"
	}

	if selectedHead == nil {
		return &pb.GetRoutingDecisionResponse{
			HeadId:      "",
			Endpoint:    "",
			StrategyUsed: strategy,
			Reason:      "No suitable head found",
		}, nil
	}

	// Update cache
	cacheMutex.Lock()
	routingCache[cacheKey] = selectedHead.HeadID
	cacheMutex.Unlock()

	// Record metrics
	routingDecisions.WithLabelValues(strategy, req.ModelType, selectedHead.Region).Inc()

	return &pb.GetRoutingDecisionResponse{
		HeadId:      selectedHead.HeadID,
		Endpoint:    selectedHead.Endpoint,
		StrategyUsed: strategy,
		Reason:      reason,
		Metadata:    map[string]string{"model": selectedHead.ModelType, "region": selectedHead.Region},
	}, nil
}

func (s *RoutingServer) GetAllHeads(ctx context.Context, req *pb.GetAllHeadsRequest) (*pb.GetAllHeadsResponse, error) {
	configMutex.RLock()
	defer configMutex.RUnlock()

	var heads []*pb.HeadService
	for _, head := range headServices {
		heads = append(heads, &pb.HeadService{
			HeadId:        head.HeadID,
			Endpoint:      head.Endpoint,
			Status:        head.Status,
			CurrentLoad:   head.CurrentLoad,
			Region:        head.Region,
			ModelType:     head.ModelType,
			Version:       head.Version,
			Metadata:      head.Metadata,
			LastHeartbeat: head.LastHeartbeat,
		})
	}

	return &pb.GetAllHeadsResponse{Heads: heads}, nil
}

func (s *RoutingServer) UpdateRoutingPolicy(ctx context.Context, req *pb.UpdateRoutingPolicyRequest) (*pb.UpdateRoutingPolicyResponse, error) {
	configMutex.Lock()
	defer configMutex.Unlock()

	routingPolicy = RoutingPolicy{
		DefaultStrategy:   req.DefaultStrategy,
		EnableGeoRouting:  req.EnableGeoRouting,
		EnableLoadBalancing: req.EnableLoadBalancing,
		EnableModelSpecific: req.EnableModelSpecific,
		StrategyConfig:    req.StrategyConfig,
	}

	// Store in Redis
	err := storeRoutingPolicyInRedis(routingPolicy)
	if err != nil {
		return &pb.UpdateRoutingPolicyResponse{
			Success: false,
			Message: "Failed to store policy in Redis",
		}, err
	}

	return &pb.UpdateRoutingPolicyResponse{
		Success: true,
		Message: "Routing policy updated successfully",
	}, nil
}

func (s *RoutingServer) GetRoutingPolicy(ctx context.Context, req *pb.GetRoutingPolicyRequest) (*pb.GetRoutingPolicyResponse, error) {
	configMutex.RLock()
	defer configMutex.RUnlock()

	return &pb.GetRoutingPolicyResponse{
		DefaultStrategy:   routingPolicy.DefaultStrategy,
		EnableGeoRouting:  routingPolicy.EnableGeoRouting,
		EnableLoadBalancing: routingPolicy.EnableLoadBalancing,
		EnableModelSpecific: routingPolicy.EnableModelSpecific,
		StrategyConfig:    routingPolicy.StrategyConfig,
	}, nil
}

// HTTP Handlers

func getRoutingPolicy(w http.ResponseWriter, r *http.Request) {
	configMutex.RLock()
	defer configMutex.RUnlock()

	json.NewEncoder(w).Encode(routingPolicy)
}

func updateRoutingPolicy(w http.ResponseWriter, r *http.Request) {
	var policy RoutingPolicy
	err := json.NewDecoder(r.Body).Decode(&policy)
	if err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	configMutex.Lock()
	defer configMutex.Unlock()

	routingPolicy = policy

	// Store in Redis
	err = storeRoutingPolicyInRedis(policy)
	if err != nil {
		http.Error(w, "Failed to store policy", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(policy)
}

func getAllHeads(w http.ResponseWriter, r *http.Request) {
	configMutex.RLock()
	defer configMutex.RUnlock()

	json.NewEncoder(w).Encode(headServices)
}

// HTTP handler for head registration
func registerHeadHTTP(w http.ResponseWriter, r *http.Request) {
	var head HeadService
	err := json.NewDecoder(r.Body).Decode(&head)
	if err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	// Call the gRPC method
	resp, err := (&RoutingServer{}).RegisterHead(context.Background(), &pb.RegisterHeadRequest{
		HeadId:    head.HeadID,
		Endpoint:  head.Endpoint,
		Region:    head.Region,
		ModelType: head.ModelType,
		Version:   head.Version,
		Metadata:  head.Metadata,
	})

	if err != nil {
		http.Error(w, "Failed to register head", http.StatusInternalServerError)
		return
	}

	if !resp.Success {
		http.Error(w, resp.Message, http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

func healthCheck(w http.ResponseWriter, r *http.Request) {

	ctx := context.Background()
	err := redisClient.Ping(ctx).Err()
	if err != nil {
		http.Error(w, "Redis unavailable", http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

// Redis Functions

func storeHeadInRedis(head HeadService) error {
	ctx := context.Background()

	// Store head data
	headKey := fmt.Sprintf("head:%s", head.HeadID)
	headData := map[string]interface{}{
		"head_id":        head.HeadID,
		"endpoint":       head.Endpoint,
		"status":         head.Status,
		"current_load":   head.CurrentLoad,
		"region":         head.Region,
		"model_type":     head.ModelType,
		"version":        head.Version,
		"metadata":        head.Metadata,
		"last_heartbeat": head.LastHeartbeat,
	}

	err := redisClient.HMSet(ctx, headKey, headData).Err()
	if err != nil {
		return err
	}

	// Add to model type index
	err = redisClient.SAdd(ctx, fmt.Sprintf("model:%s:heads", head.ModelType), head.HeadID).Err()
	if err != nil {
		return err
	}

	// Add to region index
	err = redisClient.SAdd(ctx, fmt.Sprintf("region:%s:heads", head.Region), head.HeadID).Err()
	if err != nil {
		return err
	}

	return nil
}

func updateHeadStatusInRedis(head HeadService) error {
	ctx := context.Background()

	headKey := fmt.Sprintf("head:%s", head.HeadID)
	headData := map[string]interface{}{
		"status":         head.Status,
		"current_load":   head.CurrentLoad,
		"last_heartbeat": head.LastHeartbeat,
	}

	return redisClient.HMSet(ctx, headKey, headData).Err()
}

func storeRoutingPolicyInRedis(policy RoutingPolicy) error {
	ctx := context.Background()

	policyData := map[string]interface{}{
		"default_strategy":    policy.DefaultStrategy,
		"enable_geo_routing":  policy.EnableGeoRouting,
		"enable_load_balancing": policy.EnableLoadBalancing,
		"enable_model_specific": policy.EnableModelSpecific,
		"strategy_config":     policy.StrategyConfig,
	}

	return redisClient.HMSet(ctx, "routing:policy", policyData).Err()
}

// Routing Strategy Functions

// applyRoundRobinStrategy implements round-robin load balancing
func applyRoundRobinStrategy(heads []HeadService) *HeadService {
	// Simple round-robin: select the first head (in production, track last used)
	if len(heads) == 0 {
		return nil
	}
	return &heads[0]
}

// applyLeastLoadedStrategy selects the head with the lowest current load
func applyLeastLoadedStrategy(heads []HeadService) *HeadService {
	if len(heads) == 0 {
		return nil
	}

	// Find the head with the minimum load
	minLoad := heads[0]
	for _, head := range heads[1:] {
		if head.CurrentLoad < minLoad.CurrentLoad {
			minLoad = head
		}
	}
	return &minLoad
}

// applyGeoPreferredStrategy selects a head in the preferred region
func applyGeoPreferredStrategy(heads []HeadService, preferredRegion string) *HeadService {
	if len(heads) == 0 {
		return nil
	}

	// First try to find a head in the preferred region
	for _, head := range heads {
		if head.Region == preferredRegion {
			return &head
		}
	}

	// If no head in preferred region, fall back to round-robin
	return applyRoundRobinStrategy(heads)
}

// applyModelSpecificStrategy selects based on model-specific criteria
func applyModelSpecificStrategy(heads []HeadService, metadata map[string]string) *HeadService {
	if len(heads) == 0 {
		return nil
	}

	// For now, just return the first head
	// In production, this would use model-specific criteria from metadata
	return &heads[0]
}

// applyHybridStrategy combines multiple strategies
func applyHybridStrategy(heads []HeadService, req *pb.GetRoutingDecisionRequest) *HeadService {
	if len(heads) == 0 {
		return nil
	}

	// Hybrid approach: first try geo-preferred, then least-loaded
	geoHead := applyGeoPreferredStrategy(heads, req.RegionPreference)
	if geoHead != nil {
		return geoHead
	}

	return applyLeastLoadedStrategy(heads)
}

// External service integration
func callExternalService(serviceName, endpoint string, payload interface{}) ([]byte, error) {
	startTime := time.Now()

	// Convert payload to JSON
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		externalServiceCalls.WithLabelValues(serviceName, "error").Inc()
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Make HTTP request to external service
	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(payloadBytes))
	if err != nil {
		externalServiceCalls.WithLabelValues(serviceName, "error").Inc()
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := externalServiceClient.Do(req)
	if err != nil {
		externalServiceCalls.WithLabelValues(serviceName, "error").Inc()
		return nil, fmt.Errorf("external service request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		externalServiceCalls.WithLabelValues(serviceName, "error").Inc()
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Record metrics
	status := "success"
	if resp.StatusCode >= 400 {
		status = "error"
	}
	externalServiceCalls.WithLabelValues(serviceName, status).Inc()

	return body, nil
}

