package strategies

import (
	routing "github.com/MaksimVF/ZB/services/routing-service/routing"
)

// AdaptiveStrategy selects a head based on real-time conditions and performance
type AdaptiveStrategy struct {
	registry routing.HeadRegistry
	policy   *routing.RoutingPolicy
}

// NewAdaptiveStrategy creates a new adaptive strategy
func NewAdaptiveStrategy(registry routing.HeadRegistry, policy *routing.RoutingPolicy) *AdaptiveStrategy {
	return &AdaptiveStrategy{
		registry: registry,
		policy:   policy,
	}
}

// Name returns the strategy name
func (s *AdaptiveStrategy) Name() string {
	return "adaptive"
}

// SelectHead selects a head based on real-time conditions
func (s *AdaptiveStrategy) SelectHead(heads []routing.HeadService, req *routing.RoutingRequest) *routing.HeadService {
	if len(heads) == 0 {
		return nil
	}

	// First check for model-specific requirements
	modelHead := NewModelSpecificStrategy().SelectHead(heads, req)
	if modelHead != nil && s.canHandleLoad(modelHead) {
		return modelHead
	}

	// Check geo-preference
	geoHead := NewGeoPreferredStrategy().SelectHead(heads, req)
	if geoHead != nil && s.canHandleLoad(geoHead) {
		return geoHead
	}

	// Use predictive load balancing
	return NewPredictiveStrategy(nil, s.policy).SelectHead(heads, req)
}

// canHandleLoad checks if a head can handle additional load
func (s *AdaptiveStrategy) canHandleLoad(head *routing.HeadService) bool {
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