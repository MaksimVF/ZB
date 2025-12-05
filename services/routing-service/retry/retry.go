



package retry

import (
	"math"
	"math/rand"
	"time"
)

// RetryConfig holds the configuration for retry logic
type RetryConfig struct {
	MaxAttempts     int
	InitialDelay    time.Duration
	MaxDelay        time.Duration
	JitterFactor    float64
	RetryableErrors map[error]bool
}

// DefaultConfig returns a default retry configuration
func DefaultConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:     3,
		InitialDelay:    100 * time.Millisecond,
		MaxDelay:        5 * time.Second,
		JitterFactor:    0.2,
		RetryableErrors: make(map[error]bool),
	}
}

// Do executes a function with retry logic
func Do(config RetryConfig, fn func() (interface{}, error)) (interface{}, error) {
	var lastErr error
	var result interface{}

	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		result, lastErr = fn()
		if lastErr == nil {
			return result, nil
		}

		// Check if error is retryable
		if !config.IsRetryable(lastErr) {
			return nil, lastErr
		}

		// Calculate delay with exponential backoff and jitter
		delay := config.calculateDelay(attempt)

		// Wait before next attempt
		time.Sleep(delay)
	}

	return nil, lastErr
}

// IsRetryable checks if an error should be retried
func (c RetryConfig) IsRetryable(err error) bool {
	if len(c.RetryableErrors) == 0 {
		// If no specific errors are configured, retry all errors
		return true
	}
	return c.RetryableErrors[err]
}

// calculateDelay calculates the delay with exponential backoff and jitter
func (c RetryConfig) calculateDelay(attempt int) time.Duration {
	// Exponential backoff: initialDelay * 2^(attempt-1)
	backoff := c.InitialDelay * time.Duration(math.Pow(2, float64(attempt-1)))

	// Apply max delay limit
	if backoff > c.MaxDelay {
		backoff = c.MaxDelay
	}

	// Add jitter: random value between [-jitterFactor*backoff, +jitterFactor*backoff]
	jitter := time.Duration(float64(backoff) * c.JitterFactor * (rand.Float64()*2 - 1))
	delay := backoff + jitter

	// Ensure delay is not negative
	if delay < 0 {
		delay = 0
	}

	return delay
}



