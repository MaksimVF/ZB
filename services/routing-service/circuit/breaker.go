package circuit

import (
	"sync"
	"time"

	"github.com/MaksimVF/ZB/services/routing-service/config"
)

// CircuitBreaker implements circuit breaker pattern
type CircuitBreaker struct {
	mu               sync.Mutex
	failures         map[string]int
	lastFailure      map[string]time.Time
	threshold        int
	resetTimeout     time.Duration
	halfOpen         bool
	halfOpenUntil    time.Time
	halfOpenDuration time.Duration
	successCount     map[string]int
	recoveryAttempts map[string]int
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(cfg *config.Config) *CircuitBreaker {
	return &CircuitBreaker{
		failures:         make(map[string]int),
		lastFailure:      make(map[string]time.Time),
		threshold:        cfg.CircuitBreaker.Threshold,
		resetTimeout:     cfg.CircuitBreaker.ResetTimeout,
		halfOpenDuration: cfg.CircuitBreaker.HalfOpenDuration,
		successCount:     make(map[string]int),
		recoveryAttempts: make(map[string]int),
	}
}

// Allow checks if request is allowed based on circuit breaker state
func (cb *CircuitBreaker) Allow(service string) bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Check if circuit breaker is open
	if failures, exists := cb.failures[service]; exists && failures >= cb.threshold {
		// Check if reset timeout has passed
		if lastFailure, exists := cb.lastFailure[service]; exists {
			if time.Since(lastFailure) < cb.resetTimeout {
				// Circuit is open
				return false
			}

			// Check if we're in half-open state
			if cb.halfOpen && time.Now().Before(cb.halfOpenUntil) {
				// Allow one request to test the service
				cb.halfOpen = false
				return true
			}

			// Reset circuit breaker
			delete(cb.failures, service)
			delete(cb.lastFailure, service)
			cb.halfOpen = true
			cb.halfOpenUntil = time.Now().Add(cb.halfOpenDuration)
			return true
		}
	}
	return true
}

// Fail records a failure
func (cb *CircuitBreaker) Fail(service string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Increment failure count
	cb.failures[service]++
	cb.lastFailure[service] = time.Now()
	cb.halfOpen = false
}

// Success records a success
func (cb *CircuitBreaker) Success(service string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Increment success count
	cb.successCount[service]++
	cb.recoveryAttempts[service]++

	// Reset failure count on success
	delete(cb.failures, service)
	delete(cb.lastFailure, service)
	cb.halfOpen = false
}

// State returns the current state of the circuit breaker for a service
func (cb *CircuitBreaker) State(service string) string {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if failures, exists := cb.failures[service]; exists && failures >= cb.threshold {
		if lastFailure, exists := cb.lastFailure[service]; exists {
			if time.Since(lastFailure) < cb.resetTimeout {
				return "open"
			}
			return "half-open"
		}
	}
	return "closed"
}

// Reset resets the circuit breaker for a service
func (cb *CircuitBreaker) Reset(service string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	delete(cb.failures, service)
	delete(cb.lastFailure, service)
	delete(cb.successCount, service)
	delete(cb.recoveryAttempts, service)
	cb.halfOpen = false
}

// SetThreshold sets custom threshold for a service
func (cb *CircuitBreaker) SetThreshold(service string, threshold int) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	// In a more advanced implementation, this would store per-service thresholds
	// For now, we use the global threshold
	_ = service
	_ = threshold
}

// SetResetTimeout sets custom reset timeout for a service
func (cb *CircuitBreaker) SetResetTimeout(service string, resetTimeout time.Duration) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	// In a more advanced implementation, this would store per-service timeouts
	// For now, we use the global timeout
	_ = service
	_ = resetTimeout
}

// GetMetrics returns circuit breaker metrics
func (cb *CircuitBreaker) GetMetrics() map[string]interface{} {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	metrics := make(map[string]interface{})
	for service, failures := range cb.failures {
		metrics[service] = map[string]interface{}{
			"failures":        failures,
			"last_failure":    cb.lastFailure[service],
			"state":           cb.State(service),
			"success_count":   cb.successCount[service],
			"recovery_attempts": cb.recoveryAttempts[service],
		}
	}
	return metrics
}