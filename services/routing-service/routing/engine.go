package routing

import (
	"fmt"
	"math/rand"
	"time"
)

// RoutingEngine handles routing decisions
type RoutingEngine struct {
	policy   *RoutingPolicy
	registry HeadRegistry
	cache    Cache
}

// Cache interface for caching
type Cache interface {
	Get(key string) (interface{}, bool)
	Set(key string, value interface{}, ttl time.Duration)
}

// NewRoutingEngine creates a new routing engine
func NewRoutingEngine(policy *RoutingPolicy, registry HeadRegistry, cache Cache, policyManager PolicyManager) *RoutingEngine {
	return &RoutingEngine{
		policy:   policy,
		registry: registry,
		cache:    cache,
	}
}

// GetDecision makes a routing decision
func (e *RoutingEngine) GetDecision(req *RoutingRequest) (*RoutingResponse, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("%s-%s-%s", req.ModelType, req.RegionPreference, req.RoutingStrategy)
	if cached, found := e.cache.Get(cacheKey); found {
		if headID, ok := cached.(string); ok {
			heads := e.registry.GetAll()
			for _, head := range heads {
				if head.HeadID == headID && head.Status == "active" {
					return &RoutingResponse{
						HeadID:       head.HeadID,
						Endpoint:     head.Endpoint,
						StrategyUsed: "cached",
						Reason:       "Cached decision",
						Metadata:     req.Metadata,
					}, nil
				}
			}
		}
	}

	// Get candidates
	candidates := e.registry.GetByModelType(req.ModelType)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no heads available for model type %s", req.ModelType)
	}

	// Apply strategy
	var selected *HeadService
	var reason string

	switch req.RoutingStrategy {
	case "round_robin":
		selected = e.applyRoundRobin(candidates)
		reason = "Round-robin selection"
	case "least_loaded":
		selected = e.applyLeastLoaded(candidates)
		reason = "Least loaded selection"
	case "geo_preferred":
		selected = e.applyGeoPreferred(candidates, req.RegionPreference)
		reason = "Geo-preferred selection"
	default:
		selected = e.applyRoundRobin(candidates)
		reason = "Default round-robin selection"
	}

	if selected == nil {
		return nil, fmt.Errorf("no suitable head found")
	}

	// Cache result
	e.cache.Set(cacheKey, selected.HeadID, 5*time.Minute)

	return &RoutingResponse{
		HeadID:       selected.HeadID,
		Endpoint:     selected.Endpoint,
		StrategyUsed: req.RoutingStrategy,
		Reason:       reason,
		Metadata:     req.Metadata,
	}, nil
}

// applyRoundRobin implements round-robin selection
func (e *RoutingEngine) applyRoundRobin(heads []HeadService) *HeadService {
	if len(heads) == 0 {
		return nil
	}
	return &heads[rand.Intn(len(heads))]
}

// applyLeastLoaded selects the head with lowest load
func (e *RoutingEngine) applyLeastLoaded(heads []HeadService) *HeadService {
	if len(heads) == 0 {
		return nil
	}
	minLoad := heads[0]
	for _, head := range heads[1:] {
		if head.CurrentLoad < minLoad.CurrentLoad {
			minLoad = head
		}
	}
	return &minLoad
}

// applyGeoPreferred selects head in preferred region
func (e *RoutingEngine) applyGeoPreferred(heads []HeadService, region string) *HeadService {
	for _, head := range heads {
		if head.Region == region {
			return &head
		}
	}
	// Fallback to round-robin
	return e.applyRoundRobin(heads)
}
