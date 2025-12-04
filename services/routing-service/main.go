



package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/go-redis/redis/v8"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/nats-io/nats.go"
	"github.com/graph-gophers/graphql-go"
	"github.com/graph-gophers/graphql-go/relay"
	"github.com/sonh/phony"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	pb "github.com/MaksimVF/ZB/gen/proto"
)

var (
	redisClient   *redis.Client
	logger        *zap.Logger
	httpServer    *http.Server
	grpcServer    *grpc.Server
	natsConn      *nats.Conn
	headServices  = make(map[string]HeadService)
	routingPolicy RoutingPolicy
	configMutex   sync.RWMutex

	// Performance optimization
	routingCache = make(map[string]string) // Cache for routing decisions
	cacheMutex   sync.RWMutex

	// External service integration
	externalServiceClient *http.Client

	// SSE and WebSocket clients
	headStatusClients        = make([]chan string, 0)
	routingDecisionClients   = make([]chan string, 0)
	clientsMutex             sync.Mutex

	// WebSocket upgrader
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all origins for now
		},
	}

	// Prometheus metrics
	routingDecisions = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "routing_decisions_total",
			Help: "Total number of routing decisions made",
		},
		[]string{"strategy", "model_type", "region"},
	)

	headRegistrations = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "head_registrations_total",
			Help: "Total number of head registrations",
		},
	)

	headStatusUpdates = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "head_status_updates_total",
			Help: "Total number of head status updates",
		},
	)

	activeHeads = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "active_heads",
			Help: "Number of active heads",
		},
	)

	httpRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status"},
	)

	// Cache performance metrics
	cacheHits = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "cache_hits_total",
			Help: "Total number of cache hits",
		},
	)

	cacheMisses = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "cache_misses_total",
			Help: "Total number of cache misses",
		},
	)

	// External service metrics
	externalServiceCalls = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "external_service_calls_total",
			Help: "Total number of external service calls",
		},
		[]string{"service", "status"},
	)

	// Message queue metrics
	messageQueueMessages = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "message_queue_messages_total",
			Help: "Total number of message queue messages",
		},
		[]string{"queue", "status"},
	)

	// SSE and WebSocket metrics
	sseConnections = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "sse_connections",
			Help: "Number of active SSE connections",
		},
	)

	websocketConnections = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "websocket_connections",
			Help: "Number of active WebSocket connections",
		},
	)
)

type HeadService struct {
	HeadID        string            `json:"head_id"`
	Endpoint      string            `json:"endpoint"`
	Status        string            `json:"status"`
	CurrentLoad   int32             `json:"current_load"`
	Region        string            `json:"region"`
	ModelType     string            `json:"model_type"`
	Version       string            `json:"version"`
	Metadata      map[string]string `json:"metadata"`
	LastHeartbeat int64             `json:"last_heartbeat"`
}

type RoutingPolicy struct {
	DefaultStrategy   string            `json:"default_strategy"`
	EnableGeoRouting  bool              `json:"enable_geo_routing"`
	EnableLoadBalancing bool            `json:"enable_load_balancing"`
	EnableModelSpecific bool            `json:"enable_model_specific"`
	StrategyConfig    map[string]string `json:"strategy_config"`
}

type RoutingServer struct {
	pb.UnimplementedRoutingServiceServer
}

func main() {
	// Initialize logger
	var err error
	logger, err = zap.NewProduction()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	// Initialize Prometheus metrics
	prometheus.MustRegister(
		routingDecisions,
		headRegistrations,
		headStatusUpdates,
		activeHeads,
		httpRequests,
		cacheHits,
		cacheMisses,
		externalServiceCalls,
		messageQueueMessages,
		sseConnections,
		websocketConnections,
	)

	// Initialize Redis client
	redisClient = redis.NewClient(&redis.Options{
		Addr: "redis:6379",
	})

	// Test Redis connection
	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		logger.Fatal("Failed to connect to Redis", zap.Error(err))
	}

	// Initialize default routing policy
	routingPolicy = RoutingPolicy{
		DefaultStrategy:   "round_robin",
		EnableGeoRouting:  true,
		EnableLoadBalancing: true,
		EnableModelSpecific: true,
		StrategyConfig:    make(map[string]string),
	}

	// Initialize external service client
	externalServiceClient = &http.Client{
		Timeout: 10 * time.Second,
	}

	// Initialize NATS connection
	var err error
	natsConn, err = nats.Connect(nats.DefaultURL)
	if err != nil {
		logger.Fatal("Failed to connect to NATS", zap.Error(err))
	}
	defer natsConn.Close()

	// Start message queue subscribers
	go startMessageQueueSubscribers()

	// Start gRPC server
	go startGRPCServer()

	// Start HTTP server
	go startHTTPServer()

	// Wait for shutdown signal
	waitForShutdown()
}

func startGRPCServer() {
	lis, err := net.Listen("tcp", ":50055")
	if err != nil {
		logger.Fatal("Failed to listen", zap.Error(err))
	}

	// Load TLS certificates
	serverCert, err := tls.LoadX509KeyPair("certs/server.crt", "certs/server.key")
	if err != nil {
		logger.Fatal("Failed to load server certificates", zap.Error(err))
	}

	// Load CA certificate
	caCert, err := os.ReadFile("certs/ca.crt")
	if err != nil {
		logger.Fatal("Failed to read CA certificate", zap.Error(err))
	}

	// Create certificate pool
	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(caCert) {
		logger.Fatal("Failed to add CA certificate to pool")
	}

	// Create TLS configuration
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:   certPool,
	}

	// Create gRPC server with TLS
	grpcServer = grpc.NewServer(
		grpc.Creds(credentials.NewTLS(tlsConfig)),
	)
	pb.RegisterRoutingServiceServer(grpcServer, &RoutingServer{})

	logger.Info("Starting gRPC server with mTLS on :50055")
	if err := grpcServer.Serve(lis); err != nil {
		logger.Fatal("gRPC server failed", zap.Error(err))
	}
}


type UserRole string

const (
	RoleAdmin    UserRole = "admin"
	RoleOperator UserRole = "operator"
	RoleViewer   UserRole = "viewer"
)

type UserContext struct {
	UserID string
	Role   UserRole
}

func jwtMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip authentication for health check
		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Missing authorization header", http.StatusUnauthorized)
			httpRequests.WithLabelValues(r.Method, r.URL.Path, "401").Inc()
			return
		}

		// In production, validate the JWT token and extract user info
		// For now, we'll simulate token validation and role extraction
		var userCtx UserContext

		// Simulate token validation
		switch authHeader {
		case "Bearer admin-token":
			userCtx = UserContext{UserID: "admin-user", Role: RoleAdmin}
		case "Bearer operator-token":
			userCtx = UserContext{UserID: "operator-user", Role: RoleOperator}
		case "Bearer viewer-token":
			userCtx = UserContext{UserID: "viewer-user", Role: RoleViewer}
		default:
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			httpRequests.WithLabelValues(r.Method, r.URL.Path, "401").Inc()
			return
		}

		// Add user context to request
		ctx := context.WithValue(r.Context(), "user", userCtx)

		// Create a response recorder to capture status code
		rec := statusRecorder{ResponseWriter: w, statusCode: 200}
		next.ServeHTTP(&rec, r.WithContext(ctx))

		// Record HTTP request metrics
		httpRequests.WithLabelValues(r.Method, r.URL.Path, fmt.Sprintf("%d", rec.statusCode)).Inc()
	})
}

// statusRecorder is a wrapper around http.ResponseWriter that captures the status code
type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

func checkRole(requiredRole UserRole) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userCtx, ok := r.Context().Value("user").(UserContext)
			if !ok || userCtx.Role != requiredRole {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func startHTTPServer() {
	router := mux.NewRouter()

	// Admin API endpoints with RBAC
	router.HandleFunc("/api/routing/policy", getRoutingPolicy).Methods("GET")
	router.Handle("/api/routing/policy", checkRole(RoleAdmin)(http.HandlerFunc(updateRoutingPolicy))).Methods("PUT")
	router.Handle("/api/routing/heads", checkRole(RoleOperator)(http.HandlerFunc(registerHeadHTTP))).Methods("POST")
	router.HandleFunc("/api/routing/heads", getAllHeads).Methods("GET")
	router.HandleFunc("/health", healthCheck).Methods("GET")

	// Webhook endpoints with rate limiting
	router.HandleFunc("/webhook/head-status", rateLimitMiddleware(handleHeadStatusWebhook)).Methods("POST")
	router.HandleFunc("/webhook/routing-decision", rateLimitMiddleware(handleRoutingDecisionWebhook)).Methods("POST")

	// Server-Sent Events (SSE) endpoints
	router.HandleFunc("/events/head-status", handleHeadStatusEvents).Methods("GET")
	router.HandleFunc("/events/routing-decisions", handleRoutingDecisionEvents).Methods("GET")

	// WebSocket endpoints
	router.HandleFunc("/ws/head-management", handleHeadManagementWebSocket)
	router.HandleFunc("/ws/routing-decisions", handleRoutingDecisionsWebSocket)

	// GraphQL endpoint
	router.Handle("/graphql", graphqlHandler()).Methods("POST")
	router.Handle("/graphiql", graphiqlHandler()).Methods("GET")

	// Serve admin interface
	router.PathPrefix("/admin/").Handler(http.StripPrefix("/admin/", http.FileServer(http.Dir("./"))))

	// Add Prometheus metrics endpoint
	router.Handle("/metrics", promhttp.Handler()).Methods("GET")

	// Apply JWT middleware
	httpServer = &http.Server{
		Addr:    ":8080",
		Handler: jwtMiddleware(router),
	}

	logger.Info("Starting HTTP server with JWT authentication, RBAC, Prometheus metrics, webhook support, SSE, WebSocket, and GraphQL on :8080")
	if err := httpServer.ListenAndServe(); err != nil && err != http.ServerClosed {
		logger.Fatal("HTTP server failed", zap.Error(err))
	}
}

func rateLimitMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get client IP
		ip := getClientIP(r)

		// Check rate limit
		if !rateLimiter.Allow(ip) {
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}

		// Call the next handler
		next.ServeHTTP(w, r)
	}
}

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

// Rate limiter implementation
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
	ipRequestCounts map[string]int
	ipLastRequests map[string]time.Time
	ipSuccessCounts map[string]int
	ipFailureCounts map[string]int
	ipRecoveryAttempts map[string]int
	ipRecoverySuccesses map[string]int
	ipRecoveryFailures map[string]int
}

var rateLimiter = &RateLimiter{
	requests:     make(map[string]int),
	lastRequest:  make(map[string]time.Time),
	threshold:    10,
	resetTimeout:  1 * time.Minute,
	burstLimit:   5,
	burstDuration: 10 * time.Second,
	ipThresholds:  make(map[string]int),
	ipBurstLimits: make(map[string]int),
	ipResetTimeouts: make(map[string]time.Duration),
	ipBurstDurations: make(map[string]time.Duration),
	ipRequestCounts: make(map[string]int),
	ipLastRequests: make(map[string]time.Time),
	ipSuccessCounts: make(map[string]int),
	ipFailureCounts: make(map[string]int),
	ipRecoveryAttempts: make(map[string]int),
	ipRecoverySuccesses: make(map[string]int),
	ipRecoveryFailures: make(map[string]int),
}

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
				// Increment failure count
				rl.ipFailureCounts[ip]++
				rl.ipRecoveryFailures[ip]++
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
					// Increment failure count
					rl.ipFailureCounts[ip]++
					rl.ipRecoveryFailures[ip]++
					return false
				}
			}
		}
	}

	// Increment request count
	rl.requests[ip]++
	rl.lastRequest[ip] = time.Now()
	rl.ipSuccessCounts[ip]++
	rl.ipRecoverySuccesses[ip]++
	return true
}

func (rl *RateLimiter) Metrics() map[string]interface{} {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	metrics := make(map[string]interface{})
	for ip, requests := range rl.requests {
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

		metrics[ip] = map[string]interface{}{
			"requests":     requests,
			"last_request": rl.lastRequest[ip],
			"threshold":    threshold,
			"burst_limit":   burstLimit,
			"reset_timeout": resetTimeout,
			"burst_duration": burstDuration,
			"success_count": rl.ipSuccessCounts[ip],
			"failure_count": rl.ipFailureCounts[ip],
			"recovery_attempts": rl.ipRecoveryAttempts[ip],
			"recovery_successes": rl.ipRecoverySuccesses[ip],
			"recovery_failures": rl.ipRecoveryFailures[ip],
		}
	}
	return metrics
}

func (rl *RateLimiter) Reset(ip string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	delete(rl.requests, ip)
	delete(rl.lastRequest, ip)
	delete(rl.ipThresholds, ip)
	delete(rl.ipBurstLimits, ip)
	delete(rl.ipResetTimeouts, ip)
	delete(rl.ipBurstDurations, ip)
	delete(rl.ipRequestCounts, ip)
	delete(rl.ipLastRequests, ip)
	delete(rl.ipSuccessCounts, ip)
	delete(rl.ipFailureCounts, ip)
	delete(rl.ipRecoveryAttempts, ip)
	delete(rl.ipRecoverySuccesses, ip)
	delete(rl.ipRecoveryFailures, ip)
}

func (rl *RateLimiter) SetThreshold(ip string, threshold int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Set custom threshold for specific IP
	rl.ipThresholds[ip] = threshold
	logger.Info("Setting custom rate limit threshold", zap.String("ip", ip), zap.Int("threshold", threshold))
}

func (rl *RateLimiter) SetBurstLimit(ip string, burstLimit int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Set custom burst limit for specific IP
	rl.ipBurstLimits[ip] = burstLimit
	logger.Info("Setting custom rate limit burst limit", zap.String("ip", ip), zap.Int("burst_limit", burstLimit))
}

func (rl *RateLimiter) SetResetTimeout(ip string, resetTimeout time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Set custom reset timeout for specific IP
	rl.ipResetTimeouts[ip] = resetTimeout
	logger.Info("Setting custom rate limit reset timeout", zap.String("ip", ip), zap.Duration("reset_timeout", resetTimeout))
}

func (rl *RateLimiter) SetBurstDuration(ip string, burstDuration time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Set custom burst duration for specific IP
	rl.ipBurstDurations[ip] = burstDuration
	logger.Info("Setting custom rate limit burst duration", zap.String("ip", ip), zap.Duration("burst_duration", burstDuration))
}

func (rl *RateLimiter) SetGlobalThreshold(threshold int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Set global threshold
	rl.threshold = threshold
	logger.Info("Setting global rate limit threshold", zap.Int("threshold", threshold))
}

func (rl *RateLimiter) SetGlobalBurstLimit(burstLimit int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Set global burst limit
	rl.burstLimit = burstLimit
	logger.Info("Setting global rate limit burst limit", zap.Int("burst_limit", burstLimit))
}

func (rl *RateLimiter) SetGlobalResetTimeout(resetTimeout time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Set global reset timeout
	rl.resetTimeout = resetTimeout
	logger.Info("Setting global rate limit reset timeout", zap.Duration("reset_timeout", resetTimeout))
}

func (rl *RateLimiter) SetGlobalBurstDuration(burstDuration time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Set global burst duration
	rl.burstDuration = burstDuration
	logger.Info("Setting global rate limit burst duration", zap.Duration("burst_duration", burstDuration))
}

func (rl *RateLimiter) SetGlobalSuccessCount(ip string, successCount int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Set global success count
	rl.ipSuccessCounts[ip] = successCount
	logger.Info("Setting global rate limit success count", zap.String("ip", ip), zap.Int("success_count", successCount))
}

func (rl *RateLimiter) SetGlobalFailureCount(ip string, failureCount int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Set global failure count
	rl.ipFailureCounts[ip] = failureCount
	logger.Info("Setting global rate limit failure count", zap.String("ip", ip), zap.Int("failure_count", failureCount))
}

func (rl *RateLimiter) SetGlobalRecoveryAttempts(ip string, recoveryAttempts int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Set global recovery attempts
	rl.ipRecoveryAttempts[ip] = recoveryAttempts
	logger.Info("Setting global rate limit recovery attempts", zap.String("ip", ip), zap.Int("recovery_attempts", recoveryAttempts))
}

func (rl *RateLimiter) SetGlobalRecoverySuccesses(ip string, recoverySuccesses int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Set global recovery successes
	rl.ipRecoverySuccesses[ip] = recoverySuccesses
	logger.Info("Setting global rate limit recovery successes", zap.String("ip", ip), zap.Int("recovery_successes", recoverySuccesses))
}

func (rl *RateLimiter) SetGlobalRecoveryFailures(ip string, recoveryFailures int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Set global recovery failures
	rl.ipRecoveryFailures[ip] = recoveryFailures
	logger.Info("Setting global rate limit recovery failures", zap.String("ip", ip), zap.Int("recovery_failures", recoveryFailures))
}

func graphqlHandler() http.Handler {
	// Define GraphQL schema
	schema := `
	type Head {
		id: ID!
		endpoint: String!
		modelType: String!
		region: String!
		status: String!
		currentLoad: Int!
		lastHeartbeat: String!
		metadata: JSON
		createdAt: String!
		updatedAt: String!
		uptime: Float!
		availability: Float!
		latency: Float!
		errorRate: Float!
		successRate: Float!
		throughput: Float!
		retryCount: Int!
		recoveryTime: Float!
		recoveryAttempts: Int!
		recoverySuccesses: Int!
		recoveryFailures: Int!
		recoveryLatency: Float!
		recoveryThroughput: Float!
	}

	type RoutingDecision {
		headId: String!
		endpoint: String!
		strategyUsed: String!
		reason: String!
		metadata: JSON
		timestamp: String!
		processingTime: Float!
		successRate: Float!
		errorCount: Int!
		latency: Float!
		retryCount: Int!
		recoveryTime: Float!
		recoveryAttempts: Int!
		recoverySuccesses: Int!
		recoveryFailures: Int!
		recoveryLatency: Float!
		recoveryThroughput: Float!
	}

	type Query {
		heads: [Head!]!
		head(id: ID!): Head
		routingDecision(modelType: String!, regionPreference: String, strategy: String): RoutingDecision!
		routingPolicy: RoutingPolicy!
		headStatusHistory(headId: ID!, limit: Int): [HeadStatus!]!
		routingDecisionsHistory(modelType: String!, limit: Int): [RoutingDecision!]!
		systemHealth: SystemHealth!
		circuitBreakerStatus: CircuitBreakerStatus!
		rateLimiterStatus: RateLimiterStatus!
		externalServiceMetrics: [ExternalServiceMetric!]!
		headPerformanceMetrics: [HeadPerformanceMetric!]!
		headStatusChanges: [HeadStatusChange!]!
		routingDecisionChanges: [RoutingDecisionChange!]!
		headRecoveryMetrics: [HeadRecoveryMetric!]!
		routingDecisionRecoveryMetrics: [RoutingDecisionRecoveryMetric!]!
	}

	type Mutation {
		registerHead(input: RegisterHeadInput!): Head!
		updateHeadStatus(id: ID!, status: String!, currentLoad: Int!): Head!
		deregisterHead(id: ID!): Boolean!
		updateRoutingPolicy(input: UpdateRoutingPolicyInput!): RoutingPolicy!
		resetCircuitBreaker(service: String!): Boolean!
		resetRateLimiter(ip: String!): Boolean!
		setRateLimitThreshold(ip: String!, threshold: Int!): Boolean!
		setRateLimitBurstLimit(ip: String!, burstLimit: Int!): Boolean!
		setCircuitBreakerThreshold(service: String!, threshold: Int!): Boolean!
		setCircuitBreakerResetTimeout(service: String!, resetTimeout: Int!): Boolean!
		setRateLimitResetTimeout(ip: String!, resetTimeout: Int!): Boolean!
		setRateLimitBurstDuration(ip: String!, burstDuration: Int!): Boolean!
		setHeadRecoveryMetrics(headId: String!, recoveryMetrics: HeadRecoveryMetricsInput!): Boolean!
		setRoutingDecisionRecoveryMetrics(routingDecisionId: String!, recoveryMetrics: RoutingDecisionRecoveryMetricsInput!): Boolean!
	}

	input RegisterHeadInput {
		endpoint: String!
		modelType: String!
		region: String!
		status: String!
		metadata: JSON
	}

	input UpdateRoutingPolicyInput {
		defaultStrategy: String
		enableGeoRouting: Boolean
		enableLoadBalancing: Boolean
		enableModelSpecific: Boolean
		strategyConfig: JSON
	}

	input HeadRecoveryMetricsInput {
		recoveryTime: Float!
		recoveryAttempts: Int!
		recoverySuccesses: Int!
		recoveryFailures: Int!
		recoveryLatency: Float!
		recoveryThroughput: Float!
	}

	input RoutingDecisionRecoveryMetricsInput {
		recoveryTime: Float!
		recoveryAttempts: Int!
		recoverySuccesses: Int!
		recoveryFailures: Int!
		recoveryLatency: Float!
		recoveryThroughput: Float!
	}

	type RoutingPolicy {
		defaultStrategy: String!
		enableGeoRouting: Boolean!
		enableLoadBalancing: Boolean!
		enableModelSpecific: Boolean!
		strategyConfig: JSON
		lastUpdated: String!
		updateCount: Int!
		policyVersion: String!
		policyStatus: String!
		policyMetrics: PolicyMetrics!
		policyRecoveryMetrics: PolicyRecoveryMetrics!
	}

	type HeadStatus {
		headId: String!
		status: String!
		currentLoad: Int!
		timestamp: String!
		previousStatus: String
		changeDuration: Float!
		changeReason: String!
		changeInitiator: String!
		changeMetrics: HeadStatusChangeMetrics!
		changeRecoveryMetrics: HeadStatusChangeRecoveryMetrics!
	}

	type SystemHealth {
		uptime: String!
		memoryUsage: Float!
		cpuUsage: Float!
		activeConnections: Int!
		errorRate: Float!
		latency: Float!
		throughput: Float!
		successRate: Float!
		systemMetrics: SystemHealthMetrics!
		systemRecoveryMetrics: SystemRecoveryMetrics!
	}

	type CircuitBreakerStatus {
		service: String!
		state: String!
		failureCount: Int!
		lastFailure: String
		halfOpenUntil: String
		successCount: Int!
		recoveryAttempts: Int!
		circuitBreakerMetrics: CircuitBreakerMetrics!
		circuitBreakerRecoveryMetrics: CircuitBreakerRecoveryMetrics!
	}

	type RateLimiterStatus {
		ip: String!
		requestCount: Int!
		lastRequest: String!
		threshold: Int!
		burstLimit: Int!
		burstDuration: Int!
		resetTimeout: Int!
		rateLimiterMetrics: RateLimiterMetrics!
		rateLimiterRecoveryMetrics: RateLimiterRecoveryMetrics!
	}

	type ExternalServiceMetric {
		service: String!
		successRate: Float!
		errorRate: Float!
		latency: Float!
		throughput: Float!
		availability: Float!
		externalServiceMetrics: ExternalServiceMetrics!
		externalServiceRecoveryMetrics: ExternalServiceRecoveryMetrics!
	}

	type HeadPerformanceMetric {
		headId: String!
		latency: Float!
		throughput: Float!
		errorRate: Float!
		successRate: Float!
		availability: Float!
		headPerformanceMetrics: HeadPerformanceMetrics!
		headPerformanceRecoveryMetrics: HeadPerformanceRecoveryMetrics!
	}

	type HeadStatusChange {
		headId: String!
		fromStatus: String!
		toStatus: String!
		timestamp: String!
		changeDuration: Float!
		changeReason: String!
		changeInitiator: String!
		changeMetrics: HeadStatusChangeMetrics!
		changeRecoveryMetrics: HeadStatusChangeRecoveryMetrics!
	}

	type RoutingDecisionChange {
		headId: String!
		fromStrategy: String!
		toStrategy: String!
		timestamp: String!
		changeDuration: Float!
		changeReason: String!
		changeInitiator: String!
		changeMetrics: RoutingDecisionChangeMetrics!
		changeRecoveryMetrics: RoutingDecisionChangeRecoveryMetrics!
	}

	type PolicyMetrics {
		policyChanges: Int!
		policyErrors: Int!
		policySuccessRate: Float!
		policyLatency: Float!
		policyRecoveryMetrics: PolicyRecoveryMetrics!
	}

	type HeadStatusChangeMetrics {
		changeLatency: Float!
		changeSuccessRate: Float!
		changeErrorRate: Float!
		changeThroughput: Float!
		changeRecoveryMetrics: HeadStatusChangeRecoveryMetrics!
	}

	type RoutingDecisionChangeMetrics {
		changeLatency: Float!
		changeSuccessRate: Float!
		changeErrorRate: Float!
		changeThroughput: Float!
		changeRecoveryMetrics: RoutingDecisionChangeRecoveryMetrics!
	}

	type SystemHealthMetrics {
		systemLatency: Float!
		systemSuccessRate: Float!
		systemErrorRate: Float!
		systemThroughput: Float!
		systemRecoveryMetrics: SystemRecoveryMetrics!
	}

	type CircuitBreakerMetrics {
		circuitBreakerLatency: Float!
		circuitBreakerSuccessRate: Float!
		circuitBreakerErrorRate: Float!
		circuitBreakerThroughput: Float!
		circuitBreakerRecoveryMetrics: CircuitBreakerRecoveryMetrics!
	}

	type RateLimiterMetrics {
		rateLimiterLatency: Float!
		rateLimiterSuccessRate: Float!
		rateLimiterErrorRate: Float!
		rateLimiterThroughput: Float!
		rateLimiterRecoveryMetrics: RateLimiterRecoveryMetrics!
	}

	type ExternalServiceMetrics {
		externalServiceLatency: Float!
		externalServiceSuccessRate: Float!
		externalServiceErrorRate: Float!
		externalServiceThroughput: Float!
		externalServiceRecoveryMetrics: ExternalServiceRecoveryMetrics!
	}

	type HeadPerformanceMetrics {
		headPerformanceLatency: Float!
		headPerformanceSuccessRate: Float!
		headPerformanceErrorRate: Float!
		headPerformanceThroughput: Float!
		headPerformanceRecoveryMetrics: HeadPerformanceRecoveryMetrics!
	}

	type PolicyRecoveryMetrics {
		policyRecoveryTime: Float!
		policyRecoveryAttempts: Int!
		policyRecoverySuccesses: Int!
		policyRecoveryFailures: Int!
		policyRecoveryLatency: Float!
		policyRecoveryThroughput: Float!
	}

	type HeadStatusChangeRecoveryMetrics {
		changeRecoveryTime: Float!
		changeRecoveryAttempts: Int!
		changeRecoverySuccesses: Int!
		changeRecoveryFailures: Int!
		changeRecoveryLatency: Float!
		changeRecoveryThroughput: Float!
	}

	type RoutingDecisionChangeRecoveryMetrics {
		changeRecoveryTime: Float!
		changeRecoveryAttempts: Int!
		changeRecoverySuccesses: Int!
		changeRecoveryFailures: Int!
		changeRecoveryLatency: Float!
		changeRecoveryThroughput: Float!
	}

	type SystemRecoveryMetrics {
		systemRecoveryTime: Float!
		systemRecoveryAttempts: Int!
		systemRecoverySuccesses: Int!
		systemRecoveryFailures: Int!
		systemRecoveryLatency: Float!
		systemRecoveryThroughput: Float!
	}

	type CircuitBreakerRecoveryMetrics {
		circuitBreakerRecoveryTime: Float!
		circuitBreakerRecoveryAttempts: Int!
		circuitBreakerRecoverySuccesses: Int!
		circuitBreakerRecoveryFailures: Int!
		circuitBreakerRecoveryLatency: Float!
		circuitBreakerRecoveryThroughput: Float!
	}

	type RateLimiterRecoveryMetrics {
		rateLimiterRecoveryTime: Float!
		rateLimiterRecoveryAttempts: Int!
		rateLimiterRecoverySuccesses: Int!
		rateLimiterRecoveryFailures: Int!
		rateLimiterRecoveryLatency: Float!
		rateLimiterRecoveryThroughput: Float!
	}

	type ExternalServiceRecoveryMetrics {
		externalServiceRecoveryTime: Float!
		externalServiceRecoveryAttempts: Int!
		externalServiceRecoverySuccesses: Int!
		externalServiceRecoveryFailures: Int!
		externalServiceRecoveryLatency: Float!
		externalServiceRecoveryThroughput: Float!
	}

	type HeadPerformanceRecoveryMetrics {
		headPerformanceRecoveryTime: Float!
		headPerformanceRecoveryAttempts: Int!
		headPerformanceRecoverySuccesses: Int!
		headPerformanceRecoveryFailures: Int!
		headPerformanceRecoveryLatency: Float!
		headPerformanceRecoveryThroughput: Float!
	}

	type HeadRecoveryMetric {
		headId: String!
		recoveryTime: Float!
		recoveryAttempts: Int!
		recoverySuccesses: Int!
		recoveryFailures: Int!
		recoveryLatency: Float!
		recoveryThroughput: Float!
	}

	type RoutingDecisionRecoveryMetric {
		routingDecisionId: String!
		recoveryTime: Float!
		recoveryAttempts: Int!
		recoverySuccesses: Int!
		recoveryFailures: Int!
		recoveryLatency: Float!
		recoveryThroughput: Float!
	}

	scalar JSON
	`

	// Create GraphQL resolver
	resolver := &graphql.Resolver{
		Schema: graphql.MustParseSchema(schema, &graphql.ResolverConfig{
			Query: &QueryResolver{},
			Mutation: &MutationResolver{},
		}),
	}

	return &relay.Handler{Resolver: resolver}
}

type QueryResolver struct{}

func (r *QueryResolver) Heads(ctx context.Context) ([]*HeadService, error) {
	configMutex.RLock()
	defer configMutex.RUnlock()

	heads := make([]*HeadService, 0, len(headServices))
	for _, head := range headServices {
		heads = append(heads, &head)
	}
	return heads, nil
}

func (r *QueryResolver) Head(ctx context.Context, args struct{ ID string }) (*HeadService, error) {
	configMutex.RLock()
	defer configMutex.RUnlock()

	head, exists := headServices[args.ID]
	if !exists {
		return nil, fmt.Errorf("head not found")
	}
	return &head, nil
}

func (r *QueryResolver) RoutingDecision(ctx context.Context, args struct {
	ModelType       string
	RegionPreference string
	Strategy        string
}) (*pb.GetRoutingDecisionResponse, error) {
	req := &pb.GetRoutingDecisionRequest{
		ModelType:       args.ModelType,
		RegionPreference: args.RegionPreference,
		RoutingStrategy: args.Strategy,
	}

	return (&RoutingServer{}).GetRoutingDecision(ctx, req)
}

func (r *QueryResolver) RoutingPolicy(ctx context.Context) (*RoutingPolicy, error) {
	configMutex.RLock()
	defer configMutex.RUnlock()

	return &routingPolicy, nil
}

type MutationResolver struct{}

func (r *MutationResolver) RegisterHead(ctx context.Context, args struct {
	Input RegisterHeadInput
}) (*HeadService, error) {
	req := &pb.RegisterHeadRequest{
		HeadId:    args.Input.HeadID,
		Endpoint:  args.Input.Endpoint,
		ModelType: args.Input.ModelType,
		Region:    args.Input.Region,
		Status:    args.Input.Status,
		Metadata:  args.Input.Metadata,
	}

	resp, err := (&RoutingServer{}).RegisterHead(ctx, req)
	if err != nil {
		return nil, err
	}

	// Get the registered head
	head, exists := headServices[args.Input.HeadID]
	if !exists {
		return nil, fmt.Errorf("failed to register head")
	}
	return &head, nil
}

func (r *MutationResolver) UpdateHeadStatus(ctx context.Context, args struct {
	ID          string
	Status      string
	CurrentLoad int
}) (*HeadService, error) {
	req := &pb.UpdateHeadStatusRequest{
		HeadId:      args.ID,
		Status:      args.Status,
		CurrentLoad: int32(args.CurrentLoad),
		Timestamp:   time.Now().Unix(),
	}

	resp, err := (&RoutingServer{}).UpdateHeadStatus(ctx, req)
	if err != nil {
		return nil, err
	}

	// Get the updated head
	head, exists := headServices[args.ID]
	if !exists {
		return nil, fmt.Errorf("failed to update head")
	}
	return &head, nil
}

type RegisterHeadInput struct {
	HeadID    string            `json:"head_id"`
	Endpoint  string            `json:"endpoint"`
	ModelType string            `json:"model_type"`
	Region    string            `json:"region"`
	Status    string            `json:"status"`
	Metadata map[string]string `json:"metadata"`
}

func graphiqlHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Serve GraphiQL interface
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`
		<!DOCTYPE html>
		<html>
		<head>
			<title>GraphiQL</title>
			<link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/graphiql/0.11.11/graphiql.min.css" />
		</head>
		<body style="margin: 0;">
			<div id="graphiql" style="height: 100vh;"></div>
			<script src="https://cdnjs.cloudflare.com/ajax/libs/fetch/2.0.3/fetch.min.js"></script>
			<script src="https://cdnjs.cloudflare.com/ajax/libs/react/16.2.0/umd/react.production.min.js"></script>
			<script src="https://cdnjs.cloudflare.com/ajax/libs/react-dom/16.2.0/umd/react-dom.production.min.js"></script>
			<script src="https://cdnjs.cloudflare.com/ajax/libs/graphiql/0.11.11/graphiql.min.js"></script>
			<script>
				function graphQLFetcher(graphQLParams) {
					return fetch('/graphql', {
						method: 'post',
						headers: {
							'Content-Type': 'application/json',
						},
						body: JSON.stringify(graphQLParams),
					}).then(function (response) {
						return response.text();
					}).then(function (responseBody) {
						try {
							return JSON.parse(responseBody);
						} catch (error) {
							return responseBody;
						}
					});
				}

				ReactDOM.render(
					React.createElement(GraphiQL, {
						fetcher: graphQLFetcher,
						query: '# Welcome to GraphiQL\n# \n# Type queries into this side of the screen, and you will see intelligent\n# typeaheads aware of the current GraphQL type schema and live syntax\n# and validation errors highlighted within the text.\n# \n# Keyboard shortcuts:\n# \n#  Prettify Query:  Shift-Ctrl-P (or press the prettify button above)\n# \n#     Run Query:  Ctrl-Enter (or press the play button above)\n# \n#   Auto Complete:  Ctrl-Space (or just start typing)\n# \nquery MyQuery {\n  heads {\n    id\n    endpoint\n    modelType\n    region\n    status\n    currentLoad\n  }\n}\n',
					}),
					document.getElementById('graphiql')
				);
			</script>
		</body>
		</html>
		`))
	})
}

func waitForShutdown() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	logger.Info("Shutting down...")

	// Shutdown HTTP server
	if httpServer != nil {
		if err := httpServer.Shutdown(context.Background()); err != nil {
			logger.Error("HTTP server shutdown error", zap.Error(err))
		}
	}

	// Shutdown gRPC server
	if grpcServer != nil {
		grpcServer.GracefulStop()
	}

	// Close NATS connection
	if natsConn != nil {
		natsConn.Close()
	}

	logger.Info("Shutdown complete")
}

// SSE handlers
func handleHeadStatusEvents(w http.ResponseWriter, r *http.Request) {
	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Create a channel to send events
	eventChan := make(chan string)

	// Register the client
	clientsMutex.Lock()
	headStatusClients = append(headStatusClients, eventChan)
	clientsMutex.Unlock()

	// Remove client on disconnect
	defer func() {
		clientsMutex.Lock()
		for i, client := range headStatusClients {
			if client == eventChan {
				headStatusClients = append(headStatusClients[:i], headStatusClients[i+1:]...)
				break
			}
		}
		clientsMutex.Unlock()
	}()

	// Listen for events
	for {
		select {
		case event := <-eventChan:
			fmt.Fprintf(w, "data: %s\n\n", event)
			w.(http.Flusher).Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func handleRoutingDecisionEvents(w http.ResponseWriter, r *http.Request) {
	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Create a channel to send events
	eventChan := make(chan string)

	// Register the client
	clientsMutex.Lock()
	routingDecisionClients = append(routingDecisionClients, eventChan)
	clientsMutex.Unlock()

	// Remove client on disconnect
	defer func() {
		clientsMutex.Lock()
		for i, client := range routingDecisionClients {
			if client == eventChan {
				routingDecisionClients = append(routingDecisionClients[:i], routingDecisionClients[i+1:]...)
				break
			}
		}
		clientsMutex.Unlock()
	}()

	// Listen for events
	for {
		select {
		case event := <-eventChan:
			fmt.Fprintf(w, "data: %s\n\n", event)
			w.(http.Flusher).Flush()
		case <-r.Context().Done():
			return
		}
	}
}

// WebSocket handlers
func handleHeadManagementWebSocket(w http.ResponseWriter, r *http.Request) {
	// Upgrade connection to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Error("Failed to upgrade to WebSocket", zap.Error(err))
		return
	}
	defer conn.Close()

	// Handle WebSocket messages
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			logger.Error("WebSocket read error", zap.Error(err))
			break
		}

		// Process the message
		var request struct {
			Type    string                 `json:"type"`
			Payload map[string]interface{} `json:"payload"`
		}

		if err := json.Unmarshal(message, &request); err != nil {
			logger.Error("Failed to parse WebSocket message", zap.Error(err))
			continue
		}

		// Handle different message types
		switch request.Type {
		case "register_head":
			// Handle head registration
			handleWebSocketHeadRegistration(conn, request.Payload)
		case "update_status":
			// Handle status update
			handleWebSocketStatusUpdate(conn, request.Payload)
		case "get_heads":
			// Handle get heads request
			handleWebSocketGetHeads(conn)
		default:
			// Unknown message type
			response := map[string]interface{}{
				"type":    "error",
				"message": "Unknown message type",
			}
			conn.WriteJSON(response)
		}
	}
}

func handleRoutingDecisionsWebSocket(w http.ResponseWriter, r *http.Request) {
	// Upgrade connection to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Error("Failed to upgrade to WebSocket", zap.Error(err))
		return
	}
	defer conn.Close()

	// Handle WebSocket messages
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			logger.Error("WebSocket read error", zap.Error(err))
			break
		}

		// Process the message
		var request struct {
			Type    string                 `json:"type"`
			Payload map[string]interface{} `json:"payload"`
		}

		if err := json.Unmarshal(message, &request); err != nil {
			logger.Error("Failed to parse WebSocket message", zap.Error(err))
			continue
		}

		// Handle different message types
		switch request.Type {
		case "get_routing_decision":
			// Handle routing decision request
			handleWebSocketRoutingDecision(conn, request.Payload)
		case "get_routing_strategies":
			// Handle get routing strategies request
			handleWebSocketGetRoutingStrategies(conn)
		default:
			// Unknown message type
			response := map[string]interface{}{
				"type":    "error",
				"message": "Unknown message type",
			}
			conn.WriteJSON(response)
		}
	}
}

func handleWebSocketHeadRegistration(conn *websocket.Conn, payload map[string]interface{}) {
	// Convert payload to RegisterHeadRequest
	req := &pb.RegisterHeadRequest{
		HeadId:    payload["head_id"].(string),
		Endpoint:  payload["endpoint"].(string),
		ModelType: payload["model_type"].(string),
		Region:    payload["region"].(string),
		Status:    payload["status"].(string),
		Metadata:  make(map[string]string),
	}

	// Convert metadata
	if metadata, ok := payload["metadata"].(map[string]interface{}); ok {
		for k, v := range metadata {
			req.Metadata[k] = v.(string)
		}
	}

	// Register the head
	resp, err := (&RoutingServer{}).RegisterHead(context.Background(), req)
	if err != nil {
		response := map[string]interface{}{
			"type":    "error",
			"message": err.Error(),
		}
		conn.WriteJSON(response)
		return
	}

	// Send success response
	response := map[string]interface{}{
		"type":    "register_head_response",
		"success": resp.Success,
		"message": resp.Message,
	}
	conn.WriteJSON(response)
}

func handleWebSocketStatusUpdate(conn *websocket.Conn, payload map[string]interface{}) {
	// Convert payload to UpdateHeadStatusRequest
	req := &pb.UpdateHeadStatusRequest{
		HeadId:      payload["head_id"].(string),
		Status:      payload["status"].(string),
		CurrentLoad: int32(payload["current_load"].(float64)),
		Timestamp:   int64(payload["timestamp"].(float64)),
	}

	// Update the head status
	resp, err := (&RoutingServer{}).UpdateHeadStatus(context.Background(), req)
	if err != nil {
		response := map[string]interface{}{
			"type":    "error",
			"message": err.Error(),
		}
		conn.WriteJSON(response)
		return
	}

	// Send success response
	response := map[string]interface{}{
		"type":    "update_status_response",
		"success": resp.Success,
		"message": resp.Message,
	}
	conn.WriteJSON(response)
}

func handleWebSocketGetHeads(conn *websocket.Conn) {
	// Get all heads
	resp, err := (&RoutingServer{}).GetAllHeads(context.Background(), &pb.GetAllHeadsRequest{})
	if err != nil {
		response := map[string]interface{}{
			"type":    "error",
			"message": err.Error(),
		}
		conn.WriteJSON(response)
		return
	}

	// Send success response
	response := map[string]interface{}{
		"type":  "get_heads_response",
		"heads":  resp.Heads,
	}
	conn.WriteJSON(response)
}

func handleWebSocketRoutingDecision(conn *websocket.Conn, payload map[string]interface{}) {
	// Convert payload to GetRoutingDecisionRequest
	req := &pb.GetRoutingDecisionRequest{
		ModelType:       payload["model_type"].(string),
		RegionPreference: payload["region_preference"].(string),
		RoutingStrategy: payload["routing_strategy"].(string),
		Metadata:        make(map[string]string),
	}

	// Convert metadata
	if metadata, ok := payload["metadata"].(map[string]interface{}); ok {
		for k, v := range metadata {
			req.Metadata[k] = v.(string)
		}
	}

	// Get routing decision
	resp, err := (&RoutingServer{}).GetRoutingDecision(context.Background(), req)
	if err != nil {
		response := map[string]interface{}{
			"type":    "error",
			"message": err.Error(),
		}
		conn.WriteJSON(response)
		return
	}

	// Send success response
	response := map[string]interface{}{
		"type":           "routing_decision_response",
		"head_id":         resp.HeadId,
		"endpoint":       resp.Endpoint,
		"strategy_used":    resp.StrategyUsed,
		"reason":          resp.Reason,
		"metadata":        resp.Metadata,
	}
	conn.WriteJSON(response)
}

func handleWebSocketGetRoutingStrategies(conn *websocket.Conn) {
	// Get routing policy
	configMutex.RLock()
	defer configMutex.RUnlock()

	// Send success response
	response := map[string]interface{}{
		"type":             "get_routing_strategies_response",
		"default_strategy":  routingPolicy.DefaultStrategy,
		"enable_geo_routing": routingPolicy.EnableGeoRouting,
		"enable_load_balancing": routingPolicy.EnableLoadBalancing,
		"enable_model_specific": routingPolicy.EnableModelSpecific,
		"strategy_config": routingPolicy.StrategyConfig,
	}
	conn.WriteJSON(response)
}

func startMessageQueueSubscribers() {
	// Subscribe to head status updates
	natsConn.Subscribe("head.status.update", func(msg *nats.Msg) {
		var statusUpdate struct {
			HeadID     string `json:"head_id"`
			Status     string `json:"status"`
			CurrentLoad int32  `json:"current_load"`
			Timestamp  int64  `json:"timestamp"`
		}

		if err := json.Unmarshal(msg.Data, &statusUpdate); err != nil {
			messageQueueMessages.WithLabelValues("head.status.update", "error").Inc()
			return
		}

		// Process the status update
		err := processHeadStatusUpdate(statusUpdate.HeadID, statusUpdate.Status, statusUpdate.CurrentLoad, statusUpdate.Timestamp)
		if err != nil {
			messageQueueMessages.WithLabelValues("head.status.update", "error").Inc()
			return
		}

		messageQueueMessages.WithLabelValues("head.status.update", "success").Inc()
	})

	// Subscribe to routing decision requests
	natsConn.Subscribe("routing.decision.request", func(msg *nats.Msg) {
		var decisionRequest struct {
			ModelType       string            `json:"model_type"`
			RegionPreference string            `json:"region_preference"`
			RoutingStrategy  string            `json:"routing_strategy"`
			Metadata        map[string]string `json:"metadata"`
		}

		if err := json.Unmarshal(msg.Data, &decisionRequest); err != nil {
			messageQueueMessages.WithLabelValues("routing.decision.request", "error").Inc()
			return
		}

		// Process the routing decision request
		decision, err := makeRoutingDecisionFromWebhook(decisionRequest.ModelType, decisionRequest.RegionPreference, decisionRequest.RoutingStrategy, decisionRequest.Metadata)
		if err != nil {
			messageQueueMessages.WithLabelValues("routing.decision.request", "error").Inc()
			return
		}

		// Publish the decision response
		responseData, err := json.Marshal(decision)
		if err != nil {
			messageQueueMessages.WithLabelValues("routing.decision.response", "error").Inc()
			return
		}

		natsConn.Publish("routing.decision.response", responseData)
		messageQueueMessages.WithLabelValues("routing.decision.request", "success").Inc()
	})

	// Subscribe to head registration requests
	natsConn.Subscribe("head.registration.request", func(msg *nats.Msg) {
		var registrationRequest struct {
			HeadID    string            `json:"head_id"`
			Endpoint string            `json:"endpoint"`
			ModelType string            `json:"model_type"`
			Region    string            `json:"region"`
			Status    string            `json:"status"`
			Metadata map[string]string `json:"metadata"`
		}

		if err := json.Unmarshal(msg.Data, &registrationRequest); err != nil {
			messageQueueMessages.WithLabelValues("head.registration.request", "error").Inc()
			return
		}

		// Process the registration request
		_, err := (&RoutingServer{}).RegisterHead(context.Background(), &pb.RegisterHeadRequest{
			HeadId:    registrationRequest.HeadID,
			Endpoint:  registrationRequest.Endpoint,
			ModelType: registrationRequest.ModelType,
			Region:    registrationRequest.Region,
			Status:    registrationRequest.Status,
			Metadata:  registrationRequest.Metadata,
		})
		if err != nil {
			messageQueueMessages.WithLabelValues("head.registration.request", "error").Inc()
			return
		}

		messageQueueMessages.WithLabelValues("head.registration.request", "success").Inc()
	})
}

// Webhook handlers
func handleHeadStatusWebhook(w http.ResponseWriter, r *http.Request) {
	var webhookData struct {
		HeadID     string `json:"head_id"`
		Status     string `json:"status"`
		CurrentLoad int32  `json:"current_load"`
		Timestamp  int64  `json:"timestamp"`
	}

	if err := json.NewDecoder(r.Body).Decode(&webhookData); err != nil {
		http.Error(w, "Invalid webhook payload", http.StatusBadRequest)
		return
	}

	// Process the webhook
	err := processHeadStatusUpdate(webhookData.HeadID, webhookData.Status, webhookData.CurrentLoad, webhookData.Timestamp)
	if err != nil {
		http.Error(w, "Failed to process webhook", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Webhook processed successfully")
}

func handleRoutingDecisionWebhook(w http.ResponseWriter, r *http.Request) {
	var webhookData struct {
		ModelType       string            `json:"model_type"`
		RegionPreference string            `json:"region_preference"`
		RoutingStrategy  string            `json:"routing_strategy"`
		Metadata        map[string]string `json:"metadata"`
	}

	if err := json.NewDecoder(r.Body).Decode(&webhookData); err != nil {
		http.Error(w, "Invalid webhook payload", http.StatusBadRequest)
		return
	}

	// Process the routing decision request
	decision, err := makeRoutingDecisionFromWebhook(webhookData.ModelType, webhookData.RegionPreference, webhookData.RoutingStrategy, webhookData.Metadata)
	if err != nil {
		http.Error(w, "Failed to make routing decision", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(decision)
}

func processHeadStatusUpdate(headID, status string, currentLoad int32, timestamp int64) error {
	// Update head status in the system
	_, err := (&RoutingServer{}).UpdateHeadStatus(context.Background(), &pb.UpdateHeadStatusRequest{
		HeadId:      headID,
		Status:      status,
		CurrentLoad: currentLoad,
		Timestamp:   timestamp,
	})
	return err
}

func makeRoutingDecisionFromWebhook(modelType, regionPreference, routingStrategy string, metadata map[string]string) (map[string]interface{}, error) {
	// Make a routing decision based on webhook data
	resp, err := (&RoutingServer{}).GetRoutingDecision(context.Background(), &pb.GetRoutingDecisionRequest{
		ModelType:       modelType,
		RegionPreference: regionPreference,
		RoutingStrategy: routingStrategy,
		Metadata:        metadata,
	})
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"head_id":       resp.HeadId,
		"endpoint":      resp.Endpoint,
		"strategy_used": resp.StrategyUsed,
		"reason":        resp.Reason,
		"metadata":      resp.Metadata,
	}, nil

// gRPC Methods

func (s *RoutingServer) RegisterHead(ctx context.Context, req *pb.RegisterHeadRequest) (*pb.RegisterHeadResponse, error) {
	configMutex.Lock()
	defer configMutex.Unlock()

	head := HeadService{
		HeadID:      req.HeadId,
		Endpoint:    req.Endpoint,
		Status:      "active",
		Region:      req.Region,
		ModelType:   req.ModelType,
		Version:     req.Version,
		Metadata:    req.Metadata,
		LastHeartbeat: time.Now().Unix(),
	}

	headServices[req.HeadId] = head

	// Store in Redis
	err := storeHeadInRedis(head)
	if err != nil {
		return &pb.RegisterHeadResponse{
			Success: false,
			Message: "Failed to store head in Redis",
		}, err
	}

	// Record metrics
	headRegistrations.Inc()
	activeHeads.Inc()

	return &pb.RegisterHeadResponse{
		Success: true,
		Message: "Head registered successfully",
	}, nil
}

func (s *RoutingServer) UpdateHeadStatus(ctx context.Context, req *pb.UpdateHeadStatusRequest) (*pb.UpdateHeadStatusResponse, error) {
	configMutex.Lock()
	defer configMutex.Unlock()

	head, exists := headServices[req.HeadId]
	if !exists {
		return &pb.UpdateHeadStatusResponse{
			Success: false,
			Message: "Head not found",
		}, nil
	}

	head.Status = req.Status
	head.CurrentLoad = req.CurrentLoad
	head.LastHeartbeat = req.Timestamp
	headServices[req.HeadId] = head

	// Update in Redis
	err := updateHeadStatusInRedis(head)
	if err != nil {
		return &pb.UpdateHeadStatusResponse{
			Success: false,
			Message: "Failed to update head in Redis",
		}, err
	}

	// Invalidate cache if head becomes inactive
	if head.Status != "active" {
		// Clear cache entries that might reference this head
		cacheMutex.Lock()
		for key, headID := range routingCache {
			if headID == head.HeadID {
				delete(routingCache, key)
			}
		}
		cacheMutex.Unlock()
	}

	// Record metrics
	headStatusUpdates.Inc()

	return &pb.UpdateHeadStatusResponse{Success: true, Message: "Head status updated successfully"}, nil
}

func (s *RoutingServer) GetRoutingDecision(ctx context.Context, req *pb.GetRoutingDecisionRequest) (*pb.GetRoutingDecisionResponse, error) {

	// Implement routing decision logic based on current policy
	// This is a simplified version - in production this would be more sophisticated

	// Check cache first
	cacheKey := fmt.Sprintf("%s-%s-%s-%s", req.ModelType, req.RegionPreference, req.RoutingStrategy, req.Metadata["model"])
	cacheMutex.RLock()
	cachedHeadID, found := routingCache[cacheKey]
	cacheMutex.RUnlock()

	if found {
		// Cache hit
		cacheHits.Inc()

		// Find the cached head in our current list
		configMutex.RLock()
		for _, head := range headServices {
			if head.HeadID == cachedHeadID && head.Status == "active" {
				configMutex.RUnlock()
				return &pb.GetRoutingDecisionResponse{
					HeadId:      head.HeadID,
					Endpoint:    head.Endpoint,
					StrategyUsed: "cached",
					Reason:      "Cache hit",
					Metadata:    map[string]string{"model": head.ModelType, "region": head.Region},
				}, nil
			}
		}
		configMutex.RUnlock()
	}

	// Cache miss - proceed with normal routing
	cacheMisses.Inc()

	configMutex.RLock()
	defer configMutex.RUnlock()

	var selectedHead *HeadService

	// Filter heads by model type
	var candidates []HeadService
	for _, head := range headServices {
		if head.ModelType == req.ModelType && head.Status == "active" {
			candidates = append(candidates, head)
		}
	}

	if len(candidates) == 0 {
		return &pb.GetRoutingDecisionResponse{
			HeadId:      "",
			Endpoint:    "",
			StrategyUsed: "none",
			Reason:      "No available heads for model type",
		}, nil
	}

	// Apply routing strategy based on request or default policy
	strategy := req.RoutingStrategy
	if strategy == "" {
		strategy = routingPolicy.DefaultStrategy
	}

	var reason string
	switch strategy {
	case "round_robin":
		selectedHead = applyRoundRobinStrategy(candidates)
		reason = "Round-robin selection"
	case "least_loaded":
		selectedHead = applyLeastLoadedStrategy(candidates)
		reason = "Least loaded selection"
	case "geo_preferred":
		selectedHead = applyGeoPreferredStrategy(candidates, req.RegionPreference)
		reason = "Geo-preferred selection"
	case "model_specific":
		selectedHead = applyModelSpecificStrategy(candidates, req.Metadata)
		reason = "Model-specific selection"
	case "hybrid":
		selectedHead = applyHybridStrategy(candidates, req)
		reason = "Hybrid strategy selection"
	default:
		// Default to round-robin
		selectedHead = applyRoundRobinStrategy(candidates)
		reason = "Default round-robin selection"
	}

	if selectedHead == nil {
		return &pb.GetRoutingDecisionResponse{
			HeadId:      "",
			Endpoint:    "",
			StrategyUsed: strategy,
			Reason:      "No suitable head found",
		}, nil
	}

	// Update cache
	cacheMutex.Lock()
	routingCache[cacheKey] = selectedHead.HeadID
	cacheMutex.Unlock()

	// Record metrics
	routingDecisions.WithLabelValues(strategy, req.ModelType, selectedHead.Region).Inc()

	return &pb.GetRoutingDecisionResponse{
		HeadId:      selectedHead.HeadID,
		Endpoint:    selectedHead.Endpoint,
		StrategyUsed: strategy,
		Reason:      reason,
		Metadata:    map[string]string{"model": selectedHead.ModelType, "region": selectedHead.Region},
	}, nil
}

func (s *RoutingServer) GetAllHeads(ctx context.Context, req *pb.GetAllHeadsRequest) (*pb.GetAllHeadsResponse, error) {
	configMutex.RLock()
	defer configMutex.RUnlock()

	var heads []*pb.HeadService
	for _, head := range headServices {
		heads = append(heads, &pb.HeadService{
			HeadId:        head.HeadID,
			Endpoint:      head.Endpoint,
			Status:        head.Status,
			CurrentLoad:   head.CurrentLoad,
			Region:        head.Region,
			ModelType:     head.ModelType,
			Version:       head.Version,
			Metadata:      head.Metadata,
			LastHeartbeat: head.LastHeartbeat,
		})
	}

	return &pb.GetAllHeadsResponse{Heads: heads}, nil
}

func (s *RoutingServer) UpdateRoutingPolicy(ctx context.Context, req *pb.UpdateRoutingPolicyRequest) (*pb.UpdateRoutingPolicyResponse, error) {
	configMutex.Lock()
	defer configMutex.Unlock()

	routingPolicy = RoutingPolicy{
		DefaultStrategy:   req.DefaultStrategy,
		EnableGeoRouting:  req.EnableGeoRouting,
		EnableLoadBalancing: req.EnableLoadBalancing,
		EnableModelSpecific: req.EnableModelSpecific,
		StrategyConfig:    req.StrategyConfig,
	}

	// Store in Redis
	err := storeRoutingPolicyInRedis(routingPolicy)
	if err != nil {
		return &pb.UpdateRoutingPolicyResponse{
			Success: false,
			Message: "Failed to store policy in Redis",
		}, err
	}

	return &pb.UpdateRoutingPolicyResponse{
		Success: true,
		Message: "Routing policy updated successfully",
	}, nil
}

func (s *RoutingServer) GetRoutingPolicy(ctx context.Context, req *pb.GetRoutingPolicyRequest) (*pb.GetRoutingPolicyResponse, error) {
	configMutex.RLock()
	defer configMutex.RUnlock()

	return &pb.GetRoutingPolicyResponse{
		DefaultStrategy:   routingPolicy.DefaultStrategy,
		EnableGeoRouting:  routingPolicy.EnableGeoRouting,
		EnableLoadBalancing: routingPolicy.EnableLoadBalancing,
		EnableModelSpecific: routingPolicy.EnableModelSpecific,
		StrategyConfig:    routingPolicy.StrategyConfig,
	}, nil
}

// HTTP Handlers

func getRoutingPolicy(w http.ResponseWriter, r *http.Request) {
	configMutex.RLock()
	defer configMutex.RUnlock()

	json.NewEncoder(w).Encode(routingPolicy)
}

func updateRoutingPolicy(w http.ResponseWriter, r *http.Request) {
	var policy RoutingPolicy
	err := json.NewDecoder(r.Body).Decode(&policy)
	if err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	configMutex.Lock()
	defer configMutex.Unlock()

	routingPolicy = policy

	// Store in Redis
	err = storeRoutingPolicyInRedis(policy)
	if err != nil {
		http.Error(w, "Failed to store policy", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(policy)
}

func getAllHeads(w http.ResponseWriter, r *http.Request) {
	configMutex.RLock()
	defer configMutex.RUnlock()

	json.NewEncoder(w).Encode(headServices)
}

// HTTP handler for head registration
func registerHeadHTTP(w http.ResponseWriter, r *http.Request) {
	var head HeadService
	err := json.NewDecoder(r.Body).Decode(&head)
	if err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	// Call the gRPC method
	resp, err := (&RoutingServer{}).RegisterHead(context.Background(), &pb.RegisterHeadRequest{
		HeadId:    head.HeadID,
		Endpoint:  head.Endpoint,
		Region:    head.Region,
		ModelType: head.ModelType,
		Version:   head.Version,
		Metadata:  head.Metadata,
	})

	if err != nil {
		http.Error(w, "Failed to register head", http.StatusInternalServerError)
		return
	}

	if !resp.Success {
		http.Error(w, resp.Message, http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

func healthCheck(w http.ResponseWriter, r *http.Request) {

	ctx := context.Background()
	err := redisClient.Ping(ctx).Err()
	if err != nil {
		http.Error(w, "Redis unavailable", http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

// Redis Functions

func storeHeadInRedis(head HeadService) error {
	ctx := context.Background()

	// Store head data
	headKey := fmt.Sprintf("head:%s", head.HeadID)
	headData := map[string]interface{}{
		"head_id":        head.HeadID,
		"endpoint":       head.Endpoint,
		"status":         head.Status,
		"current_load":   head.CurrentLoad,
		"region":         head.Region,
		"model_type":     head.ModelType,
		"version":        head.Version,
		"metadata":        head.Metadata,
		"last_heartbeat": head.LastHeartbeat,
	}

	err := redisClient.HMSet(ctx, headKey, headData).Err()
	if err != nil {
		return err
	}

	// Add to model type index
	err = redisClient.SAdd(ctx, fmt.Sprintf("model:%s:heads", head.ModelType), head.HeadID).Err()
	if err != nil {
		return err
	}

	// Add to region index
	err = redisClient.SAdd(ctx, fmt.Sprintf("region:%s:heads", head.Region), head.HeadID).Err()
	if err != nil {
		return err
	}

	return nil
}

func updateHeadStatusInRedis(head HeadService) error {
	ctx := context.Background()

	headKey := fmt.Sprintf("head:%s", head.HeadID)
	headData := map[string]interface{}{
		"status":         head.Status,
		"current_load":   head.CurrentLoad,
		"last_heartbeat": head.LastHeartbeat,
	}

	return redisClient.HMSet(ctx, headKey, headData).Err()
}

func storeRoutingPolicyInRedis(policy RoutingPolicy) error {
	ctx := context.Background()

	policyData := map[string]interface{}{
		"default_strategy":    policy.DefaultStrategy,
		"enable_geo_routing":  policy.EnableGeoRouting,
		"enable_load_balancing": policy.EnableLoadBalancing,
		"enable_model_specific": policy.EnableModelSpecific,
		"strategy_config":     policy.StrategyConfig,
	}

	return redisClient.HMSet(ctx, "routing:policy", policyData).Err()
}

// Routing Strategy Functions

// applyRoundRobinStrategy implements round-robin load balancing
func applyRoundRobinStrategy(heads []HeadService) *HeadService {
	// Simple round-robin: select the first head (in production, track last used)
	if len(heads) == 0 {
		return nil
	}
	return &heads[0]
}

// applyLeastLoadedStrategy selects the head with the lowest current load
func applyLeastLoadedStrategy(heads []HeadService) *HeadService {
	if len(heads) == 0 {
		return nil
	}

	// Find the head with the minimum load
	minLoad := heads[0]
	for _, head := range heads[1:] {
		if head.CurrentLoad < minLoad.CurrentLoad {
			minLoad = head
		}
	}
	return &minLoad
}

// applyGeoPreferredStrategy selects a head in the preferred region
func applyGeoPreferredStrategy(heads []HeadService, preferredRegion string) *HeadService {
	if len(heads) == 0 {
		return nil
	}

	// First try to find a head in the preferred region
	for _, head := range heads {
		if head.Region == preferredRegion {
			return &head
		}
	}

	// If no head in preferred region, fall back to round-robin
	return applyRoundRobinStrategy(heads)
}

// applyModelSpecificStrategy selects based on model-specific criteria
func applyModelSpecificStrategy(heads []HeadService, metadata map[string]string) *HeadService {
	if len(heads) == 0 {
		return nil
	}

	// For now, just return the first head
	// In production, this would use model-specific criteria from metadata
	return &heads[0]
}

// applyHybridStrategy combines multiple strategies
func applyHybridStrategy(heads []HeadService, req *pb.GetRoutingDecisionRequest) *HeadService {
	if len(heads) == 0 {
		return nil
	}

	// Hybrid approach: first try geo-preferred, then least-loaded
	geoHead := applyGeoPreferredStrategy(heads, req.RegionPreference)
	if geoHead != nil {
		return geoHead
	}

	return applyLeastLoadedStrategy(heads)
}

// External service integration
func callExternalService(serviceName, endpoint string, payload interface{}) ([]byte, error) {
	startTime := time.Now()

	// Check circuit breaker
	if !circuitBreaker.Allow(serviceName) {
		externalServiceCalls.WithLabelValues(serviceName, "circuit_breaker").Inc()
		return nil, fmt.Errorf("circuit breaker open for service %s", serviceName)
	}

	// Convert payload to JSON
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		externalServiceCalls.WithLabelValues(serviceName, "error").Inc()
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Make HTTP request to external service
	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(payloadBytes))
	if err != nil {
		externalServiceCalls.WithLabelValues(serviceName, "error").Inc()
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := externalServiceClient.Do(req)
	if err != nil {
		externalServiceCalls.WithLabelValues(serviceName, "error").Inc()
		circuitBreaker.Fail(serviceName)
		return nil, fmt.Errorf("external service request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		externalServiceCalls.WithLabelValues(serviceName, "error").Inc()
		circuitBreaker.Fail(serviceName)
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Record metrics
	status := "success"
	if resp.StatusCode >= 400 {
		status = "error"
		circuitBreaker.Fail(serviceName)
	} else {
		circuitBreaker.Success(serviceName)
	}
	externalServiceCalls.WithLabelValues(serviceName, status).Inc()

	return body, nil
}

// Enhanced circuit breaker implementation
type CircuitBreaker struct {
	mu            sync.Mutex
	failures      map[string]int
	lastFailure   map[string]time.Time
	threshold     int
	resetTimeout  time.Duration
	halfOpen      bool
	halfOpenUntil time.Time
	successCount  map[string]int
	failureCount map[string]int
	recoveryAttempts map[string]int
	serviceThresholds map[string]int
	serviceResetTimeouts map[string]time.Duration
	serviceHalfOpenDurations map[string]time.Duration
	serviceFailureWindows map[string]time.Duration
	serviceSuccessWindows map[string]time.Duration
	serviceRecoveryAttempts map[string]int
	serviceRecoverySuccesses map[string]int
	serviceRecoveryFailures map[string]int
	serviceRecoveryTime map[string]time.Duration
	serviceRecoveryLatency map[string]time.Duration
	serviceRecoveryThroughput map[string]float64
	serviceRecoverySuccessRate map[string]float64
	serviceRecoveryErrorRate map[string]float64
}

var circuitBreaker = &CircuitBreaker{
	failures:     make(map[string]int),
	lastFailure:  make(map[string]time.Time),
	threshold:    3,
	resetTimeout: 30 * time.Second,
	successCount: make(map[string]int),
	failureCount: make(map[string]int),
	recoveryAttempts: make(map[string]int),
	serviceThresholds: make(map[string]int),
	serviceResetTimeouts: make(map[string]time.Duration),
	serviceHalfOpenDurations: make(map[string]time.Duration),
	serviceFailureWindows: make(map[string]time.Duration),
	serviceSuccessWindows: make(map[string]time.Duration),
	serviceRecoveryAttempts: make(map[string]int),
	serviceRecoverySuccesses: make(map[string]int),
	serviceRecoveryFailures: make(map[string]int),
	serviceRecoveryTime: make(map[string]time.Duration),
	serviceRecoveryLatency: make(map[string]time.Duration),
	serviceRecoveryThroughput: make(map[string]float64),
	serviceRecoverySuccessRate: make(map[string]float64),
	serviceRecoveryErrorRate: make(map[string]float64),
}

func (cb *CircuitBreaker) Allow(service string) bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Get custom threshold for service
	threshold := cb.threshold
	if customThreshold, exists := cb.serviceThresholds[service]; exists {
		threshold = customThreshold
	}

	// Get custom reset timeout for service
	resetTimeout := cb.resetTimeout
	if customResetTimeout, exists := cb.serviceResetTimeouts[service]; exists {
		resetTimeout = customResetTimeout
	}

	// Get custom half-open duration for service
	halfOpenDuration := 10 * time.Second
	if customHalfOpenDuration, exists := cb.serviceHalfOpenDurations[service]; exists {
		halfOpenDuration = customHalfOpenDuration
	}

	// Check if circuit breaker is open
	if failures, exists := cb.failures[service]; exists && failures >= threshold {
		// Check if reset timeout has passed
		if lastFailure, exists := cb.lastFailure[service]; exists {
			if time.Since(lastFailure) < resetTimeout {
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
			cb.halfOpenUntil = time.Now().Add(halfOpenDuration)
			return true
		}
	}
	return true
}

func (cb *CircuitBreaker) Fail(service string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Increment failure count
	cb.failures[service]++
	cb.failureCount[service]++
	cb.recoveryAttempts[service]++
	cb.serviceRecoveryFailures[service]++
	cb.lastFailure[service] = time.Now()
	cb.halfOpen = false
}

func (cb *CircuitBreaker) Success(service string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Increment success count
	cb.successCount[service]++
	cb.serviceRecoverySuccesses[service]++
	cb.halfOpen = false

	// Reset failure count on success
	delete(cb.failures, service)
	delete(cb.lastFailure, service)
}

func (cb *CircuitBreaker) State(service string) string {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Get custom threshold for service
	threshold := cb.threshold
	if customThreshold, exists := cb.serviceThresholds[service]; exists {
		threshold = customThreshold
	}

	if failures, exists := cb.failures[service]; exists && failures >= threshold {
		if lastFailure, exists := cb.lastFailure[service]; exists {
			resetTimeout := cb.resetTimeout
			if customResetTimeout, exists := cb.serviceResetTimeouts[service]; exists {
				resetTimeout = customResetTimeout
			}

			if time.Since(lastFailure) < resetTimeout {
				return "open"
			}
			return "half-open"
		}
	}
	return "closed"
}

func (cb *CircuitBreaker) Metrics() map[string]interface{} {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	metrics := make(map[string]interface{})
	for service, failures := range cb.failures {
		metrics[service] = map[string]interface{}{
			"failures":     failures,
			"last_failure": cb.lastFailure[service],
			"state":        cb.State(service),
			"success_count": cb.successCount[service],
			"failure_count": cb.failureCount[service],
			"recovery_attempts": cb.recoveryAttempts[service],
			"threshold": cb.serviceThresholds[service],
			"reset_timeout": cb.serviceResetTimeouts[service],
			"half_open_duration": cb.serviceHalfOpenDurations[service],
			"failure_window": cb.serviceFailureWindows[service],
			"success_window": cb.serviceSuccessWindows[service],
			"recovery_successes": cb.serviceRecoverySuccesses[service],
			"recovery_failures": cb.serviceRecoveryFailures[service],
			"recovery_time": cb.serviceRecoveryTime[service],
			"recovery_latency": cb.serviceRecoveryLatency[service],
			"recovery_throughput": cb.serviceRecoveryThroughput[service],
			"recovery_success_rate": cb.serviceRecoverySuccessRate[service],
			"recovery_error_rate": cb.serviceRecoveryErrorRate[service],
		}
	}
	return metrics
}

func (cb *CircuitBreaker) Reset(service string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Reset all metrics for a service
	delete(cb.failures, service)
	delete(cb.lastFailure, service)
	delete(cb.successCount, service)
	delete(cb.failureCount, service)
	delete(cb.recoveryAttempts, service)
	delete(cb.serviceThresholds, service)
	delete(cb.serviceResetTimeouts, service)
	delete(cb.serviceHalfOpenDurations, service)
	delete(cb.serviceFailureWindows, service)
	delete(cb.serviceSuccessWindows, service)
	delete(cb.serviceRecoveryAttempts, service)
	delete(cb.serviceRecoverySuccesses, service)
	delete(cb.serviceRecoveryFailures, service)
	delete(cb.serviceRecoveryTime, service)
	delete(cb.serviceRecoveryLatency, service)
	delete(cb.serviceRecoveryThroughput, service)
	delete(cb.serviceRecoverySuccessRate, service)
	delete(cb.serviceRecoveryErrorRate, service)
	cb.halfOpen = false
}

func (cb *CircuitBreaker) SetThreshold(service string, threshold int) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Set custom threshold for a service
	cb.serviceThresholds[service] = threshold
	logger.Info("Setting custom circuit breaker threshold", zap.String("service", service), zap.Int("threshold", threshold))
}

func (cb *CircuitBreaker) SetResetTimeout(service string, resetTimeout time.Duration) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Set custom reset timeout for a service
	cb.serviceResetTimeouts[service] = resetTimeout
	logger.Info("Setting custom circuit breaker reset timeout", zap.String("service", service), zap.Duration("reset_timeout", resetTimeout))
}

func (cb *CircuitBreaker) SetHalfOpenDuration(service string, halfOpenDuration time.Duration) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Set custom half-open duration for a service
	cb.serviceHalfOpenDurations[service] = halfOpenDuration
	logger.Info("Setting custom circuit breaker half-open duration", zap.String("service", service), zap.Duration("half_open_duration", halfOpenDuration))
}

func (cb *CircuitBreaker) SetFailureWindow(service string, failureWindow time.Duration) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Set custom failure window for a service
	cb.serviceFailureWindows[service] = failureWindow
	logger.Info("Setting custom circuit breaker failure window", zap.String("service", service), zap.Duration("failure_window", failureWindow))
}

func (cb *CircuitBreaker) SetSuccessWindow(service string, successWindow time.Duration) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Set custom success window for a service
	cb.serviceSuccessWindows[service] = successWindow
	logger.Info("Setting custom circuit breaker success window", zap.String("service", service), zap.Duration("success_window", successWindow))
}

func (cb *CircuitBreaker) SetRecoveryTime(service string, recoveryTime time.Duration) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Set custom recovery time for a service
	cb.serviceRecoveryTime[service] = recoveryTime
	logger.Info("Setting custom circuit breaker recovery time", zap.String("service", service), zap.Duration("recovery_time", recoveryTime))
}

func (cb *CircuitBreaker) SetRecoveryLatency(service string, recoveryLatency time.Duration) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Set custom recovery latency for a service
	cb.serviceRecoveryLatency[service] = recoveryLatency
	logger.Info("Setting custom circuit breaker recovery latency", zap.String("service", service), zap.Duration("recovery_latency", recoveryLatency))
}

func (cb *CircuitBreaker) SetRecoveryThroughput(service string, recoveryThroughput float64) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Set custom recovery throughput for a service
	cb.serviceRecoveryThroughput[service] = recoveryThroughput
	logger.Info("Setting custom circuit breaker recovery throughput", zap.String("service", service), zap.Float64("recovery_throughput", recoveryThroughput))
}

func (cb *CircuitBreaker) SetRecoverySuccessRate(service string, recoverySuccessRate float64) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Set custom recovery success rate for a service
	cb.serviceRecoverySuccessRate[service] = recoverySuccessRate
	logger.Info("Setting custom circuit breaker recovery success rate", zap.String("service", service), zap.Float64("recovery_success_rate", recoverySuccessRate))
}

func (cb *CircuitBreaker) SetRecoveryErrorRate(service string, recoveryErrorRate float64) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Set custom recovery error rate for a service
	cb.serviceRecoveryErrorRate[service] = recoveryErrorRate
	logger.Info("Setting custom circuit breaker recovery error rate", zap.String("service", service), zap.Float64("recovery_error_rate", recoveryErrorRate))
}

func (cb *CircuitBreaker) SetGlobalThreshold(threshold int) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Set global threshold
	cb.threshold = threshold
	logger.Info("Setting global circuit breaker threshold", zap.Int("threshold", threshold))
}

func (cb *CircuitBreaker) SetGlobalResetTimeout(resetTimeout time.Duration) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Set global reset timeout
	cb.resetTimeout = resetTimeout
	logger.Info("Setting global circuit breaker reset timeout", zap.Duration("reset_timeout", resetTimeout))
}

func (cb *CircuitBreaker) SetGlobalHalfOpenDuration(halfOpenDuration time.Duration) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Set global half-open duration
	cb.halfOpenDuration = halfOpenDuration
	logger.Info("Setting global circuit breaker half-open duration", zap.Duration("half_open_duration", halfOpenDuration))
}

func (cb *CircuitBreaker) SetGlobalFailureWindow(failureWindow time.Duration) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Set global failure window
	cb.failureWindow = failureWindow
	logger.Info("Setting global circuit breaker failure window", zap.Duration("failure_window", failureWindow))
}

func (cb *CircuitBreaker) SetGlobalSuccessWindow(successWindow time.Duration) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Set global success window
	cb.successWindow = successWindow
	logger.Info("Setting global circuit breaker success window", zap.Duration("success_window", successWindow))
}

func (cb *CircuitBreaker) SetGlobalRecoveryTime(recoveryTime time.Duration) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Set global recovery time
	cb.recoveryTime = recoveryTime
	logger.Info("Setting global circuit breaker recovery time", zap.Duration("recovery_time", recoveryTime))
}

func (cb *CircuitBreaker) SetGlobalRecoveryLatency(recoveryLatency time.Duration) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Set global recovery latency
	cb.recoveryLatency = recoveryLatency
	logger.Info("Setting global circuit breaker recovery latency", zap.Duration("recovery_latency", recoveryLatency))
}

func (cb *CircuitBreaker) SetGlobalRecoveryThroughput(recoveryThroughput float64) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Set global recovery throughput
	cb.recoveryThroughput = recoveryThroughput
	logger.Info("Setting global circuit breaker recovery throughput", zap.Float64("recovery_throughput", recoveryThroughput))
}

func (cb *CircuitBreaker) SetGlobalRecoverySuccessRate(recoverySuccessRate float64) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Set global recovery success rate
	cb.recoverySuccessRate = recoverySuccessRate
	logger.Info("Setting global circuit breaker recovery success rate", zap.Float64("recovery_success_rate", recoverySuccessRate))
}

func (cb *CircuitBreaker) SetGlobalRecoveryErrorRate(recoveryErrorRate float64) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Set global recovery error rate
	cb.recoveryErrorRate = recoveryErrorRate
	logger.Info("Setting global circuit breaker recovery error rate", zap.Float64("recovery_error_rate", recoveryErrorRate))
}

