package strategies

import (
	routing "github.com/MaksimVF/ZB/services/routing-service/routing"
)

// LeastLoadedStrategy selects the head with the lowest current load
type LeastLoadedStrategy struct{}

// NewLeastLoadedStrategy creates a new least-loaded strategy
func NewLeastLoadedStrategy() *LeastLoadedStrategy {
	return &LeastLoadedStrategy{}
}

// Name returns the strategy name
func (s *LeastLoadedStrategy) Name() string {
	return "least_loaded"
}

// SelectHead selects the head with the lowest load
func (s *LeastLoadedStrategy) SelectHead(heads []routing.HeadService, req *routing.RoutingRequest) *routing.HeadService {
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