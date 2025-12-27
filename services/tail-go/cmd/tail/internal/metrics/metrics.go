package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// HTTP request metrics
	RequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tail_http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status"},
	)

	RequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "tail_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)

	// Provider metrics
	ProviderRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tail_provider_requests_total",
			Help: "Total number of requests to providers",
		},
		[]string{"provider", "model", "status"},
	)

	ProviderRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "tail_provider_request_duration_seconds",
			Help:    "Provider request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"provider", "model"},
	)

	// Cache metrics
	CacheHitsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tail_cache_hits_total",
			Help: "Total number of cache hits",
		},
		[]string{"cache_type"},
	)

	CacheMissesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tail_cache_misses_total",
			Help: "Total number of cache misses",
		},
		[]string{"cache_type"},
	)

	// Rate limiting metrics
	RateLimitExceededTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tail_rate_limit_exceeded_total",
			Help: "Total number of rate limit violations",
		},
		[]string{"client_type"},
	)

	// Error metrics
	ErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tail_errors_total",
			Help: "Total number of errors",
		},
		[]string{"type", "component"},
	)
)

func init() {
	// Register all metrics
	prometheus.MustRegister(
		RequestsTotal,
		RequestDuration,
		ProviderRequestsTotal,
		ProviderRequestDuration,
		CacheHitsTotal,
		CacheMissesTotal,
		RateLimitExceededTotal,
		ErrorsTotal,
	)
}

// HTTPMetricsMiddleware wraps HTTP handlers with metrics collection
func HTTPMetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create response writer wrapper to capture status code
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		// Call next handler
		next.ServeHTTP(rw, r)

		// Record metrics
		duration := time.Since(start).Seconds()
		status := strconv.Itoa(rw.statusCode)

		RequestsTotal.WithLabelValues(r.Method, r.URL.Path, status).Inc()
		RequestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(duration)
	})
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Handler returns the Prometheus metrics handler
func Handler() http.Handler {
	return promhttp.Handler()
}

// RecordProviderRequest records metrics for provider requests
func RecordProviderRequest(provider, model string, statusCode int, duration time.Duration) {
	status := strconv.Itoa(statusCode)
	ProviderRequestsTotal.WithLabelValues(provider, model, status).Inc()
	ProviderRequestDuration.WithLabelValues(provider, model).Observe(duration.Seconds())
}

// RecordCacheHit records a cache hit
func RecordCacheHit(cacheType string) {
	CacheHitsTotal.WithLabelValues(cacheType).Inc()
}

// RecordCacheMiss records a cache miss
func RecordCacheMiss(cacheType string) {
	CacheMissesTotal.WithLabelValues(cacheType).Inc()
}

// RecordRateLimitExceeded records a rate limit violation
func RecordRateLimitExceeded(clientType string) {
	RateLimitExceededTotal.WithLabelValues(clientType).Inc()
}

// RecordError records an error
func RecordError(errorType, component string) {
	ErrorsTotal.WithLabelValues(errorType, component).Inc()
}
