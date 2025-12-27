package monitoring

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
)

// Metrics holds all Prometheus metrics
type Metrics struct {
	// Routing metrics
	routingDecisions prometheus.CounterVec
	headRegistrations prometheus.Counter
	headStatusUpdates prometheus.Counter
	activeHeads prometheus.Gauge

	// HTTP metrics
	httpRequests prometheus.CounterVec
	httpRequestDuration prometheus.HistogramVec

	// Cache metrics
	cacheHits prometheus.Counter
	cacheMisses prometheus.Counter

	// External service metrics
	externalServiceCalls prometheus.CounterVec

	// Message queue metrics
	messageQueueMessages prometheus.CounterVec

	// Real-time connection metrics
	sseConnections prometheus.Gauge
	websocketConnections prometheus.Gauge

	// Circuit breaker metrics
	circuitBreakerFailures prometheus.CounterVec
	circuitBreakerSuccesses prometheus.CounterVec
}

// NewMetrics creates new metrics collector
func NewMetrics() *Metrics {
	return &Metrics{
		routingDecisions: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "routing_decisions_total",
				Help: "Total number of routing decisions made",
			},
			[]string{"strategy", "model_type", "region"},
		),
		headRegistrations: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "head_registrations_total",
				Help: "Total number of head registrations",
			},
		),
		headStatusUpdates: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "head_status_updates_total",
				Help: "Total number of head status updates",
			},
		),
		activeHeads: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "active_heads",
				Help: "Number of active heads",
			},
		),
		httpRequests: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"method", "endpoint", "status"},
		),
		httpRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "HTTP request latency distribution",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "endpoint"},
		),
		cacheHits: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "cache_hits_total",
				Help: "Total number of cache hits",
			},
		),
		cacheMisses: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "cache_misses_total",
				Help: "Total number of cache misses",
			},
		),
		externalServiceCalls: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "external_service_calls_total",
				Help: "Total number of external service calls",
			},
			[]string{"service", "status"},
		),
		messageQueueMessages: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "message_queue_messages_total",
				Help: "Total number of message queue messages",
			},
			[]string{"queue", "status"},
		),
		sseConnections: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "sse_connections",
				Help: "Number of active SSE connections",
			},
		),
		websocketConnections: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "websocket_connections",
				Help: "Number of active WebSocket connections",
			},
		),
		circuitBreakerFailures: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "circuit_breaker_failures_total",
				Help: "Total number of circuit breaker failures",
			},
			[]string{"service"},
		),
		circuitBreakerSuccesses: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "circuit_breaker_successes_total",
				Help: "Total number of circuit breaker successes",
			},
			[]string{"service"},
		),
	}
}

// Register registers all metrics with Prometheus
func (m *Metrics) Register() {
	prometheus.MustRegister(
		m.routingDecisions,
		m.headRegistrations,
		m.headStatusUpdates,
		m.activeHeads,
		m.httpRequests,
		m.httpRequestDuration,
		m.cacheHits,
		m.cacheMisses,
		m.externalServiceCalls,
		m.messageQueueMessages,
		m.sseConnections,
		m.websocketConnections,
		m.circuitBreakerFailures,
		m.circuitBreakerSuccesses,
	)
}

// IncRoutingDecision increments routing decision counter
func (m *Metrics) IncRoutingDecision(strategy, modelType, region string) {
	m.routingDecisions.WithLabelValues(strategy, modelType, region).Inc()
}

// IncHeadRegistration increments head registration counter
func (m *Metrics) IncHeadRegistration() {
	m.headRegistrations.Inc()
}

// IncHeadStatusUpdate increments head status update counter
func (m *Metrics) IncHeadStatusUpdate() {
	m.headStatusUpdates.Inc()
}

// SetActiveHeads sets the number of active heads
func (m *Metrics) SetActiveHeads(count float64) {
	m.activeHeads.Set(count)
}

// IncHTTPRequest increments HTTP request counter
func (m *Metrics) IncHTTPRequest(method, endpoint, status string) {
	m.httpRequests.WithLabelValues(method, endpoint, status).Inc()
}

// ObserveHTTPRequestDuration observes HTTP request duration
func (m *Metrics) ObserveHTTPRequestDuration(method, endpoint string, duration float64) {
	m.httpRequestDuration.WithLabelValues(method, endpoint).Observe(duration)
}

// IncCacheHit increments cache hit counter
func (m *Metrics) IncCacheHit() {
	m.cacheHits.Inc()
}

// IncCacheMiss increments cache miss counter
func (m *Metrics) IncCacheMiss() {
	m.cacheMisses.Inc()
}

// IncExternalServiceCall increments external service call counter
func (m *Metrics) IncExternalServiceCall(service, status string) {
	m.externalServiceCalls.WithLabelValues(service, status).Inc()
}

// IncMessageQueueMessage increments message queue message counter
func (m *Metrics) IncMessageQueueMessage(queue, status string) {
	m.messageQueueMessages.WithLabelValues(queue, status).Inc()
}

// SetSSEConnections sets the number of SSE connections
func (m *Metrics) SetSSEConnections(count float64) {
	m.sseConnections.Set(count)
}

// SetWebSocketConnections sets the number of WebSocket connections
func (m *Metrics) SetWebSocketConnections(count float64) {
	m.websocketConnections.Set(count)
}

// IncCircuitBreakerFailure increments circuit breaker failure counter
func (m *Metrics) IncCircuitBreakerFailure(service string) {
	m.circuitBreakerFailures.WithLabelValues(service).Inc()
}

// IncCircuitBreakerSuccess increments circuit breaker success counter
func (m *Metrics) IncCircuitBreakerSuccess(service string) {
	m.circuitBreakerSuccesses.WithLabelValues(service).Inc()
}

// MetricsHandler returns HTTP handler for Prometheus metrics
func (m *Metrics) MetricsHandler() http.Handler {
	return promhttp.Handler()
}