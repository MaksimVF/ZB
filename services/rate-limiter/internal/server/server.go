package server

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	pb "github.com/MaksimVF/ZB/services/rate-limiter/pb"
	"github.com/go-redis/redis/v8"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

type RateLimiterServer struct {
	pb.UnimplementedRateLimiterServer
	limits  map[string]map[string]int // path -> authPrefix -> limit
	usage   map[string]map[string]int // path -> authPrefix -> currentUsage
	redis   *redis.Client
	mutex   sync.RWMutex
	metrics *Metrics
}

type Metrics struct {
	checkRequests    prometheus.Counter
	checkAllowed     prometheus.Counter
	checkDenied      prometheus.Counter
	setLimitRequests prometheus.Counter
	getLimitRequests prometheus.Counter
	activeRequests   prometheus.Gauge
}

func NewMetricsWithRegistry(registry *prometheus.Registry) *Metrics {
	m := &Metrics{
		checkRequests: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "rate_limiter",
			Name:      "check_requests_total",
			Help:      "Total number of check requests",
		}),
		checkAllowed: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "rate_limiter",
			Name:      "check_allowed_total",
			Help:      "Total number of allowed check requests",
		}),
		checkDenied: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "rate_limiter",
			Name:      "check_denied_total",
			Help:      "Total number of denied check requests",
		}),
		setLimitRequests: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "rate_limiter",
			Name:      "set_limit_requests_total",
			Help:      "Total number of set limit requests",
		}),
		getLimitRequests: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "rate_limiter",
			Name:      "get_limit_requests_total",
			Help:      "Total number of get limit requests",
		}),
		activeRequests: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "rate_limiter",
			Name:      "active_requests",
			Help:      "Current number of active requests",
		}),
	}

	// Register metrics in the provided registry
	if registry != nil {
		registry.MustRegister(
			m.checkRequests,
			m.checkAllowed,
			m.checkDenied,
			m.setLimitRequests,
			m.getLimitRequests,
			m.activeRequests,
		)
	} else {
		// Register metrics in the global registry
		prometheus.MustRegister(
			m.checkRequests,
			m.checkAllowed,
			m.checkDenied,
			m.setLimitRequests,
			m.getLimitRequests,
			m.activeRequests,
		)
	}

	return m
}

func NewMetrics() *Metrics {
	return NewMetricsWithRegistry(nil)
}

func NewRateLimiterServer() *RateLimiterServer {
	// Initialize Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr:     "redis:6379",
		Password: "",
		DB:       0,
	})

	// Test Redis connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Printf("Warning: Failed to connect to Redis: %v. Using in-memory storage only.", err)
	}

	return &RateLimiterServer{
		limits:  make(map[string]map[string]int),
		usage:   make(map[string]map[string]int),
		redis:   redisClient,
		metrics: NewMetrics(),
	}
}

func (s *RateLimiterServer) Run() error {
	// Start metrics server
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		log.Println("Metrics server starting on :8086")
		if err := http.ListenAndServe(":8086", nil); err != nil {
			log.Printf("Metrics server failed: %v", err)
		}
	}()

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	// Try to load TLS credentials, fallback to insecure if not available
	var grpcOpts []grpc.ServerOption
	var creds credentials.TransportCredentials
	creds, err = credentials.NewServerTLSFromFile("/certs/rate-limiter.pem", "/certs/rate-limiter-key.pem")
	if err != nil {
		log.Printf("Failed to load TLS credentials: %v. Running without TLS.", err)
		creds = insecure.NewCredentials()
	}
	grpcOpts = append(grpcOpts, grpc.Creds(creds))

	grpcServer := grpc.NewServer(grpcOpts...)
	pb.RegisterRateLimiterServer(grpcServer, s)

	if creds != insecure.NewCredentials() {
		log.Println("Rate limiter service running on :50051 (TLS enabled)")
	} else {
		log.Println("Rate limiter service running on :50051 (no TLS)")
	}

	return grpcServer.Serve(lis)
}

func (s *RateLimiterServer) Check(ctx context.Context, req *pb.CheckRequest) (*pb.CheckResponse, error) {
	s.metrics.checkRequests.Inc()
	s.metrics.activeRequests.Inc()
	defer s.metrics.activeRequests.Dec()

	path := req.Path
	auth := req.Authorization

	// Determine the auth prefix (JWT or API key)
	var authPrefix string
	if strings.HasPrefix(auth, "Bearer ") {
		authPrefix = "jwt"
	} else if strings.HasPrefix(auth, "tvo_") {
		authPrefix = "api_key"
	} else {
		authPrefix = "anonymous"
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Get or initialize limits for this path
	if s.limits[path] == nil {
		s.initializeDefaultLimits(path)
	}

	// Get current usage from Redis or memory
	currentUsage, err := s.getCurrentUsage(path, authPrefix)
	if err != nil {
		log.Printf("Error getting current usage: %v", err)
		// On Redis error, allow the request to avoid complete service disruption
		s.metrics.checkAllowed.Inc()
		return &pb.CheckResponse{
			Allowed:        true,
			RetryAfterSecs: 0,
		}, nil
	}

	limit := s.limits[path][authPrefix]

	// Check if limit is exceeded
	if currentUsage >= limit {
		s.metrics.checkDenied.Inc()
		return &pb.CheckResponse{
			Allowed:        false,
			RetryAfterSecs: 60, // 1 minute retry
		}, nil
	}

	// Increment usage in Redis
	err = s.incrementUsage(path, authPrefix)
	if err != nil {
		log.Printf("Error incrementing usage: %v", err)
		// On Redis error, allow the request to avoid complete service disruption
		s.metrics.checkAllowed.Inc()
		return &pb.CheckResponse{
			Allowed:        true,
			RetryAfterSecs: 0,
		}, nil
	}

	s.metrics.checkAllowed.Inc()
	return &pb.CheckResponse{
		Allowed:        true,
		RetryAfterSecs: 0,
	}, nil
}

func (s *RateLimiterServer) initializeDefaultLimits(path string) {
	s.limits[path] = make(map[string]int)

	// Set default limits per path
	switch path {
	case "/v1/chat/completions", "/v1/completions":
		s.limits[path]["jwt"] = 60      // 60 requests per minute for JWT
		s.limits[path]["api_key"] = 30  // 30 requests per minute for API keys
		s.limits[path]["anonymous"] = 5 // 5 requests per minute for anonymous
	case "/v1/embeddings":
		s.limits[path]["jwt"] = 120
		s.limits[path]["api_key"] = 60
		s.limits[path]["anonymous"] = 10
	case "/v1/agentic":
		s.limits[path]["jwt"] = 30
		s.limits[path]["api_key"] = 15
		s.limits[path]["anonymous"] = 3
	default:
		// Default limits for unknown paths
		s.limits[path]["jwt"] = 100
		s.limits[path]["api_key"] = 50
		s.limits[path]["anonymous"] = 10
	}
}

func (s *RateLimiterServer) getCurrentUsage(path, authPrefix string) (int, error) {
	key := fmt.Sprintf("rl:%s:%s", path, authPrefix)

	// Try Redis first
	if s.redis != nil {
		val, err := s.redis.Get(context.Background(), key).Int()
		if err == nil {
			return val, nil
		}
		if err != redis.Nil {
			return 0, fmt.Errorf("redis get error: %w", err)
		}
	}

	// Fallback to memory
	return s.usage[path][authPrefix], nil
}

func (s *RateLimiterServer) incrementUsage(path, authPrefix string) error {
	key := fmt.Sprintf("rl:%s:%s", path, authPrefix)

	// Use Redis if available
	if s.redis != nil {
		pipe := s.redis.Pipeline()
		pipe.Incr(context.Background(), key)
		pipe.Expire(context.Background(), key, time.Minute)
		_, err := pipe.Exec(context.Background())
		return err
	}

	// Fallback to memory
	if s.usage[path] == nil {
		s.usage[path] = make(map[string]int)
	}
	s.usage[path][authPrefix]++

	return nil
}

func (s *RateLimiterServer) SetLimit(ctx context.Context, req *pb.SetLimitRequest) (*pb.SetLimitResponse, error) {
	s.metrics.setLimitRequests.Inc()

	path := req.Path
	authType := req.AuthType
	limit := req.Limit

	// Validate auth type
	if authType != "jwt" && authType != "api_key" && authType != "anonymous" {
		return &pb.SetLimitResponse{
			Success: false,
			Message: "Invalid auth_type. Must be 'jwt', 'api_key', or 'anonymous'",
		}, nil
	}

	// Validate limit
	if limit <= 0 || limit > 10000 {
		return &pb.SetLimitResponse{
			Success: false,
			Message: "Invalid limit. Must be between 1 and 10000",
		}, nil
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Initialize path if not exists
	if s.limits[path] == nil {
		s.limits[path] = make(map[string]int)
	}

	// Set the limit
	s.limits[path][authType] = int(limit)

	// Save to Redis for persistence
	if s.redis != nil {
		redisKey := fmt.Sprintf("limit:%s:%s", path, authType)
		err := s.redis.Set(context.Background(), redisKey, limit, 0).Err()
		if err != nil {
			log.Printf("Warning: Failed to save limit to Redis: %v", err)
		}
	}

	return &pb.SetLimitResponse{
		Success: true,
		Message: fmt.Sprintf("Limit set for %s/%s: %d", path, authType, limit),
	}, nil
}

func (s *RateLimiterServer) GetLimits(ctx context.Context, req *pb.GetLimitsRequest) (*pb.GetLimitsResponse, error) {
	s.metrics.getLimitRequests.Inc()

	s.mutex.RLock()
	defer s.mutex.RUnlock()

	limits := make(map[string]*pb.LimitConfig)

	for path, authLimits := range s.limits {
		limits[path] = &pb.LimitConfig{
			Path:           path,
			JwtLimit:       int32(authLimits["jwt"]),
			ApiKeyLimit:    int32(authLimits["api_key"]),
			AnonymousLimit: int32(authLimits["anonymous"]),
		}
	}

	return &pb.GetLimitsResponse{
		Limits: limits,
	}, nil
}
