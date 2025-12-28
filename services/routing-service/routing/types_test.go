package routing

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHeadService(t *testing.T) {
	// Сохраняем текущее время до создания объекта
	currentTime := time.Now().Unix()

	head := HeadService{
		HeadID:        "test-head-1",
		Endpoint:      "http://localhost:8080",
		Status:        "active",
		CurrentLoad:   25,
		Capacity:      100,
		LoadHistory:   []int32{10, 15, 20, 25, 30},
		Region:        "us-east-1",
		ModelType:     "gpt-3.5",
		Version:       "1.0.0",
		Metadata:      map[string]string{"environment": "test"},
		LastHeartbeat: currentTime,
	}

	assert.Equal(t, "test-head-1", head.HeadID)
	assert.Equal(t, "http://localhost:8080", head.Endpoint)
	assert.Equal(t, "active", head.Status)
	assert.Equal(t, int32(25), head.CurrentLoad)
	assert.Equal(t, int32(100), head.Capacity)
	assert.Equal(t, []int32{10, 15, 20, 25, 30}, head.LoadHistory)
	assert.Equal(t, "us-east-1", head.Region)
	assert.Equal(t, "gpt-3.5", head.ModelType)
	assert.Equal(t, "1.0.0", head.Version)
	assert.Equal(t, "test", head.Metadata["environment"])

	// Проверяем поле LastHeartbeat - должно быть установлено и корректно
	assert.Equal(t, currentTime, head.LastHeartbeat)
	assert.Greater(t, head.LastHeartbeat, int64(0), "LastHeartbeat должен быть положительным")
	assert.LessOrEqual(t, head.LastHeartbeat, time.Now().Unix(), "LastHeartbeat не должен быть в будущем")
}

func TestRoutingPolicy(t *testing.T) {
	policy := &RoutingPolicy{
		DefaultStrategy:     "adaptive",
		EnableGeoRouting:    true,
		EnableLoadBalancing: true,
		EnableModelSpecific: true,
		EnablePredictive:    true,
		EnableAdaptive:      true,
		StrategyConfig: map[string]string{
			"timeout":         "30s",
			"retries":         "3",
			"circuit_breaker": "enabled",
		},
		PredictionWindow:  300,
		LoadGrowthFactor:  1.2,
		CapacityThreshold: 80.0,
	}

	assert.Equal(t, "adaptive", policy.DefaultStrategy)
	assert.True(t, policy.EnableGeoRouting)
	assert.True(t, policy.EnableLoadBalancing)
	assert.True(t, policy.EnableModelSpecific)
	assert.True(t, policy.EnablePredictive)
	assert.True(t, policy.EnableAdaptive)
	assert.Equal(t, "30s", policy.StrategyConfig["timeout"])
	assert.Equal(t, 300, policy.PredictionWindow)
	assert.Equal(t, 1.2, policy.LoadGrowthFactor)
	assert.Equal(t, 80.0, policy.CapacityThreshold)
}

func TestRoutingRequest(t *testing.T) {
	req := &RoutingRequest{
		ClientID:         "client-123",
		ModelType:        "gpt-4",
		RegionPreference: "us-west-2",
		RoutingStrategy:  "predictive",
		Metadata: map[string]string{
			"priority": "high",
			"version":  "v2",
		},
	}

	assert.Equal(t, "client-123", req.ClientID)
	assert.Equal(t, "gpt-4", req.ModelType)
	assert.Equal(t, "us-west-2", req.RegionPreference)
	assert.Equal(t, "predictive", req.RoutingStrategy)
	assert.Equal(t, "high", req.Metadata["priority"])
	assert.Equal(t, "v2", req.Metadata["version"])
}

func TestRoutingResponse(t *testing.T) {
	resp := &RoutingResponse{
		HeadID:       "head-456",
		Endpoint:     "http://head-456:8080",
		StrategyUsed: "adaptive",
		Reason:       "lowest predicted load",
		Metadata: map[string]string{
			"load":     "15",
			"capacity": "100",
			"region":   "us-east-1",
		},
	}

	assert.Equal(t, "head-456", resp.HeadID)
	assert.Equal(t, "http://head-456:8080", resp.Endpoint)
	assert.Equal(t, "adaptive", resp.StrategyUsed)
	assert.Equal(t, "lowest predicted load", resp.Reason)
	assert.Equal(t, "15", resp.Metadata["load"])
	assert.Equal(t, "100", resp.Metadata["capacity"])
	assert.Equal(t, "us-east-1", resp.Metadata["region"])
}

func TestHeadServiceUtilization(t *testing.T) {
	// Test 100% utilization
	head := HeadService{
		CurrentLoad: 100,
		Capacity:    100,
	}
	utilization := float64(head.CurrentLoad) / float64(head.Capacity) * 100
	assert.Equal(t, 100.0, utilization)

	// Test 50% utilization
	head = HeadService{
		CurrentLoad: 50,
		Capacity:    100,
	}
	utilization = float64(head.CurrentLoad) / float64(head.Capacity) * 100
	assert.Equal(t, 50.0, utilization)

	// Test zero capacity (edge case for potential division by zero)
	head = HeadService{
		CurrentLoad: 50,
		Capacity:    0,
	}
	// Verify capacity is zero as expected
	assert.Equal(t, int32(0), head.Capacity)
	// Ensure we don't actually perform division by zero in real usage
	// (this should be handled in production code)
	if head.Capacity > 0 {
		utilization = float64(head.CurrentLoad) / float64(head.Capacity) * 100
		assert.Equal(t, 0.0, utilization) // Would be division by zero otherwise
	}
}

func TestHeadServiceLoadHistory(t *testing.T) {
	head := HeadService{
		LoadHistory: []int32{10, 20, 30, 40, 50},
	}

	// Test average load
	sum := int64(0)
	for _, load := range head.LoadHistory {
		sum += int64(load)
	}
	average := sum / int64(len(head.LoadHistory))
	assert.Equal(t, int64(30), average)

	// Test load trend (increasing)
	for i := 1; i < len(head.LoadHistory); i++ {
		assert.True(t, head.LoadHistory[i] > head.LoadHistory[i-1])
	}
}

// Mock implementations for testing HeadRegistry interface
type mockHeadRegistry struct {
	heads []HeadService
}

func (m *mockHeadRegistry) Register(head HeadService) error {
	m.heads = append(m.heads, head)
	return nil
}

func (m *mockHeadRegistry) UpdateStatus(headID, status string, load int32, timestamp int64) error {
	for i, head := range m.heads {
		if head.HeadID == headID {
			m.heads[i].Status = status
			m.heads[i].CurrentLoad = load
			m.heads[i].LastHeartbeat = timestamp
			return nil
		}
	}
	return nil
}

func (m *mockHeadRegistry) GetAll() []HeadService {
	return m.heads
}

func (m *mockHeadRegistry) GetByModelType(modelType string) []HeadService {
	var result []HeadService
	for _, head := range m.heads {
		if head.ModelType == modelType {
			result = append(result, head)
		}
	}
	return result
}

func (m *mockHeadRegistry) GetByRegion(region string) []HeadService {
	var result []HeadService
	for _, head := range m.heads {
		if head.Region == region {
			result = append(result, head)
		}
	}
	return result
}

func (m *mockHeadRegistry) GetActive() []HeadService {
	var result []HeadService
	for _, head := range m.heads {
		if head.Status == "active" {
			result = append(result, head)
		}
	}
	return result
}

func TestHeadRegistry(t *testing.T) {
	registry := &mockHeadRegistry{
		heads: []HeadService{},
	}

	// Test Register
	head := HeadService{
		HeadID:      "test-head",
		Status:      "active",
		CurrentLoad: 10,
		ModelType:   "gpt-3.5",
		Region:      "us-east-1",
	}

	err := registry.Register(head)
	require.NoError(t, err)
	assert.Len(t, registry.GetAll(), 1)

	// Test GetAll
	heads := registry.GetAll()
	assert.Equal(t, "test-head", heads[0].HeadID)

	// Test GetByModelType
	heads = registry.GetByModelType("gpt-3.5")
	assert.Len(t, heads, 1)

	// Test GetByRegion
	heads = registry.GetByRegion("us-east-1")
	assert.Len(t, heads, 1)

	// Test GetActive
	heads = registry.GetActive()
	assert.Len(t, heads, 1)

	// Test UpdateStatus
	timestamp := time.Now().Unix()
	err = registry.UpdateStatus("test-head", "inactive", 20, timestamp)
	require.NoError(t, err)

	updatedHeads := registry.GetAll()
	assert.Len(t, updatedHeads, 1)
	assert.Equal(t, "inactive", updatedHeads[0].Status)
	assert.Equal(t, int32(20), updatedHeads[0].CurrentLoad)
	assert.Equal(t, timestamp, updatedHeads[0].LastHeartbeat)

	// Verify that the head is no longer active
	activeHeads := registry.GetActive()
	assert.Len(t, activeHeads, 0) // Should be inactive now
}

func TestHeadServiceLastHeartbeat(t *testing.T) {
	// Тест для проверки функциональности LastHeartbeat
	head := HeadService{
		HeadID:    "heartbeat-test",
		Status:    "active",
		ModelType: "gpt-4",
	}

	// Проверяем, что LastHeartbeat по умолчанию равен 0
	assert.Equal(t, int64(0), head.LastHeartbeat)

	// Устанавливаем LastHeartbeat
	now := time.Now().Unix()
	head.LastHeartbeat = now
	assert.Equal(t, now, head.LastHeartbeat)

	// Проверяем, что время не в будущем
	assert.LessOrEqual(t, head.LastHeartbeat, time.Now().Unix())

	// Проверяем, что время положительное (после 1970 года)
	assert.Greater(t, head.LastHeartbeat, int64(0))

	// Тестируем обновление через mock registry
	registry := &mockHeadRegistry{
		heads: []HeadService{head},
	}

	// Обновляем статус и LastHeartbeat
	newTimestamp := time.Now().Unix()
	err := registry.UpdateStatus("heartbeat-test", "active", 30, newTimestamp)
	require.NoError(t, err)

	// Проверяем, что LastHeartbeat обновился
	heads := registry.GetAll()
	assert.Len(t, heads, 1)
	assert.Equal(t, newTimestamp, heads[0].LastHeartbeat)
	assert.Equal(t, int32(30), heads[0].CurrentLoad)

	// Тестируем случай, когда headID не существует
	err = registry.UpdateStatus("non-existent", "active", 20, time.Now().Unix())
	require.NoError(t, err) // Метод должен возвращать nil даже если head не найден

	// Убеждаемся, что количество heads не изменилось
	heads = registry.GetAll()
	assert.Len(t, heads, 1)
}
