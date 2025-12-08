

package middleware

import (
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

var (
	rateLimiter  = rate.NewLimiter(10, 50) // 10 requests per second, burst of 50
	rateLimiterMu sync.Mutex
)

func RateLimiter(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rateLimiterMu.Lock()
		if !rateLimiter.Allow() {
			rateLimiterMu.Unlock()
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}
		rateLimiterMu.Unlock()

		next.ServeHTTP(w, r)
	}
}


