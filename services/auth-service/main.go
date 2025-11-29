


package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/mux"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var (
	db           *gorm.DB
	rdb          *redis.Client
	secret       []byte
	logger       zerolog.Logger
	authCounter  = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "auth_operations_total",
			Help: "Total number of authentication operations",
		},
		[]string{"operation", "status"},
	)
	httpDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path", "status"},
	)
)

const (
	InvalidCredentialsError = "invalid credentials"
	WeakPasswordError       = "weak password"
	InvalidEmailError       = "invalid email"
	RateLimitExceededError  = "rate limit exceeded"
	InternalServerError     = "internal server error"
	UnauthorizedError       = "unauthorized"
)

func init() {
	// Initialize structured logger
	logger = zerolog.New(os.Stdout).With().
		Timestamp().
		Str("service", "auth-service").
		Logger()

	// Register Prometheus metrics
	prometheus.MustRegister(authCounter, httpDuration)

	// Load JWT secret from environment
	secret = []byte(os.Getenv("JWT_SECRET"))
	if len(secret) == 0 {
		logger.Fatal().Msg("JWT_SECRET environment variable not set")
	}

	// Initialize Redis
	rdb = redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_ADDR"),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       0,
	})

	// Test Redis connection
	_, err := rdb.Ping(context.Background()).Result()
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to connect to Redis")
	}

	logger.Info().Msg("Auth service initialized successfully")
}

type User struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	Email     string    `gorm:"unique" json:"email"`
	Password  string    `json:"-"`
	Role      string    `json:"role"` // user, admin, superadmin
	Balance   float64   `json:"balance_usd"`
	TOTP      string    `json:"-"` // encrypted secret
	CreatedAt time.Time `json:"created_at"`
}

type APIKey struct {
	ID      string    `gorm:"primaryKey"`
	UserID  string    `gorm:"index"`
	Key     string    `gorm:"unique"`
	Prefix  string
	Name    string
	Active  bool
	Created time.Time
}

func main() {
	// Initialize Prometheus metrics
	prometheus.MustRegister(authCounter, httpDuration)

	// Initialize database
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
		os.Getenv("DB_PORT"),
	)

	var err error
	db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to connect to database")
	}

	// Auto migrate database schema
	err = db.AutoMigrate(&User{}, &APIKey{})
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to migrate database schema")
	}

	r := mux.NewRouter()
	r.HandleFunc("/register", Register).Methods("POST")
	r.HandleFunc("/login", rateLimitMiddleware(Login)).Methods("POST")
	r.HandleFunc("/me", AuthMiddleware(Me)).Methods("GET")
	r.HandleFunc("/api-keys", AuthMiddleware(ListAPIKeys)).Methods("GET")
	r.HandleFunc("/api-keys", AuthMiddleware(CreateAPIKey)).Methods("POST")
	r.HandleFunc("/balance", AuthMiddleware(GetBalance)).Methods("GET")

	// Health check endpoint
	r.HandleFunc("/health", HealthCheck).Methods("GET")

	// Prometheus metrics endpoint
	r.Handle("/metrics", promhttp.Handler())

	// gRPC for gateway with mTLS
	go func() {
		lis, err := net.Listen("tcp", ":50051")
		if err != nil {
			logger.Fatal().Err(err).Msg("Failed to listen for gRPC")
		}

		creds, err := loadTLSCredentials()
		if err != nil {
			logger.Fatal().Err(err).Msg("Failed to load TLS credentials")
		}

		s := grpc.NewServer(grpc.Creds(creds))
		pb.RegisterAuthServiceServer(s, &server{})
		logger.Info().Msg("Auth service gRPC+mTLS listening on :50051")
		if err := s.Serve(lis); err != nil {
			logger.Fatal().Err(err).Msg("gRPC server failed")
		}
	}()

	logger.Info().Msg("Auth service: HTTP :8081 | gRPC+mTLS :50051")
	log.Fatal(http.ListenAndServe(":8081", r))
}

// Custom error types
type AuthError struct {
	Code    codes.Code
	Message string
	Details string
}

func (e *AuthError) Error() string {
	return fmt.Sprintf("%s: %s", e.Message, e.Details)
}

func newAuthError(code codes.Code, message, details string) *AuthError {
	return &AuthError{Code: code, Message: message, Details: details}
}

// === HTTP API ===
func Register(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	logger.Info().Str("method", "Register").Msg("Received registration request")

	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error().Err(err).Msg("Failed to decode request body")
		http.Error(w, "invalid input", 400)
		httpDuration.WithLabelValues("POST", "/register", "400").Observe(time.Since(start).Seconds())
		return
	}

	// Validate email
	if !isValidEmail(req.Email) {
		logger.Warn().Str("email", req.Email).Msg("Invalid email format")
		http.Error(w, InvalidEmailError, 400)
		httpDuration.WithLabelValues("POST", "/register", "400").Observe(time.Since(start).Seconds())
		return
	}

	// Validate password strength
	if !isStrongPassword(req.Password) {
		logger.Warn().Msg("Weak password attempt")
		http.Error(w, WeakPasswordError, 400)
		httpDuration.WithLabelValues("POST", "/register", "400").Observe(time.Since(start).Seconds())
		return
	}

	// Check if user already exists
	var existingUser User
	if err := db.Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
		logger.Warn().Str("email", req.Email).Msg("User already exists")
		http.Error(w, "user already exists", 409)
		httpDuration.WithLabelValues("POST", "/register", "409").Observe(time.Since(start).Seconds())
		return
	}

	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to hash password")
		http.Error(w, InternalServerError, 500)
		httpDuration.WithLabelValues("POST", "/register", "500").Observe(time.Since(start).Seconds())
		return
	}

	// Create user
	user := User{
		ID:        uuid.New().String(),
		Email:     req.Email,
		Password:  string(hash),
		Role:      "user",
		Balance:   10.0, // starting bonus
		CreatedAt: time.Now(),
	}

	if err := db.Create(&user).Error; err != nil {
		logger.Error().Err(err).Str("email", req.Email).Msg("Failed to create user")
		http.Error(w, InternalServerError, 500)
		httpDuration.WithLabelValues("POST", "/register", "500").Observe(time.Since(start).Seconds())
		return
	}

	// Generate first API key
	createAPIKeyForUser(user.ID, "Default key")

	logger.Info().Str("user_id", user.ID).Msg("User registered successfully")
	httpDuration.WithLabelValues("POST", "/register", "200").Observe(time.Since(start).Seconds())
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "user_id": user.ID})
}

func Login(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	logger.Info().Str("method", "Login").Msg("Received login request")

	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error().Err(err).Msg("Failed to decode request body")
		http.Error(w, "invalid input", 400)
		httpDuration.WithLabelValues("POST", "/login", "400").Observe(time.Since(start).Seconds())
		return
	}

	var user User
	if err := db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		logger.Warn().Str("email", req.Email).Msg("User not found")
		http.Error(w, InvalidCredentialsError, 401)
		httpDuration.WithLabelValues("POST", "/login", "401").Observe(time.Since(start).Seconds())
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		logger.Warn().Str("email", req.Email).Msg("Invalid password attempt")
		http.Error(w, InvalidCredentialsError, 401)
		httpDuration.WithLabelValues("POST", "/login", "401").Observe(time.Since(start).Seconds())
		return
	}

	// Generate JWT token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": user.ID,
		"email":   user.Email,
		"role":    user.Role,
		"exp":     time.Now().Add(30 * 24 * time.Hour).Unix(),
	})

	signed, err := token.SignedString(secret)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to sign JWT token")
		http.Error(w, InternalServerError, 500)
		httpDuration.WithLabelValues("POST", "/login", "500").Observe(time.Since(start).Seconds())
		return
	}

	logger.Info().Str("user_id", user.ID).Msg("User logged in successfully")
	httpDuration.WithLabelValues("POST", "/login", "200").Observe(time.Since(start).Seconds())
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token":   signed,
		"user":   user,
		"api_keys": getUserAPIKeys(user.ID),
	})
}

func Me(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	user := r.Context().Value("user").(User)

	logger.Info().Str("user_id", user.ID).Msg("User info request")
	httpDuration.WithLabelValues("GET", "/me", "200").Observe(time.Since(start).Seconds())
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

func ListAPIKeys(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	user := r.Context().Value("user").(User)

	logger.Info().Str("user_id", user.ID).Msg("List API keys request")
	keys := getUserAPIKeys(user.ID)
	httpDuration.WithLabelValues("GET", "/api-keys", "200").Observe(time.Since(start).Seconds())
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(keys)
}

func CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	user := r.Context().Value("user").(User)

	var req struct { Name string }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error().Err(err).Msg("Failed to decode request body")
		http.Error(w, "invalid input", 400)
		httpDuration.WithLabelValues("POST", "/api-keys", "400").Observe(time.Since(start).Seconds())
		return
	}

	if req.Name == "" {
		logger.Warn().Msg("API key name is required")
		http.Error(w, "name is required", 400)
		httpDuration.WithLabelValues("POST", "/api-keys", "400").Observe(time.Since(start).Seconds())
		return
	}

	createAPIKeyForUser(user.ID, req.Name)

	logger.Info().Str("user_id", user.ID).Str("key_name", req.Name).Msg("API key created")
	httpDuration.WithLabelValues("POST", "/api-keys", "200").Observe(time.Since(start).Seconds())
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "created"})
}

func GetBalance(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	user := r.Context().Value("user").(User)

	logger.Info().Str("user_id", user.ID).Msg("Balance check request")
	httpDuration.WithLabelValues("GET", "/balance", "200").Observe(time.Since(start).Seconds())
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"balance": user.Balance,
		"currency": "USD",
	})
}

func createAPIKeyForUser(userID, name string) {
	prefix := "tvo_"
	raw := make([]byte, 32)
	rand.Read(raw)
	key := prefix + base64.URLEncoding.EncodeToString(raw)[:32]

	apiKey := APIKey{
		ID:      uuid.New().String(),
		UserID:  userID,
		Key:     key,
		Prefix:  prefix,
		Name:    name,
		Active:  true,
		Created: time.Now(),
	}

	if err := db.Create(&apiKey).Error; err != nil {
		logger.Error().Err(err).Str("user_id", userID).Msg("Failed to create API key")
	}
}

func getUserAPIKeys(userID string) []map[string]interface{} {
	var keys []APIKey
	if err := db.Where("user_id = ?", userID).Find(&keys).Error; err != nil {
		logger.Error().Err(err).Str("user_id", userID).Msg("Failed to get API keys")
		return []map[string]interface{}{}
	}

	var result []map[string]interface{}
	for _, k := range keys {
		result = append(result, map[string]interface{}{
			"id":      k.ID,
			"name":    k.Name,
			"key":     k.Key,
			"prefix":  k.Prefix,
			"created": k.Created,
		})
	}
	return result
}

// === gRPC for gateway ===
type server struct{ pb.UnimplementedAuthServiceServer }

func (s *server) ValidateAPIKey(ctx context.Context, req *pb.ValidateRequest) (*pb.ValidateResponse, error) {
	key := req.ApiKey
	if !strings.HasPrefix(key, "tvo_") {
		return &pb.ValidateResponse{Valid: false}, nil
	}

	var apiKey APIKey
	if err := db.Where("key = ?", key).First(&apiKey).Error; err != nil {
		return &pb.ValidateResponse{Valid: false}, nil
	}

	var user User
	if err := db.First(&user, "id = ?", apiKey.UserID).Error; err != nil {
		return &pb.ValidateResponse{Valid: false}, nil
	}

	return &pb.ValidateResponse{
		Valid:   true,
		UserId:  user.ID,
		Role:    user.Role,
		Balance: user.Balance,
	}, nil
}

// === Middleware ===
func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		tokenStr := r.Header.Get("Authorization")
		if strings.HasPrefix(tokenStr, "Bearer ") {
			tokenStr = strings.TrimPrefix(tokenStr, "Bearer ")
		}

		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			// Validate signing method
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				logger.Warn().Msg("Invalid JWT signing method")
				http.Error(w, UnauthorizedError, 401)
				httpDuration.WithLabelValues(r.Method, r.URL.Path, "401").Observe(time.Since(start).Seconds())
				return nil, fmt.Errorf("invalid signing method")
			}
			return secret, nil
		})

		if err != nil {
			logger.Warn().Err(err).Msg("JWT parsing failed")
			http.Error(w, UnauthorizedError, 401)
			httpDuration.WithLabelValues(r.Method, r.URL.Path, "401").Observe(time.Since(start).Seconds())
			return
		}

		if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
			var user User
			if err := db.First(&user, "id = ?", claims["user_id"]).Error; err != nil {
				logger.Warn().Str("user_id", claims["user_id"].(string)).Msg("User not found")
				http.Error(w, UnauthorizedError, 401)
				httpDuration.WithLabelValues(r.Method, r.URL.Path, "401").Observe(time.Since(start).Seconds())
				return
			}
			ctx := context.WithValue(r.Context(), "user", user)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		logger.Warn().Msg("Invalid JWT token")
		http.Error(w, UnauthorizedError, 401)
		httpDuration.WithLabelValues(r.Method, r.URL.Path, "401").Observe(time.Since(start).Seconds())
	}
}

func rateLimitMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ip := r.RemoteAddr
		key := fmt.Sprintf("rate_limit:%s", ip)

		// Check rate limit in Redis
		count, err := rdb.Get(context.Background(), key).Int()
		if err == nil && count >= 5 {
			logger.Warn().Str("ip", ip).Msg("Rate limit exceeded")
			http.Error(w, RateLimitExceededError, 429)
			httpDuration.WithLabelValues(r.Method, r.URL.Path, "429").Observe(time.Since(start).Seconds())
			return
		}

		// Increment counter
		pipe := rdb.TxPipeline()
		pipe.Incr(context.Background(), key)
		pipe.Expire(context.Background(), key, 5*time.Minute)
		_, err = pipe.Exec(context.Background())
		if err != nil {
			logger.Error().Err(err).Msg("Failed to increment rate limit counter")
		}

		next.ServeHTTP(w, r)
	}
}

func HealthCheck(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// Check database
	if err := db.Exec("SELECT 1").Error; err != nil {
		logger.Error().Err(err).Msg("Database health check failed")
		http.Error(w, "database unhealthy", 503)
		httpDuration.WithLabelValues("GET", "/health", "503").Observe(time.Since(start).Seconds())
		return
	}

	// Check Redis
	_, err := rdb.Ping(context.Background()).Result()
	if err != nil {
		logger.Error().Err(err).Msg("Redis health check failed")
		http.Error(w, "redis unhealthy", 503)
		httpDuration.WithLabelValues("GET", "/health", "503").Observe(time.Since(start).Seconds())
		return
	}

	logger.Info().Msg("Health check passed")
	httpDuration.WithLabelValues("GET", "/health", "200").Observe(time.Since(start).Seconds())
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

// === Helper Functions ===
func isValidEmail(email string) bool {
	// Simple email validation
	return strings.Contains(email, "@") && len(email) > 5
}

func isStrongPassword(password string) bool {
	// Simple password strength check
	return len(password) >= 8 &&
		strings.ContainsAny(password, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") &&
		strings.ContainsAny(password, "abcdefghijklmnopqrstuvwxyz") &&
		strings.ContainsAny(password, "0123456789")
}

func loadTLSCredentials() (credentials.TransportCredentials, error) {
	// Load server certificate and key
	serverCert, err := tls.LoadX509KeyPair("/certs/auth-service.pem", "/certs/auth-service-key.pem")
	if err != nil {
		return nil, fmt.Errorf("failed to load server certificate: %w", err)
	}

	// Load CA certificate for client verification
	caCert, err := os.ReadFile("/certs/ca.pem")
	if err != nil {
		return nil, fmt.Errorf("failed to load CA certificate: %w", err)
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to add CA certificate to pool")
	}

	// Create TLS config with proper validation
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    certPool,
		MinVersion:   tls.VersionTLS12,
	}

	return credentials.NewTLS(tlsConfig), nil
}


