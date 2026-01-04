package middleware

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-redis/redis/v8"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

// AuthMiddleware handles JWT authentication
type AuthMiddleware struct {
	secretKey string
	logger    *zap.Logger
}

// NewAuthMiddleware creates a new auth middleware
func NewAuthMiddleware(secretKey string, logger *zap.Logger) *AuthMiddleware {
	return &AuthMiddleware{
		secretKey: secretKey,
		logger:    logger,
	}
}

// AuthRequired middleware validates JWT tokens
func (am *AuthMiddleware) AuthRequired(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		// Extract token from "Bearer <token>" format
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, "Invalid authorization header format", http.StatusUnauthorized)
			return
		}

		tokenString := parts[1]
		claims := &Claims{}

		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			return []byte(am.secretKey), nil
		})

		if err != nil {
			am.logger.Warn("Invalid token", zap.Error(err))
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		if !token.Valid {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		// Add user info to request context
		ctx := r.Context()
		ctx = withUserID(ctx, claims.Subject)
		ctx = withUserRole(ctx, claims.Role)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Claims represents JWT claims
type Claims struct {
	Subject string `json:"sub"`
	Role    string `json:"role"`
	jwt.RegisteredClaims
}

// RateLimitMiddleware handles rate limiting using Redis
type RateLimitMiddleware struct {
	redis  *redis.Client
	logger *zap.Logger
	limits map[string]RateLimit
}

// RateLimit represents rate limiting configuration
type RateLimit struct {
	Requests int           `json:"requests"`
	Window   time.Duration `json:"window"`
}

// NewRateLimitMiddleware creates a new rate limit middleware
func NewRateLimitMiddleware(redisClient *redis.Client, logger *zap.Logger) *RateLimitMiddleware {
	return &RateLimitMiddleware{
		redis:  redisClient,
		logger: logger,
		limits: map[string]RateLimit{
			"default": {Requests: 100, Window: time.Hour},
			"admin":   {Requests: 1000, Window: time.Hour},
		},
	}
}

// RateLimit middleware implementation
func (rlm *RateLimitMiddleware) RateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		userID := getUserID(ctx)
		if userID == "" {
			// Allow anonymous requests with lower limits
			userID = getClientIP(r)
		}

		// Get rate limit for user role
		role := getUserRole(ctx)
		limit := rlm.limits[role]
		if limit.Requests == 0 {
			limit = rlm.limits["default"]
		}

		key := "rate_limit:" + userID
		now := time.Now()
		window := limit.Window

		// Clean old entries and count current
		err := rlm.redis.ZRemRangeByScore(ctx, key, "0", strconv.FormatInt(now.Add(-window).Unix(), 10)).Err()
		if err != nil {
			rlm.logger.Error("Failed to clean rate limit data", zap.Error(err))
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		count, err := rlm.redis.ZCard(ctx, key).Result()
		if err != nil {
			rlm.logger.Error("Failed to get rate limit count", zap.Error(err))
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		if int(count) >= limit.Requests {
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		// Add current request
		score := float64(now.UnixNano()) / float64(time.Second)
		err = rlm.redis.ZAdd(ctx, key, redis.Z{
			Score:  score,
			Member: strconv.FormatInt(now.Unix(), 10),
		}).Err()
		if err != nil {
			rlm.logger.Error("Failed to add rate limit entry", zap.Error(err))
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Set expiration
		err = rlm.redis.Expire(ctx, key, window).Err()
		if err != nil {
			rlm.logger.Error("Failed to set rate limit expiration", zap.Error(err))
		}

		next.ServeHTTP(w, r)
	})
}

// LoggingMiddleware logs HTTP requests
func LoggingMiddleware(logger *zap.Logger) func(next http.Handler) http.Handler {
	return middleware.Logger
}

// RecoveryMiddleware handles panics
func RecoveryMiddleware(logger *zap.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					logger.Error("Panic recovered", zap.Any("error", err))
					http.Error(w, "Internal server error", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}
}

// CORS middleware
func CORS(allowedOrigins []string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Check if origin is allowed
			allowed := false
			for _, allowedOrigin := range allowedOrigins {
				if allowedOrigin == "*" || allowedOrigin == origin {
					allowed = true
					break
				}
			}

			if allowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
			}

			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Context helper functions
type contextKey string

const (
	userIDKey   contextKey = "userID"
	userRoleKey contextKey = "userRole"
)

func withUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

func getUserID(ctx context.Context) string {
	if value := ctx.Value(userIDKey); value != nil {
		if id, ok := value.(string); ok {
			return id
		}
	}
	return ""
}

func withUserRole(ctx context.Context, role string) context.Context {
	return context.WithValue(ctx, userRoleKey, role)
}

func getUserRole(ctx context.Context) string {
	if value := ctx.Value(userRoleKey); value != nil {
		if role, ok := value.(string); ok {
			return role
		}
	}
	return "user"
}

func getClientIP(r *http.Request) string {
	// Try common headers first
	headers := []string{
		"X-Forwarded-For",
		"X-Real-IP",
		"X-Client-IP",
		"CF-Connecting-IP",
	}

	for _, header := range headers {
		if ip := r.Header.Get(header); ip != "" {
			// Handle comma-separated IPs
			ips := strings.Split(ip, ",")
			return strings.TrimSpace(ips[0])
		}
	}

	// Fallback to remote address
	return r.RemoteAddr
}
