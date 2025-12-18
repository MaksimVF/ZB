package strategies

import (
	"sync/atomic"

	routing "github.com/MaksimVF/ZB/services/routing-service/routing"
)

// RoundRobinStrategy implements round-robin load balancing
type RoundRobinStrategy struct {
	counter uint64
}

// NewRoundRobinStrategy creates a new round-robin strategy
func NewRoundRobinStrategy() *RoundRobinStrategy {
	return &RoundRobinStrategy{}
}

// Name returns the strategy name
func (s *RoundRobinStrategy) Name() string {
	return "round_robin"
}

// SelectHead selects a head using round-robin algorithm
func (s *RoundRobinStrategy) SelectHead(heads []routing.HeadService, req *routing.RoutingRequest) *routing.HeadService {
	if len(heads) == 0 {
		return nil
	}

	// Get next index using atomic counter
	next := atomic.AddUint64(&s.counter, 1) - 1
	index := int(next % uint64(len(heads)))

	return &heads[index]
}