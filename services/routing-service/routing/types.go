package routing

import (
	"time"
)

// HeadService represents a head service instance
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
	
	// Optimization fields
	LoadHistory    []int32   `json:"load_history,omitempty"` // Historical load data for prediction
	ResponseTimes []int64   `json:"response_times,omitempty"` // Historical response times
	Capacity      int32     `json:"capacity,omitempty"` // Maximum capacity
	Utilization   float64   `json:"utilization,omitempty"` // Current utilization percentage
}

// RoutingPolicy represents the routing configuration
type RoutingPolicy struct {
	DefaultStrategy       string            `json:"default_strategy"`
	EnableGeoRouting      bool              `json:"enable_geo_routing"`
	EnableLoadBalancing   bool              `json:"enable_load_balancing"`
	EnableModelSpecific    bool              `json:"enable_model_specific"`
	EnablePredictive       bool              `json:"enable_predictive"`
	EnableAdaptive         bool              `json:"enable_adaptive"`
	StrategyConfig        map[string]string `json:"strategy_config"`
	PredictionWindow      int               `json:"prediction_window"` // Time window for predictions in minutes
	LoadGrowthFactor      float64           `json:"load_growth_factor"` // Growth factor for load prediction
	CapacityThreshold      float64           `json:"capacity_threshold"` // Utilization threshold for routing
}

// RoutingRequest represents a routing decision request
type RoutingRequest struct {
	ModelType        string            `json:"model_type"`
	RegionPreference string            `json:"region_preference"`
	RoutingStrategy  string            `json:"routing_strategy"`
	Metadata         map[string]string `json:"metadata"`
}

// RoutingResponse represents a routing decision response
type RoutingResponse struct {
	HeadID       string            `json:"head_id"`
	Endpoint     string            `json:"endpoint"`
	StrategyUsed string            `json:"strategy_used"`
	Reason       string            `json:"reason"`
	Metadata     map[string]string `json:"metadata"`
}

// Strategy interface for different routing strategies
type Strategy interface {
	Name() string
	SelectHead([]HeadService, *RoutingRequest) *HeadService
}

// HeadRegistry manages head service registrations
type HeadRegistry interface {
	Register(head HeadService) error
	UpdateStatus(headID, status string, load int32, timestamp int64) error
	GetAll() []HeadService
	GetByModelType(modelType string) []HeadService
	GetByRegion(region string) []HeadService
	GetActive() []HeadService
}

// PolicyManager manages routing policies
type PolicyManager interface {
	Get() *RoutingPolicy
	Update(policy *RoutingPolicy) error
}

// RoutingEngine is the core routing decision engine
type RoutingEngine struct {
	policy      *RoutingPolicy
	registry    HeadRegistry
	cache       Cache
	metrics     Metrics
	strategies  map[string]Strategy
}

// NewRoutingEngine creates a new routing engine
func NewRoutingEngine(policy *RoutingPolicy, registry HeadRegistry, cache Cache, metrics Metrics) *RoutingEngine {
	engine := &RoutingEngine{
		policy:     policy,
		registry:   registry,
		cache:      cache,
		metrics:    metrics,
		strategies: make(map[string]Strategy),
	}
	
	// Register strategies
	engine.registerStrategies()
	
	return engine
}

// GetDecision makes a routing decision
func (e *RoutingEngine) GetDecision(req *RoutingRequest) (*RoutingResponse, error) {
	// Check cache first
	cacheKey := e.generateCacheKey(req)
	if cached, found := e.cache.Get(cacheKey); found {
		e.metrics.IncCacheHit()
		return cached, nil
	}
	
	// Get available heads
	candidates := e.registry.GetByModelType(req.ModelType)
	activeHeads := filterActiveHeads(candidates)
	
	if len(activeHeads) == 0 {
		return &RoutingResponse{
			HeadID:       "",
			Endpoint:     "",
			StrategyUsed: "none",
			Reason:       "No available heads for model type",
		}, nil
	}
	
	// Select strategy
	strategy := e.selectStrategy(req)
	selectedHead := strategy.SelectHead(activeHeads, req)
	
	if selectedHead == nil {
		return &RoutingResponse{
			HeadID:       "",
			Endpoint:     "",
			StrategyUsed: strategy.Name(),
			Reason:       "No suitable head found",
		}, nil
	}
	
	// Create response
	response := &RoutingResponse{
		HeadID:       selectedHead.HeadID,
		Endpoint:     selectedHead.Endpoint,
		StrategyUsed: strategy.Name(),
		Reason:       e.getReasonForStrategy(strategy.Name()),
		Metadata: map[string]string{
			"model":  selectedHead.ModelType,
			"region": selectedHead.Region,
		},
	}
	
	// Cache the decision
	e.cache.Set(cacheKey, response)
	
	// Record metrics
	e.metrics.IncRoutingDecision(strategy.Name(), req.ModelType, selectedHead.Region)
	
	return response, nil
}

// selectStrategy selects the appropriate routing strategy
func (e *RoutingEngine) selectStrategy(req *RoutingRequest) Strategy {
	strategyName := req.RoutingStrategy
	if strategyName == "" {
		strategyName = e.policy.DefaultStrategy
	}
	
	if strategy, exists := e.strategies[strategyName]; exists {
		return strategy
	}
	
	// Fallback to adaptive routing
	return e.strategies["adaptive"]
}

// registerStrategies registers all available routing strategies
func (e *RoutingEngine) registerStrategies() {
	e.strategies["round_robin"] = NewRoundRobinStrategy()
	e.strategies["least_loaded"] = NewLeastLoadedStrategy()
	e.strategies["geo_preferred"] = NewGeoPreferredStrategy()
	e.strategies["model_specific"] = NewModelSpecificStrategy()
	e.strategies["predictive"] = NewPredictiveStrategy(e.cache, e.policy)
	e.strategies["adaptive"] = NewAdaptiveStrategy(e.registry, e.policy)
	e.strategies["hybrid"] = NewHybridStrategy(e.registry, e.cache, e.policy)
}

// generateCacheKey generates a cache key for the routing request
func (e *RoutingEngine) generateCacheKey(req *RoutingRequest) string {
	return generateCacheKey(req.ModelType, req.RegionPreference, req.RoutingStrategy, req.Metadata)
}

// getReasonForStrategy returns a human-readable reason for the strategy
func (e *RoutingEngine) getReasonForStrategy(strategyName string) string {
	reasons := map[string]string{
		"round_robin":    "Round-robin selection",
		"least_loaded":   "Least loaded selection",
		"geo_preferred":  "Geo-preferred selection",
		"model_specific": "Model-specific selection",
		"predictive":     "Predictive load balancing",
		"adaptive":       "Adaptive routing",
		"hybrid":         "Hybrid strategy selection",
	}
	
	return reasons[strategyName]
}

// filterActiveHeads filters only active heads
func filterActiveHeads(heads []HeadService) []HeadService {
	var active []HeadService
	for _, head := range heads {
		if head.Status == "active" {
			active = append(active, head)
		}
	}
	return active
}

// generateCacheKey generates a cache key for routing requests
func generateCacheKey(modelType, regionPreference, strategy string, metadata map[string]string) string {
	key := modelType + "-" + regionPreference + "-" + strategy
	if model, exists := metadata["model"]; exists {
		key += "-" + model
	}
	return key
}