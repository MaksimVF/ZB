












package resilience

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCircuitBreakerInitialization(t *testing.T) {
	configs := []CircuitBreakerConfig{
		{
			Name:          "test",
			MaxRequests:    3,
			Interval:       10 * time.Second,
			Timeout:        1 * time.Second,
			ReadyToTrip:    DefaultReadyToTrip,
			OnStateChange: DefaultOnStateChange,
		},
	}

	InitCircuitBreakers(configs)

	// Verify circuit breaker was created
	cb, err := GetCircuitBreaker("test")
	require.NoError(t, err)
	require.NotNil(t, cb)
}

func TestCircuitBreakerExecution(t *testing.T) {
	configs := []CircuitBreakerConfig{
		{
			Name:          "test",
			MaxRequests:    3,
			Interval:       10 * time.Second,
			Timeout:        1 * time.Second,
			ReadyToTrip:    DefaultReadyToTrip,
			OnStateChange: DefaultOnStateChange,
		},
	}

	InitCircuitBreakers(configs)

	// Test successful execution
	result, err := ExecuteWithCircuitBreaker("test", func() (interface{}, error) {
		return "success", nil
	})

	assert.NoError(t, err)
	assert.Equal(t, "success", result)
}

func TestCircuitBreakerFailure(t *testing.T) {
	configs := []CircuitBreakerConfig{
		{
			Name:          "test",
			MaxRequests:    3,
			Interval:       10 * time.Second,
			Timeout:        1 * time.Second,
			ReadyToTrip:    DefaultReadyToTrip,
			OnStateChange: DefaultOnStateChange,
		},
	}

	InitCircuitBreakers(configs)

	// Test failure execution
	result, err := ExecuteWithCircuitBreaker("test", func() (interface{}, error) {
		return nil, errors.New("test error")
	})

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestRetryLogic(t *testing.T) {
	attempt := 0
	err := WithRetry(func() error {
		attempt++
		if attempt < 3 {
			return errors.New("temporary failure")
		}
		return nil
	}, 5, 100*time.Millisecond)

	assert.NoError(t, err)
	assert.Equal(t, 3, attempt)
}

func TestRetryFailure(t *testing.T) {
	err := WithRetry(func() error {
		return errors.New("permanent failure")
	}, 3, 100*time.Millisecond)

	assert.Error(t, err)
}

func TestCircuitBreakerState(t *testing.T) {
	configs := []CircuitBreakerConfig{
		{
			Name:          "test",
			MaxRequests:    3,
			Interval:       10 * time.Second,
			Timeout:        1 * time.Second,
			ReadyToTrip:    DefaultReadyToTrip,
			OnStateChange: DefaultOnStateChange,
		},
	}

	InitCircuitBreakers(configs)

	// Test initial state
	state, _, err := GetCircuitBreakerStatus("test")
	require.NoError(t, err)
	assert.Equal(t, gobreaker.StateClosed, state)

	// Trip the circuit breaker
	for i := 0; i < 5; i++ {
		_, _ = ExecuteWithCircuitBreaker("test", func() (interface{}, error) {
			return nil, errors.New("failure")
		})
	}

	// Check if circuit is open
	state, _, err = GetCircuitBreakerStatus("test")
	require.NoError(t, err)
	assert.Equal(t, gobreaker.StateOpen, state)
}

func TestCircuitBreakerReset(t *testing.T) {
	configs := []CircuitBreakerConfig{
		{
			Name:          "test",
			MaxRequests:    3,
			Interval:       10 * time.Second,
			Timeout:        1 * time.Second,
			ReadyToTrip:    DefaultReadyToTrip,
			OnStateChange: DefaultOnStateChange,
		},
	}

	InitCircuitBreakers(configs)

	// Trip the circuit breaker
	for i := 0; i < 5; i++ {
		_, _ = ExecuteWithCircuitBreaker("test", func() (interface{}, error) {
			return nil, errors.New("failure")
		})
	}

	// Reset the circuit breaker
	err := ResetCircuitBreaker("test")
	require.NoError(t, err)

	// Check if circuit is closed
	state, _, err := GetCircuitBreakerStatus("test")
	require.NoError(t, err)
	assert.Equal(t, gobreaker.StateClosed, state)
}

func TestReadyToTripFunction(t *testing.T) {
	counts := gobreaker.Counts{
		Requests:      10,
		TotalSuccesses: 4,
		TotalFailures:  6,
	}

	result := DefaultReadyToTrip(counts)
	assert.True(t, result)

	counts = gobreaker.Counts{
		Requests:      2,
		TotalSuccesses: 2,
		TotalFailures:  0,
	}

	result = DefaultReadyToTrip(counts)
	assert.False(t, result)
}








