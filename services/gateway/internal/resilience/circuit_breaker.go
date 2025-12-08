












package resilience

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/sony/gobreaker"
	"github.com/rs/zerolog"
)

var (
	circuitBreakers = make(map[string]*gobreaker.CircuitBreaker)
	cbMutex        = &sync.RWMutex{}
	logger          = zerolog.New(os.Stdout).With().Timestamp().Str("service", "resilience").Logger()
)

type CircuitBreakerConfig struct {
	Name          string
	MaxRequests    uint32
	Interval       time.Duration
	Timeout        time.Duration
	ReadyToTrip    func(counts gobreaker.Counts) bool
	OnStateChange   func(name string, from gobreaker.State, to gobreaker.State)
}

func InitCircuitBreakers(configs []CircuitBreakerConfig) {
	cbMutex.Lock()
	defer cbMutex.Unlock()

	for _, config := range configs {
		cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
			Name:        config.Name,
			MaxRequests: config.MaxRequests,
			Interval:    config.Interval,
			Timeout:     config.Timeout,
			ReadyToTrip: config.ReadyToTrip,
			OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
				logger.Info().
					Str("circuit_breaker", name).
					Str("from", from.String()).
					Str("to", to.String()).
					Msg("Circuit breaker state change")
				if config.OnStateChange != nil {
					config.OnStateChange(name, from, to)
				}
			},
		})
		circuitBreakers[config.Name] = cb
		logger.Info().Str("circuit_breaker", config.Name).Msg("Initialized circuit breaker")
	}
}

func GetCircuitBreaker(name string) (*gobreaker.CircuitBreaker, error) {
	cbMutex.RLock()
	defer cbMutex.RUnlock()

	cb, ok := circuitBreakers[name]
	if !ok {
		return nil, fmt.Errorf("circuit breaker %s not found", name)
	}
	return cb, nil
}

func ExecuteWithCircuitBreaker(name string, operation func() (interface{}, error)) (interface{}, error) {
	cb, err := GetCircuitBreaker(name)
	if err != nil {
		return nil, err
	}

	result, err := cb.Execute(operation)
	if err != nil {
		logger.Warn().
			Str("circuit_breaker", name).
			Str("state", cb.State().String()).
			Err(err).
			Msg("Circuit breaker operation failed")
		return nil, err
	}

	return result, nil
}

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

func IsCircuitOpen(name string) (bool, error) {
	cb, err := GetCircuitBreaker(name)
	if err != nil {
		return false, err
	}
	return cb.State() == gobreaker.StateOpen, nil
}

func ResetCircuitBreaker(name string) error {
	cb, err := GetCircuitBreaker(name)
	if err != nil {
		return err
	}
	cb.Reset()
	return nil
}

func GetCircuitBreakerStatus(name string) (gobreaker.State, gobreaker.Counts, error) {
	cb, err := GetCircuitBreaker(name)
	if err != nil {
		return gobreaker.StateClosed, gobreaker.Counts{}, err
	}
	return cb.State(), cb.Counts(), nil
}

func GetAllCircuitBreakers() map[string]gobreaker.State {
	cbMutex.RLock()
	defer cbMutex.RUnlock()

	status := make(map[string]gobreaker.State)
	for name, cb := range circuitBreakers {
		status[name] = cb.State()
	}
	return status
}

func AddCircuitBreaker(config CircuitBreakerConfig) error {
	cbMutex.Lock()
	defer cbMutex.Unlock()

	if _, exists := circuitBreakers[config.Name]; exists {
		return fmt.Errorf("circuit breaker %s already exists", config.Name)
	}

	cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        config.Name,
		MaxRequests: config.MaxRequests,
		Interval:    config.Interval,
		Timeout:     config.Timeout,
		ReadyToTrip: config.ReadyToTrip,
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			logger.Info().
				Str("circuit_breaker", name).
				Str("from", from.String()).
				Str("to", to.String()).
				Msg("Circuit breaker state change")
			if config.OnStateChange != nil {
				config.OnStateChange(name, from, to)
			}
		},
	})

	circuitBreakers[config.Name] = cb
	logger.Info().Str("circuit_breaker", config.Name).Msg("Added circuit breaker")
	return nil
}

func RemoveCircuitBreaker(name string) error {
	cbMutex.Lock()
	defer cbMutex.Unlock()

	if _, exists := circuitBreakers[name]; !exists {
		return fmt.Errorf("circuit breaker %s not found", name)
	}

	delete(circuitBreakers, name)
	logger.Info().Str("circuit_breaker", name).Msg("Removed circuit breaker")
	return nil
}

func DefaultReadyToTrip(counts gobreaker.Counts) bool {
	failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
	return counts.Requests >= 3 && failureRatio >= 0.6
}

func DefaultOnStateChange(name string, from gobreaker.State, to gobreaker.State) {
	// Default implementation does nothing, can be overridden
}








