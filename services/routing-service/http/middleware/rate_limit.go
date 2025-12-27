package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/MaksimVF/ZB/services/routing-service/config"
)

// RateLimiter implements advanced rate limiting
type RateLimiter struct {
	mu            sync.Mutex
	requests      map[string]int
	lastRequest   map[string]time.Time
	threshold     int
	resetTimeout  time.Duration
	burstLimit    int
	burstDuration time.Duration
	ipThresholds  map[string]int
	ipBurstLimits map[string]int
	ipResetTimeouts map[string]time.Duration
	ipBurstDurations map[string]time.Duration
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(cfg *config.Config) *RateLimiter {
	return &RateLimiter{
		requests:      make(map[string]int),
		lastRequest:  make(map[string]time.Time),
		threshold:    cfg.RateLimit.Threshold,
		resetTimeout: cfg.RateLimit.ResetTimeout,
		burstLimit:   cfg.RateLimit.BurstLimit,
		burstDuration: cfg.RateLimit.BurstDuration,
		ipThresholds:  make(map[string]int),
		ipBurstLimits: make(map[string]int),
		ipResetTimeouts: make(map[string]time.Duration),
		ipBurstDurations: make(map[string]time.Duration),
	}
}

// Allow checks if request is allowed
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Get custom thresholds for IP
	threshold := rl.threshold
	if customThreshold, exists := rl.ipThresholds[ip]; exists {
		threshold = customThreshold
	}

	burstLimit := rl.burstLimit
	if customBurstLimit, exists := rl.ipBurstLimits[ip]; exists {
		burstLimit = customBurstLimit
	}

	resetTimeout := rl.resetTimeout
	if customResetTimeout, exists := rl.ipResetTimeouts[ip]; exists {
		resetTimeout = customResetTimeout
	}

	burstDuration := rl.burstDuration
	if customBurstDuration, exists := rl.ipBurstDurations[ip]; exists {
		burstDuration = customBurstDuration
	}

	// Check if rate limit is exceeded
	if requests, exists := rl.requests[ip]; exists && requests >= threshold {
		// Check if reset timeout has passed
		if lastRequest, exists := rl.lastRequest[ip]; exists {
			if time.Since(lastRequest) < resetTimeout {
				return false
			}
			// Reset rate limit
			delete(rl.requests, ip)
			delete(rl.lastRequest, ip)
		}
	}

	// Check burst limit
	if requests, exists := rl.requests[ip]; exists {
		if requests >= burstLimit {
			// Check if burst duration has passed
			if lastRequest, exists := rl.lastRequest[ip]; exists {
				if time.Since(lastRequest) < burstDuration {
					return false
				}
			}
		}
	}

	// Increment request count
	rl.requests[ip]++
	rl.lastRequest[ip] = time.Now()
	return true
}

// RateLimitMiddleware creates rate limiting middleware
func RateLimitMiddleware(limiter *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get client IP
			ip := getClientIP(r)

			// Check rate limit
			if !limiter.Allow(ip) {
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
				return
			}

			// Call the next handler
			next.ServeHTTP(w, r)
		})
	}
}

// getClientIP extracts client IP from request
func getClientIP(r *http.Request) string {
	// Try to get IP from X-Forwarded-For header
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		// Return the first IP in the list
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Fallback to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// SetThreshold sets custom threshold for IP
func (rl *RateLimiter) SetThreshold(ip string, threshold int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.ipThresholds[ip] = threshold
}

// SetBurstLimit sets custom burst limit for IP
func (rl *RateLimiter) SetBurstLimit(ip string, burstLimit int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.ipBurstLimits[ip] = burstLimit
}

// SetResetTimeout sets custom reset timeout for IP
func (rl *RateLimiter) SetResetTimeout(ip string, resetTimeout time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.ipResetTimeouts[ip] = resetTimeout
}

// SetBurstDuration sets custom burst duration for IP
func (rl *RateLimiter) SetBurstDuration(ip string, burstDuration time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.ipBurstDurations[ip] = burstDuration
}

// Reset resets rate limiting for IP
func (rl *RateLimiter) Reset(ip string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.requests, ip)
	delete(rl.lastRequest, ip)
	delete(rl.ipThresholds, ip)
	delete(rl.ipBurstLimits, ip)
	delete(rl.ipResetTimeouts, ip)
	delete(rl.ipBurstDurations, ip)
}