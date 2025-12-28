package strategies

import (
	"testing"

	routing "github.com/MaksimVF/ZB/services/routing-service/routing"
	"github.com/stretchr/testify/assert"
)

func TestAdaptiveStrategy_Name(t *testing.T) {
	strategy := NewAdaptiveStrategy(nil, &routing.RoutingPolicy{})
	assert.Equal(t, "adaptive", strategy.Name())
}

func TestAdaptiveStrategy_SelectHead(t *testing.T) {
	policy := &routing.RoutingPolicy{
		CapacityThreshold: 80.0,
	}
	strategy := NewAdaptiveStrategy(nil, policy)

	// Test empty heads list
	result := strategy.SelectHead([]routing.HeadService{}, &routing.RoutingRequest{})
	assert.Nil(t, result)

	// Test single head with low load
	head := routing.HeadService{
		HeadID:      "test-head-1",
		CurrentLoad: 10,
		Capacity:    100,
		Status:      "active",
	}
	result = strategy.SelectHead([]routing.HeadService{head}, &routing.RoutingRequest{})
	assert.NotNil(t, result)
	assert.Equal(t, "test-head-1", result.HeadID)
}

func TestAdaptiveStrategy_canHandleLoad(t *testing.T) {
	policy := &routing.RoutingPolicy{
		CapacityThreshold: 80.0,
	}
	strategy := NewAdaptiveStrategy(nil, policy)

	// Test head with zero capacity (should allow)
	head := routing.HeadService{
		Capacity: 0,
	}
	assert.True(t, strategy.canHandleLoad(&head))

	// Test head with low utilization
	head = routing.HeadService{
		CurrentLoad: 10,
		Capacity:    100,
	}
	assert.True(t, strategy.canHandleLoad(&head))

	// Test head with high utilization
	head = routing.HeadService{
		CurrentLoad: 90,
		Capacity:    100,
	}
	assert.False(t, strategy.canHandleLoad(&head))

	// Test custom threshold
	policy.CapacityThreshold = 50.0
	head = routing.HeadService{
		CurrentLoad: 60,
		Capacity:    100,
	}
	assert.False(t, strategy.canHandleLoad(&head))
}
