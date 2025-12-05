



package main

import (
	"errors"
	"testing"
	"time"

	"github.com/MaksimVF/ZB/services/routing-service/retry"
	"github.com/stretchr/testify/assert"
)

func TestRetryLogic(t *testing.T) {
	// Test successful operation (no retries needed)
	successConfig := retry.DefaultConfig()
	successConfig.MaxAttempts = 3

	successCount := 0
	successFn := func() (interface{}, error) {
		successCount++
		return "success", nil
	}

	result, err := retry.Do(successConfig, successFn)
	assert.NoError(t, err)
	assert.Equal(t, "success", result)
	assert.Equal(t, 1, successCount) // Should succeed on first attempt

	// Test retry logic with temporary failure
	failureConfig := retry.DefaultConfig()
	failureConfig.MaxAttempts = 3
	failureConfig.InitialDelay = 10 * time.Millisecond
	failureConfig.MaxDelay = 50 * time.Millisecond

	failureCount := 0
	failureFn := func() (interface{}, error) {
		failureCount++
		if failureCount < 2 {
			return nil, errors.New("temporary error")
		}
		return "success after retry", nil
	}

	result, err = retry.Do(failureConfig, failureFn)
	assert.NoError(t, err)
	assert.Equal(t, "success after retry", result)
	assert.Equal(t, 2, failureCount) // Should succeed on second attempt

	// Test max retries reached
	maxRetryConfig := retry.DefaultConfig()
	maxRetryConfig.MaxAttempts = 3
	maxRetryConfig.InitialDelay = 10 * time.Millisecond

	maxRetryCount := 0
	maxRetryFn := func() (interface{}, error) {
		maxRetryCount++
		return nil, errors.New("permanent error")
	}

	result, err = retry.Do(maxRetryConfig, maxRetryFn)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, 3, maxRetryCount) // Should fail after 3 attempts

	// Test exponential backoff with jitter
	backoffConfig := retry.DefaultConfig()
	backoffConfig.MaxAttempts = 5
	backoffConfig.InitialDelay = 10 * time.Millisecond
	backoffConfig.MaxDelay = 100 * time.Millisecond
	backoffConfig.JitterFactor = 0.2

	backoffCount := 0
	backoffFn := func() (interface{}, error) {
		backoffCount++
		if backoffCount < 3 {
			return nil, errors.New("retryable error")
		}
		return "success after backoff", nil
	}

	startTime := time.Now()
	result, err = retry.Do(backoffConfig, backoffFn)
	duration := time.Since(startTime)

	assert.NoError(t, err)
	assert.Equal(t, "success after backoff", result)
	assert.Equal(t, 3, backoffCount)
	assert.True(t, duration >= 10*time.Millisecond) // Should have some delay
}

func TestRetryableErrors(t *testing.T) {
	// Test specific retryable errors
	retryableError := errors.New("retryable error")
	nonRetryableError := errors.New("non-retryable error")

	config := retry.DefaultConfig()
	config.MaxAttempts = 3
	config.RetryableErrors = map[error]bool{
		retryableError: true,
		nonRetryableError: false,
	}

	// Test retryable error
	retryableCount := 0
	retryableFn := func() (interface{}, error) {
		retryableCount++
		if retryableCount < 2 {
			return nil, retryableError
		}
		return "success", nil
	}

	result, err := retry.Do(config, retryableFn)
	assert.NoError(t, err)
	assert.Equal(t, "success", result)
	assert.Equal(t, 2, retryableCount)

	// Test non-retryable error
	nonRetryableCount := 0
	nonRetryableFn := func() (interface{}, error) {
		nonRetryableCount++
		return nil, nonRetryableError
	}

	result, err = retry.Do(config, nonRetryableFn)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, 1, nonRetryableCount) // Should fail immediately
}


