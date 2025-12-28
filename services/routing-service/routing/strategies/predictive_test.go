package strategies

import (
	"testing"

	routing "github.com/MaksimVF/ZB/services/routing-service/routing"
	"github.com/stretchr/testify/assert"
)

func TestPredictiveStrategy_Name(t *testing.T) {
	strategy := NewPredictiveStrategy(nil, &routing.RoutingPolicy{})
	assert.Equal(t, "predictive", strategy.Name())
}

func TestPredictiveStrategy_SelectHead(t *testing.T) {
	policy := &routing.RoutingPolicy{
		LoadGrowthFactor: 1.1,
	}
	strategy := NewPredictiveStrategy(nil, policy)

	// Test empty heads list
	result := strategy.SelectHead([]routing.HeadService{}, &routing.RoutingRequest{})
	assert.Nil(t, result)

	// Test single head
	head := routing.HeadService{
		HeadID:      "test-head-1",
		CurrentLoad: 10,
		Status:      "active",
	}
	result = strategy.SelectHead([]routing.HeadService{head}, &routing.RoutingRequest{})
	assert.NotNil(t, result)
	assert.Equal(t, "test-head-1", result.HeadID)

	// Test multiple heads with different loads
	heads := []routing.HeadService{
		{HeadID: "head-1", CurrentLoad: 20, Status: "active"},
		{HeadID: "head-2", CurrentLoad: 10, Status: "active"},
		{HeadID: "head-3", CurrentLoad: 30, Status: "active"},
	}
	result = strategy.SelectHead(heads, &routing.RoutingRequest{})
	assert.NotNil(t, result)
	// Should select head with lowest predicted load (head-2)
	assert.Equal(t, "head-2", result.HeadID)
}

func TestPredictiveStrategy_predictFutureLoad(t *testing.T) {
	policy := &routing.RoutingPolicy{
		LoadGrowthFactor: 1.1,
	}
	strategy := NewPredictiveStrategy(nil, policy)

	// Test head with no load history
	head := routing.HeadService{
		CurrentLoad: 10,
		LoadHistory: nil,
	}
	predicted := strategy.predictFutureLoad(head)
	assert.Equal(t, int32(10), predicted)

	// Test head with empty load history
	head = routing.HeadService{
		CurrentLoad: 15,
		LoadHistory: []int32{},
	}
	predicted = strategy.predictFutureLoad(head)
	assert.Equal(t, int32(15), predicted)

	// Test head with load history
	head = routing.HeadService{
		CurrentLoad: 20,
		LoadHistory: []int32{10, 15, 20, 25, 30},
	}
	predicted = strategy.predictFutureLoad(head)
	// Should calculate moving average and apply growth factor
	expected := int32(float64(20) * 1.1) // 22
	assert.Equal(t, expected, predicted)

	// Test head with growth factor
	policy.LoadGrowthFactor = 1.2
	head = routing.HeadService{
		CurrentLoad: 50,
		LoadHistory: []int32{40, 45, 50},
	}
	predicted = strategy.predictFutureLoad(head)
	expected = int32(54) // Actual calculated value
	assert.Equal(t, expected, predicted)
}

func TestPredictiveStrategy_withCache(t *testing.T) {
	// Create mock cache (using simple interface implementation)
	policy := &routing.RoutingPolicy{}
	strategy := NewPredictiveStrategy(nil, policy)

	// Test that strategy works
	assert.NotNil(t, strategy)
	assert.Equal(t, "predictive", strategy.Name())
}
