

package main

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	pb "github.com/MaksimVF/ZB/gen/proto"
)

var (
	redisClient   *redis.Client
	logger         *zap.Logger
	grpcServer     *grpc.Server
	httpServer     *http.Server
	configMutex    sync.RWMutex
	routingPolicy  RoutingPolicy
	headServices   = make(map[string]HeadService)
)

type RoutingPolicy struct {
	DefaultStrategy      string            `json:"default_strategy"`
	EnableGeoRouting     bool              `json:"enable_geo_routing"`
	EnableLoadBalancing  bool              `json:"enable_load_balancing"`
	EnableModelSpecific  bool              `json:"enable_model_specific"`
	StrategyConfig       map[string]string `json:"strategy_config"`
}

type HeadService struct {
	HeadID        string            `json:"head_id"`
	Endpoint      string            `json:"endpoint"`
	Status        string            `json:"status"`
	CurrentLoad   int32             `json:"current_load"`
	LastHeartbeat int64             `json:"last_heartbeat"`
	Region        string            `json:"region"`
	ModelType     string            `json:"model_type"`
	Version       string            `json:"version"`
	Metadata      map[string]string `json:"metadata"`
}

type RoutingServer struct {
	pb.UnimplementedRoutingServiceServer
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

	// Load initial routing policy
	loadRoutingPolicy()

	// Load initial head services
	loadHeadServices()
}

func main() {
	// Start gRPC server
	go startGRPCServer()

	// Start HTTP server for admin interface
	go startHTTPServer()

	// Wait for shutdown signal
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	<-c

	logger.Info("Shutting down Routing Service...")
	shutdown()
}

func startGRPCServer() {
	lis, err := net.Listen("tcp", ":50061")
	if err != nil {
		logger.Fatal("Failed to listen", zap.Error(err))
	}

	grpcServer = grpc.NewServer()
	pb.RegisterRoutingServiceServer(grpcServer, &RoutingServer{})

	logger.Info("Starting gRPC server on :50061")
	if err := grpcServer.Serve(lis); err != nil {
		logger.Fatal("gRPC server failed", zap.Error(err))
	}
}

func startHTTPServer() {
	router := mux.NewRouter()

	// Admin API endpoints
	router.HandleFunc("/api/routing/policy", getRoutingPolicy).Methods("GET")
	router.HandleFunc("/api/routing/policy", updateRoutingPolicy).Methods("PUT")
	router.HandleFunc("/api/routing/heads", getAllHeads).Methods("GET")
	router.HandleFunc("/api/routing/heads", registerHeadHTTP).Methods("POST")
	router.HandleFunc("/health", healthCheck).Methods("GET")

	// Serve admin interface
	router.PathPrefix("/admin/").Handler(http.StripPrefix("/admin/", http.FileServer(http.Dir("./"))))

	httpServer = &http.Server{
		Addr:    ":8080",
		Handler: router,
	}

	logger.Info("Starting HTTP server on :8080")
	if err := httpServer.ListenAndServe(); err != nil && err != http.ServerClosed {
		logger.Fatal("HTTP server failed", zap.Error(err))
	}
}

func shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Shutdown gRPC server
	grpcServer.GracefulStop()

	// Shutdown HTTP server
	_ = httpServer.Shutdown(ctx)

	logger.Info("Routing Service stopped")
}

// Load routing policy from Redis
func loadRoutingPolicy() {
	configMutex.Lock()
	defer configMutex.Unlock()

	ctx := context.Background()
	result, err := redisClient.Get(ctx, "routing:policy").Result()
	if err == redis.Nil {
		// Default policy if not found
		routingPolicy = RoutingPolicy{
			DefaultStrategy:     "round_robin",
			EnableGeoRouting:    true,
			EnableLoadBalancing: true,
			EnableModelSpecific: true,
			StrategyConfig:     make(map[string]string),
		}
		return
	} else if err != nil {
		logger.Error("Failed to load routing policy", zap.Error(err))
		return
	}

	err = json.Unmarshal([]byte(result), &routingPolicy)
	if err != nil {
		logger.Error("Failed to parse routing policy", zap.Error(err))
	}
}

// Save routing policy to Redis
func saveRoutingPolicy(policy RoutingPolicy) error {
	configMutex.Lock()
	defer configMutex.Unlock()

	data, err := json.Marshal(policy)
	if err != nil {
		return err
	}

	ctx := context.Background()
	return redisClient.Set(ctx, "routing:policy", data, 0).Err()
}

// Load head services from Redis
func loadHeadServices() {
	configMutex.Lock()
	defer configMutex.Unlock()

	ctx := context.Background()
	pattern := "routing:head:*"
	keys, err := redisClient.Keys(ctx, pattern).Result()
	if err != nil {
		logger.Error("Failed to load head services", zap.Error(err))
		return
	}

	for _, key := range keys {
		if !contains(key, ":status") && !contains(key, ":heartbeat") {
			headID := extractHeadID(key)
			headData, err := redisClient.Get(ctx, key).Result()
			if err == nil {
				var headService HeadService
				err = json.Unmarshal([]byte(headData), &headService)
				if err == nil {
					headServices[headID] = headService
				}
			}
		}
	}
}

// Helper functions
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[len(s)-len(substr):] == substr
}

func extractHeadID(key string) string {
	// Extract head_id from "routing:head:{head_id}"
	// Simple implementation - in production use proper parsing
	return key[len("routing:head:"):]
}

// gRPC method implementations
func (s *RoutingServer) RegisterHead(ctx context.Context, req *pb.RegisterHeadRequest) (*pb.RegisterHeadResponse, error) {
	headID := req.HeadId
	headService := HeadService{
		HeadID:    headID,
		Endpoint:  req.Endpoint,
		Status:    "active",
		Region:    req.Region,
		ModelType: req.ModelType,
		Version:   req.Version,
		Metadata:  req.Metadata,
	}

	// Store in Redis
	ctxRedis := context.Background()
	headData, err := json.Marshal(headService)
	if err != nil {
		return &pb.RegisterHeadResponse{Success: false, Message: "Failed to serialize head data"}, nil
	}

	// Use Lua script to register head
	luaScript := `
	local head_data = ARGV[1]
	local model_type = ARGV[2]
	local region = ARGV[3]

	-- Store head information
	redis.call('SET', 'routing:head:' .. KEYS[1], head_data)

	-- Add to model index
	redis.call('SADD', 'routing:model:' .. model_type, KEYS[1])

	-- Add to region index
	redis.call('SADD', 'routing:region:' .. region, KEYS[1])

	return 1
	`

	_, err = redisClient.Eval(ctxRedis, luaScript, []string{headID}, headData, req.ModelType, req.Region).Result()
	if err != nil {
		return &pb.RegisterHeadResponse{Success: false, Message: "Failed to register head"}, nil
	}

	// Update local cache
	configMutex.Lock()
	headServices[headID] = headService
	configMutex.Unlock()

	return &pb.RegisterHeadResponse{Success: true, Message: "Head registered successfully"}, nil
}

func (s *RoutingServer) UpdateHeadStatus(ctx context.Context, req *pb.UpdateHeadStatusRequest) (*pb.UpdateHeadStatusResponse, error) {
	headID := req.HeadId

	// Update local cache
	configMutex.Lock()
	if headService, exists := headServices[headID]; exists {
		headService.Status = req.Status
		headService.CurrentLoad = req.CurrentLoad
		headService.LastHeartbeat = req.Timestamp
		headServices[headID] = headService
	}
	configMutex.Unlock()

	// Update in Redis
	ctxRedis := context.Background()
	statusData, err := json.Marshal(map[string]interface{}{
		"status":       req.Status,
		"current_load": req.CurrentLoad,
		"timestamp":    req.Timestamp,
	})
	if err != nil {
		return &pb.UpdateHeadStatusResponse{Success: false, Message: "Failed to serialize status data"}, nil
	}

	// Use Lua script to update status
	luaScript := `
	local status_data = ARGV[1]
	local timestamp = ARGV[2]

	-- Store status information
	redis.call('SET', 'routing:head:' .. KEYS[1] .. ':status', status_data)

	-- Update heartbeat
	redis.call('SET', 'routing:head:' .. KEYS[1] .. ':heartbeat', timestamp)

	return 1
	`

	_, err = redisClient.Eval(ctxRedis, luaScript, []string{headID}, statusData, fmt.Sprintf("%d", req.Timestamp)).Result()
	if err != nil {
		return &pb.UpdateHeadStatusResponse{Success: false, Message: "Failed to update head status"}, nil
	}

	return &pb.UpdateHeadStatusResponse{Success: true, Message: "Head status updated successfully"}, nil
}

func (s *RoutingServer) GetRoutingDecision(ctx context.Context, req *pb.GetRoutingDecisionRequest) (*pb.GetRoutingDecisionResponse, error) {
	// Implement routing decision logic based on current policy
	// This is a simplified version - in production this would be more sophisticated

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

	// Simple round-robin for now
	selectedHead = &candidates[0] // In production, implement proper routing logic

	return &pb.GetRoutingDecisionResponse{
		HeadId:      selectedHead.HeadID,
		Endpoint:    selectedHead.Endpoint,
		StrategyUsed: routingPolicy.DefaultStrategy,
		Reason:      "Basic round-robin selection",
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
			LastHeartbeat: head.LastHeartbeat,
			Region:        head.Region,
			ModelType:     head.ModelType,
			Version:       head.Version,
			Metadata:      head.Metadata,
		})
	}

	return &pb.GetAllHeadsResponse{Heads: heads}, nil
}

func (s *RoutingServer) UpdateRoutingPolicy(ctx context.Context, req *pb.UpdateRoutingPolicyRequest) (*pb.UpdateRoutingPolicyResponse, error) {
	newPolicy := RoutingPolicy{
		DefaultStrategy:     req.Policy.DefaultStrategy,
		EnableGeoRouting:    req.Policy.EnableGeoRouting,
		EnableLoadBalancing: req.Policy.EnableLoadBalancing,
		EnableModelSpecific: req.Policy.EnableModelSpecific,
		StrategyConfig:     req.Policy.StrategyConfig,
	}

	err := saveRoutingPolicy(newPolicy)
	if err != nil {
		return &pb.UpdateRoutingPolicyResponse{Success: false, Message: "Failed to save routing policy"}, nil
	}

	// Update local cache
	configMutex.Lock()
	routingPolicy = newPolicy
	configMutex.Unlock()

	return &pb.UpdateRoutingPolicyResponse{Success: true, Message: "Routing policy updated successfully"}, nil
}

func (s *RoutingServer) GetRoutingPolicy(ctx context.Context, req *pb.GetRoutingPolicyRequest) (*pb.GetRoutingPolicyResponse, error) {
	configMutex.RLock()
	defer configMutex.RUnlock()

	policy := &pb.RoutingPolicy{
		DefaultStrategy:     routingPolicy.DefaultStrategy,
		EnableGeoRouting:    routingPolicy.EnableGeoRouting,
		EnableLoadBalancing: routingPolicy.EnableLoadBalancing,
		EnableModelSpecific: routingPolicy.EnableModelSpecific,
		StrategyConfig:     routingPolicy.StrategyConfig,
	}

	return &pb.GetRoutingPolicyResponse{Policy: policy}, nil
}

// HTTP handlers
func getRoutingPolicy(w http.ResponseWriter, r *http.Request) {
	configMutex.RLock()
	defer configMutex.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(routingPolicy)
}

func updateRoutingPolicy(w http.ResponseWriter, r *http.Request) {
	var newPolicy RoutingPolicy
	err := json.NewDecoder(r.Body).Decode(&newPolicy)
	if err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	err = saveRoutingPolicy(newPolicy)
	if err != nil {
		http.Error(w, "Failed to save routing policy", http.StatusInternalServerError)
		return
	}

	// Update local cache
	configMutex.Lock()
	routingPolicy = newPolicy
	configMutex.Unlock()

	w.WriteHeader(http.StatusOK)
}

func getAllHeads(w http.ResponseWriter, r *http.Request) {
	configMutex.RLock()
	defer configMutex.RUnlock()

	w.Header().Set("Content-Type", "application/json")
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
	w.Write([]byte("OK"))
}

