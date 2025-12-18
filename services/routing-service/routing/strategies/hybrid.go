package strategies

import (
	routing "github.com/MaksimVF/ZB/services/routing-service/routing"
	"github.com/MaksimVF/ZB/services/routing-service/storage"
)

// HybridStrategy combines multiple routing strategies
type HybridStrategy struct {
	registry routing.HeadRegistry
	cache    storage.Cache
	policy   *routing.RoutingPolicy
}

// NewHybridStrategy creates a new hybrid strategy
func NewHybridStrategy(registry routing.HeadRegistry, cache storage.Cache, policy *routing.RoutingPolicy) *HybridStrategy {
	return &HybridStrategy{
		registry: registry,
		cache:    cache,
		policy:   policy,
	}
}

// Name returns the strategy name
func (s *HybridStrategy) Name() string {
	return "hybrid"
}

// SelectHead combines multiple strategies for optimal routing
func (s *HybridStrategy) SelectHead(heads []routing.HeadService, req *routing.RoutingRequest) *routing.HeadService {
	if len(heads) == 0 {
		return nil
	}

	// Enhanced hybrid approach with adaptive routing
	// First try geo-preferred, then use predictive load balancing
	geoHead := NewGeoPreferredStrategy().SelectHead(heads, req)
	if geoHead != nil {
		// Check if the geo-preferred head can handle the load
		if s.canHandleLoad(geoHead) {
			return geoHead
		}
	}

	// Use predictive load balancing
	return NewPredictiveStrategy(s.cache, s.policy).SelectHead(heads, req)
}

// canHandleLoad checks if a head can handle additional load
func (s *HybridStrategy) canHandleLoad(head *routing.HeadService) bool {
	// Calculate utilization percentage
	if head.Capacity == 0 {
		return true // If capacity not set, assume it can handle load
	}

	utilization := float64(head.CurrentLoad) / float64(head.Capacity) * 100
	// Use the configured capacity threshold from policy
	threshold := s.policy.CapacityThreshold
	if threshold == 0 {
		threshold = 80.0 // Default to 80% if not configured
	}
	return utilization < threshold
}