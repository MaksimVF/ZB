











# Resilience Features

## Overview

The Gateway Service includes comprehensive resilience features to ensure high availability and fault tolerance:

- **Circuit Breakers**: Prevent cascading failures
- **Retry Logic**: Automatic retries with exponential backoff
- **Provider Failover**: Automatic switching between providers
- **Health Monitoring**: Continuous monitoring of provider health

## Circuit Breakers

### Configuration

Circuit breakers are configured for each provider:

```go
circuitBreakerConfigs := []resilience.CircuitBreakerConfig{
    {
        Name:          "openai",
        MaxRequests:    5,
        Interval:       60 * time.Second,
        Timeout:        10 * time.Second,
        ReadyToTrip:    resilience.DefaultReadyToTrip,
        OnStateChange: resilience.DefaultOnStateChange,
    },
    // Additional providers...
}
```

### States

- **Closed**: Normal operation
- **Open**: Circuit is open, requests fail fast
- **Half-Open**: Testing if provider has recovered

### Default Trip Function

```go
func DefaultReadyToTrip(counts gobreaker.Counts) bool {
    failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
    return counts.Requests >= 3 && failureRatio >= 0.6
}
```

## Retry Logic

### Configuration

- **Max Retries**: 3 attempts
- **Backoff**: Exponential backoff (1s, 2s, 4s)

### Implementation

```go
func WithRetry(operation func() error, maxRetries int, backoff time.Duration) error {
    var err error
    for i := 0; i < maxRetries; i++ {
        err = operation()
        if err == nil {
            return nil
        }

        if i < maxRetries-1 {
            time.Sleep(backoff * time.Duration(i+1))
        }
    }
    return fmt.Errorf("operation failed after %d retries: %w", maxRetries, err)
}
```

## Provider Failover

### Strategy

1. **Primary Provider**: First attempt
2. **Retry**: 3 attempts with backoff
3. **Circuit Breaker**: If circuit is open, fail fast
4. **Fallback**: Try alternative providers

### Implementation

```go
// Execute with circuit breaker and retry logic
providerName := getProviderName(providerConfig.BaseURL)
result, err := resilience.ExecuteWithCircuitBreaker(providerName, func() (interface{}, error) {
    return executeWithRetry(providerConfig, req, 3, 1*time.Second)
})
```

## Monitoring

### Metrics

- `circuit_breaker_state`: Current state of each circuit breaker
- `circuit_breaker_failures`: Number of failures
- `circuit_breaker_requests`: Total requests

### Endpoints

- **List Circuit Breakers**: `GET /v1/circuit-breakers`
- **Get Status**: `GET /v1/circuit-breakers/{name}`
- **Reset**: `POST /v1/circuit-breakers/{name}/reset`

## Configuration

### Environment Variables

- `CIRCUIT_BREAKER_MAX_REQUESTS`: Maximum requests before tripping
- `CIRCUIT_BREAKER_INTERVAL`: Reset interval
- `CIRCUIT_BREAKER_TIMEOUT`: Request timeout
- `RETRY_MAX_ATTEMPTS`: Maximum retry attempts
- `RETRY_BACKOFF`: Initial backoff duration

## Benefits

1. **Improved Availability**: Prevents cascading failures
2. **Better Performance**: Fails fast when providers are down
3. **Resilience**: Automatic recovery from temporary failures
4. **Observability**: Detailed monitoring and metrics

## Testing

### Unit Tests

```go
func TestCircuitBreaker(t *testing.T) {
    config := resilience.CircuitBreakerConfig{
        Name:          "test",
        MaxRequests:    3,
        Interval:       10 * time.Second,
        Timeout:        1 * time.Second,
        ReadyToTrip:    resilience.DefaultReadyToTrip,
    }

    resilience.InitCircuitBreakers([]resilience.CircuitBreakerConfig{config})

    // Test circuit breaker behavior
    result, err := resilience.ExecuteWithCircuitBreaker("test", func() (interface{}, error) {
        return "success", nil
    })

    assert.NoError(t, err)
    assert.Equal(t, "success", result)
}
```

### Integration Tests

```go
func TestRetryLogic(t *testing.T) {
    attempt := 0
    err := resilience.WithRetry(func() error {
        attempt++
        if attempt < 3 {
            return errors.New("temporary failure")
        }
        return nil
    }, 5, 100*time.Millisecond)

    assert.NoError(t, err)
    assert.Equal(t, 3, attempt)
}
```

## Best Practices

1. **Fail Fast**: Use circuit breakers to fail fast when providers are down
2. **Retry Smartly**: Use exponential backoff for retries
3. **Monitor**: Track circuit breaker states and metrics
4. **Test**: Regularly test failover scenarios
5. **Tune**: Adjust thresholds based on real-world usage

## Future Enhancements

1. **Adaptive Retries**: Machine learning-based retry strategies
2. **Predictive Failover**: Predict failures before they occur
3. **Chaos Testing**: Automated resilience testing
4. **Multi-Region Support**: Region-aware failover

## Support

For support, contact:

- Email: support@your-gateway.com
- Slack: #resilience-support
- GitHub: github.com/your-gateway/resilience

## License

This implementation is licensed under the MIT License.









