package strategies

import (
	routing "github.com/MaksimVF/ZB/services/routing-service/routing"
	"github.com/MaksimVF/ZB/services/routing-service/storage"
)

// PredictiveStrategy selects a head based on predicted future load
type PredictiveStrategy struct {
	cache  storage.Cache
	policy *routing.RoutingPolicy
}

// NewPredictiveStrategy creates a new predictive strategy
func NewPredictiveStrategy(cache storage.Cache, policy *routing.RoutingPolicy) *PredictiveStrategy {
	return &PredictiveStrategy{
		cache:  cache,
		policy: policy,
	}
}

// Name returns the strategy name
func (s *PredictiveStrategy) Name() string {
	return "predictive"
}

// SelectHead selects a head based on predicted future load
func (s *PredictiveStrategy) SelectHead(heads []routing.HeadService, req *routing.RoutingRequest) *routing.HeadService {
	if len(heads) == 0 {
		return nil
	}

	// Find the head with the best predicted future load
	var bestHead *routing.HeadService
	var lowestPredictedLoad int32 = -1

	for i, head := range heads {
		// Predict future load for this head
		predictedLoad := s.predictFutureLoad(head)

		// Initialize with first head
		if bestHead == nil || predictedLoad < lowestPredictedLoad {
			bestHead = &heads[i]
			lowestPredictedLoad = predictedLoad
		}
	}

	return bestHead
}

// predictFutureLoad predicts future load based on historical data
func (s *PredictiveStrategy) predictFutureLoad(head routing.HeadService) int32 {
	// Simple moving average prediction
	if len(head.LoadHistory) == 0 {
		return head.CurrentLoad
	}

	// Calculate moving average (last 5 data points or all available)
	start := 0
	if len(head.LoadHistory) > 5 {
		start = len(head.LoadHistory) - 5
	}

	sum := int64(0)
	count := 0
	for i := start; i < len(head.LoadHistory); i++ {
		sum += int64(head.LoadHistory[i])
		count++
	}

	if count == 0 {
		return head.CurrentLoad
	}

	average := sum / int64(count)

	// Apply growth factor from policy configuration
	growthFactor := s.policy.LoadGrowthFactor
	if growthFactor == 0 {
		growthFactor = 1.1 // Default to 10% growth if not configured
	}
	predicted := int32(float64(average) * growthFactor)

	// Don't predict lower than current load
	if predicted < head.CurrentLoad {
		return head.CurrentLoad
	}

	return predicted
}