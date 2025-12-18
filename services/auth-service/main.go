package main

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
	"github.com/golang-jwt/jwt/v5"
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

	pb "github.com/MaksimVF/ZB/services/head-go/gen"
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
	securityEvents = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "security_events_total",
			Help: "Total number of security events by type and risk level",
		},
		[]string{"event_type", "risk_level"},
	)
	bruteForceAttempts = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "brute_force_attempts_total",
			Help: "Total number of brute force attempts by type",
		},
		[]string{"attempt_type", "target"},
	)
)

const (
	InvalidCredentialsError = "invalid credentials"
	WeakPasswordError       = "weak password"
	InvalidEmailError       = "invalid email"
	RateLimitExceededError  = "rate limit exceeded"
	InternalServerError     = "internal server error"
	UnauthorizedError       = "unauthorized"
	AccountLockedError      = "account temporarily locked"
	TooManyAttemptsError    = "too many attempts"
	CaptchaRequiredError    = "captcha required"
)

// Brute force protection configuration
const (
	MaxFailedAttemptsPerEmail = 5
	MaxFailedAttemptsPerIP    = 10
	LockoutDuration           = 15 * time.Minute  // Initial lockout
	MaxLockoutDuration        = 24 * time.Hour    // Maximum lockout
	CaptchaThreshold          = 3                 // Enable CAPTCHA after 3 failed attempts
	ProgressiveDelayBase      = 1 * time.Second   // Base delay for exponential backoff
)

// Attempt tracking structures
type LoginAttempt struct {
	FailedAttempts int           `json:"failed_attempts"`
	LastAttempt    time.Time     `json:"last_attempt"`
	LockedUntil    time.Time     `json:"locked_until"`
	IpAddress      string        `json:"ip_address"`
	Email          string        `json:"email"`
}

// Security event for logging
type SecurityEvent struct {
	EventType   string    `json:"event_type"`
	Email       string    `json:"email"`
	IPAddress   string    `json:"ip_address"`
	Reason      string    `json:"reason"`
	Timestamp   time.Time `json:"timestamp"`
	RiskLevel   string    `json:"risk_level"`
}

// === Brute Force Protection Functions ===

// Check if account is locked for email
func isAccountLocked(email string) (bool, time.Duration) {
	key := fmt.Sprintf("lockout:email:%s", email)
	lockedUntil, err := rdb.Get(context.Background(), key).Time()
	
	if err == nil && time.Now().Before(lockedUntil) {
		remaining := lockedUntil.Sub(time.Now())
		return true, remaining
	}

	return false, 0
}

// Check if IP is temporarily blocked
func isIPBlocked(ip string) bool {
	key := fmt.Sprintf("block:ip:%s", ip)
	blocked, err := rdb.Exists(context.Background(), key).Result()
	return err == nil && blocked > 0
}

// Record failed login attempt
func recordFailedAttempt(email, ip string) {
	now := time.Now()
	
	// Record attempt for email
	emailKey := fmt.Sprintf("attempts:email:%s", email)
	emailData := LoginAttempt{
		FailedAttempts: 1,
		LastAttempt:    now,
		IpAddress:      ip,
		Email:          email,
	}
	
	// Get existing attempts
	existingData, err := rdb.Get(context.Background(), emailKey).Result()
	if err == nil {
		var existing LoginAttempt
		if json.Unmarshal([]byte(existingData), &existing) == nil {
			emailData.FailedAttempts = existing.FailedAttempts + 1
			emailData.LockedUntil = existing.LockedUntil
		}
	}
	
	// Check if we need to lock the account
	if emailData.FailedAttempts >= MaxFailedAttemptsPerEmail {
		// Calculate lockout duration with exponential backoff
		lockoutDuration := LockoutDuration
		if emailData.FailedAttempts > MaxFailedAttemptsPerEmail {
			backoffCount := emailData.FailedAttempts - MaxFailedAttemptsPerEmail
			lockoutDuration = time.Duration(backoffCount) * LockoutDuration
			if lockoutDuration > MaxLockoutDuration {
				lockoutDuration = MaxLockoutDuration
			}
		}
		
		lockedUntil := now.Add(lockoutDuration)
		emailData.LockedUntil = lockedUntil
		
		// Set lockout in Redis
		rdb.Set(context.Background(), emailKey, lockedUntil.Unix(), 0)
		
		// Log security event
		logSecurityEvent("ACCOUNT_LOCKED", email, ip, fmt.Sprintf("Account locked for %v", lockoutDuration), "HIGH")
	} else {
		// Store attempt data
		data, _ := json.Marshal(emailData)
		rdb.SetEx(context.Background(), emailKey, data, 24*time.Hour)
	}
	
	// Record IP-based attempts
	ipKey := fmt.Sprintf("attempts:ip:%s", ip)
	ipAttempts, _ := rdb.Incr(context.Background(), ipKey).Result()
	rdb.Expire(context.Background(), ipKey, time.Hour)
	
	if ipAttempts >= MaxFailedAttemptsPerIP {
		// Block IP for 1 hour
		rdb.SetEx(context.Background(), fmt.Sprintf("block:ip:%s", ip), 1, time.Hour)
		logSecurityEvent("IP_BLOCKED", email, ip, "IP blocked due to multiple failed attempts", "MEDIUM")
	}
	
	// Log failed attempt
	logSecurityEvent("LOGIN_FAILED", email, ip, fmt.Sprintf("Failed attempt #%d", emailData.FailedAttempts), "LOW")
}

// Reset failed attempts on successful login
func resetFailedAttempts(email string) {
	emailKey := fmt.Sprintf("attempts:email:%s", email)
	rdb.Del(context.Background(), emailKey)
	
	// Remove lockout if exists
	lockoutKey := fmt.Sprintf("lockout:email:%s", email)
	rdb.Del(context.Background(), lockoutKey)
}

// Calculate progressive delay for failed attempts
func getProgressiveDelay(email string) time.Duration {
	emailKey := fmt.Sprintf("attempts:email:%s", email)
	data, err := rdb.Get(context.Background(), emailKey).Result()
	
	if err != nil {
		return 0
	}

	var attempt LoginAttempt
	if json.Unmarshal([]byte(data), &attempt) == nil {
		if attempt.FailedAttempts > 0 {
			// Exponential backoff: 1s, 2s, 4s, 8s, etc.
			delay := ProgressiveDelayBase * time.Duration(1<<uint(attempt.FailedAttempts-1))
			return time.Minute(1) + delay // Add minimum 1 minute delay
		}
	}
	
	return 0
}

// Log security event
func logSecurityEvent(eventType, email, ip, reason, riskLevel string) {
	event := SecurityEvent{
		EventType: eventType,
		Email:     email,
		IPAddress: ip,
		Reason:    reason,
		Timestamp: time.Now(),
		RiskLevel: riskLevel,
	}

	eventData, _ := json.Marshal(event)
	
	// Store in Redis for analytics
	rdb.LPush(context.Background(), "security:events", eventData)
	rdb.LTrim(context.Background(), "security:events", 0, 999) // Keep last 1000 events
	rdb.Expire(context.Background(), "security:events", 7*24*time.Hour) // Keep for 7 days

	// Increment Prometheus metrics
	securityEvents.WithLabelValues(eventType, riskLevel).Inc()
	
	// Increment brute force specific metrics
	if strings.Contains(strings.ToLower(eventType), "failed") || 
	   strings.Contains(strings.ToLower(eventType), "lock") ||
	   strings.Contains(strings.ToLower(eventType), "blocked") {
		bruteForceAttempts.WithLabelValues(eventType, email).Inc()
	}

	// Log based on risk level
	switch riskLevel {
	case "HIGH":
		logger.Warn().Str("event", eventType).Str("email", logSafeEmail(email)).Str("ip", ip).Str("reason", reason).Msg("High risk security event")
	case "MEDIUM":
		logger.Warn().Str("event", eventType).Str("email", logSafeEmail(email)).Str("ip", ip).Str("reason", reason).Msg("Medium risk security event")
	case "LOW":
		logger.Info().Str("event", eventType).Str("email", logSafeEmail(email)).Str("ip", ip).Str("reason", reason).Msg("Low risk security event")
	}
}

func init() {
	// Initialize structured logger
	logger = zerolog.New(os.Stdout).With().
		Timestamp().
		Str("service", "auth-service").
		Logger()

	// Register Prometheus metrics
	prometheus.MustRegister(authCounter, httpDuration, securityEvents, bruteForceAttempts)

	// Load JWT secret from environment
	secret = []byte(os.Getenv("JWT_SECRET"))
	if len(secret) == 0 {
		logger.Fatal().Msg("JWT_SECRET environment variable not set")
	}
	if len(secret) < 32 {
		logger.Fatal().Msg("JWT_SECRET must be at least 32 characters")
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

	r := chi.NewRouter()
	r.Use(securityHeadersMiddleware)
	r.Use(suspiciousActivityMiddleware)

	r.Post("/register", Register)
	r.With(rateLimitMiddleware).Post("/login", Login)
	r.With(AuthMiddleware).Get("/me", Me)
	r.With(AuthMiddleware).Get("/api-keys", ListAPIKeys)
	r.With(AuthMiddleware).Post("/api-keys", CreateAPIKey)
	r.With(AuthMiddleware).Get("/balance", GetBalance)

	// Security management endpoints
	r.Get("/security/status", SecurityStatus)
	r.With(AuthMiddleware).Post("/security/unlock", UnlockAccount)
	r.With(AuthMiddleware).Get("/security/events", GetSecurityEvents)
	r.With(AuthMiddleware).Post("/security/report", ReportSuspiciousActivity)

	// Health check endpoint
	r.Get("/health", HealthCheck)

	// Prometheus metrics endpoint
	r.Handle("/metrics", promhttp.Handler())

	// gRPC for gateway with mTLS
	go func() {
		grpcPort := os.Getenv("GRPC_PORT")
		if grpcPort == "" {
			grpcPort = "50051"
		}
		lis, err := net.Listen("tcp", ":"+grpcPort)
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

	httpPort := os.Getenv("HTTP_PORT")
	if httpPort == "" {
		httpPort = "8081"
	}
	logger.Info().Msgf("Auth service: HTTP :%s | gRPC+mTLS :%s", httpPort, os.Getenv("GRPC_PORT"))
	log.Fatal(http.ListenAndServe(":"+httpPort, r))
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
	ip := r.RemoteAddr
	logger.Info().Str("method", "Register").Str("ip", ip).Msg("Received registration request")

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

	// Check if IP is blocked (prevent spam registration)
	if isIPBlocked(ip) {
		logger.Warn().Str("ip", ip).Str("email", logSafeEmail(req.Email)).Msg("Registration attempt from blocked IP")
		http.Error(w, "IP temporarily blocked", 423)
		httpDuration.WithLabelValues("POST", "/register", "423").Observe(time.Since(start).Seconds())
		return
	}

	// Validate email
	if !isValidEmail(req.Email) {
		logger.Warn().Str("email", logSafeEmail(req.Email)).Str("ip", ip).Msg("Invalid email format")
		http.Error(w, InvalidEmailError, 400)
		httpDuration.WithLabelValues("POST", "/register", "400").Observe(time.Since(start).Seconds())
		return
	}

	// Validate password strength
	if !isStrongPassword(req.Password) {
		logger.Warn().Str("email", logSafeEmail(req.Email)).Str("ip", ip).Msg("Weak password attempt")
		http.Error(w, WeakPasswordError, 400)
		httpDuration.WithLabelValues("POST", "/register", "400").Observe(time.Since(start).Seconds())
		return
	}

	// Check if user already exists
	var existingUser User
	if err := db.Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
		logger.Warn().Str("email", logSafeEmail(req.Email)).Str("ip", ip).Msg("User already exists")
		
		// Log potential account enumeration attempt
		logSecurityEvent("ACCOUNT_ENUMERATION", req.Email, ip, "Registration attempt for existing user", "MEDIUM")
		
		http.Error(w, "user already exists", 409)
		httpDuration.WithLabelValues("POST", "/register", "409").Observe(time.Since(start).Seconds())
		return
	}

	// Check registration rate limiting per IP
	regKey := fmt.Sprintf("register:ip:%s", ip)
	regAttempts, _ := rdb.Incr(context.Background(), regKey).Result()
	rdb.Expire(context.Background(), regKey, time.Hour)
	
	if regAttempts > 10 { // Max 10 registrations per hour per IP
		logger.Warn().Str("ip", ip).Str("email", logSafeEmail(req.Email)).Int("attempts", int(regAttempts)).Msg("Registration rate limit exceeded")
		
		// Temporarily block IP for excessive registration attempts
		rdb.SetEx(context.Background(), fmt.Sprintf("block:ip:%s", ip), 1, time.Hour)
		logSecurityEvent("IP_BLOCKED", req.Email, ip, "Blocked for excessive registration attempts", "MEDIUM")
		
		http.Error(w, "too many registration attempts", 429)
		httpDuration.WithLabelValues("POST", "/register", "429").Observe(time.Since(start).Seconds())
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

	// Log successful registration
	logSecurityEvent("REGISTRATION_SUCCESS", req.Email, ip, "User registered successfully", "LOW")

	logger.Info().Str("user_id", user.ID).Str("email", logSafeEmail(req.Email)).Str("ip", ip).Msg("User registered successfully")
	httpDuration.WithLabelValues("POST", "/register", "200").Observe(time.Since(start).Seconds())
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "user_id": user.ID})
}

func Login(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	ip := r.RemoteAddr
	logger.Info().Str("method", "Login").Str("ip", ip).Msg("Received login request")

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

	// Check if account is locked
	if locked, remaining := isAccountLocked(req.Email); locked {
		logger.Warn().Str("email", logSafeEmail(req.Email)).Str("ip", ip).Dur("remaining", remaining).Msg("Login attempt on locked account")
		http.Error(w, fmt.Sprintf("%s (try again in %v)", AccountLockedError, remaining), 423)
		httpDuration.WithLabelValues("POST", "/login", "423").Observe(time.Since(start).Seconds())
		return
	}

	// Check if IP is blocked
	if isIPBlocked(ip) {
		logger.Warn().Str("ip", ip).Str("email", logSafeEmail(req.Email)).Msg("Login attempt from blocked IP")
		http.Error(w, "IP temporarily blocked", 423)
		httpDuration.WithLabelValues("POST", "/login", "423").Observe(time.Since(start).Seconds())
		return
	}

	// Apply progressive delay
	if delay := getProgressiveDelay(req.Email); delay > 0 {
		logger.Warn().Str("email", logSafeEmail(req.Email)).Str("ip", ip).Dur("delay", delay).Msg("Progressive delay applied")
		time.Sleep(delay)
	}

	// Check if CAPTCHA is required
	emailKey := fmt.Sprintf("attempts:email:%s", req.Email)
	attemptsData, _ := rdb.Get(context.Background(), emailKey).Result()
	if attemptsData != "" {
		var attempt LoginAttempt
		if json.Unmarshal([]byte(attemptsData), &attempt) == nil {
			if attempt.FailedAttempts >= CaptchaThreshold {
				logger.Warn().Str("email", logSafeEmail(req.Email)).Str("ip", ip).Int("attempts", attempt.FailedAttempts).Msg("CAPTCHA required")
				http.Error(w, CaptchaRequiredError, 429)
				httpDuration.WithLabelValues("POST", "/login", "429").Observe(time.Since(start).Seconds())
				return
			}
		}
	}

	var user User
	if err := db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		// User not found - still record attempt for security
		recordFailedAttempt(req.Email, ip)
		logger.Warn().Str("email", logSafeEmail(req.Email)).Str("ip", ip).Msg("Login attempt with non-existent email")
		http.Error(w, InvalidCredentialsError, 401)
		httpDuration.WithLabelValues("POST", "/login", "401").Observe(time.Since(start).Seconds())
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		// Record failed attempt
		recordFailedAttempt(req.Email, ip)
		
		logger.Warn().Str("email", logSafeEmail(req.Email)).Str("ip", ip).Msg("Invalid password attempt")
		http.Error(w, InvalidCredentialsError, 401)
		httpDuration.WithLabelValues("POST", "/login", "401").Observe(time.Since(start).Seconds())
		return
	}

	// Successful login - reset failed attempts
	resetFailedAttempts(req.Email)
	
	// Log successful login
	logSecurityEvent("LOGIN_SUCCESS", req.Email, ip, "Successful login", "LOW")

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

	logger.Info().Str("user_id", user.ID).Str("email", logSafeEmail(req.Email)).Str("ip", ip).Msg("User logged in successfully")
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

func maskAPIKey(key string) string {
	if len(key) < 8 {
		return "****"
	}
	return key[:4] + strings.Repeat("*", len(key)-8) + key[len(key)-4:]
}

func logSafeEmail(email string) string {
	if email == "" {
		return ""
	}
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "invalid-email"
	}
	username := parts[0]
	domain := parts[1]

	if len(username) <= 3 {
		return username + "@" + domain
	}
	return username[:2] + strings.Repeat("*", len(username)-2) + "@" + domain
}

// Suspicious activity detection middleware
func suspiciousActivityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ip := r.RemoteAddr
		userAgent := r.Header.Get("User-Agent")
		path := r.URL.Path

		// Skip health checks and metrics
		if path == "/health" || path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}

		// Track request patterns per IP
		patternKey := fmt.Sprintf("patterns:ip:%s", ip)
		
		// Check for rapid requests to different endpoints (potential scanning)
		requests, _ := rdb.Incr(context.Background(), fmt.Sprintf("scan:%s:%s", ip, path)).Result()
		rdb.Expire(context.Background(), fmt.Sprintf("scan:%s:%s", ip, path), time.Minute)
		
		if requests > 20 { // More than 20 requests to same endpoint per minute
			logSecurityEvent("SUSPICIOUS_SCANNING", "", ip, fmt.Sprintf("Rapid requests to %s (%d requests)", path, requests), "MEDIUM")
		}

		// Track unique endpoints accessed per minute
		endpointsKey := fmt.Sprintf("endpoints:%s", ip)
		rdb.SAdd(context.Background(), endpointsKey, path)
		rdb.Expire(context.Background(), endpointsKey, time.Minute)
		
		endpointCount, _ := rdb.SCard(context.Background(), endpointsKey).Result()
		if endpointCount > 10 { // Accessing more than 10 different endpoints per minute
			logSecurityEvent("SUSPICIOUS_PATTERNS", "", ip, fmt.Sprintf("Multiple endpoints accessed (%d endpoints)", endpointCount), "MEDIUM")
		}

		// Check for missing User-Agent (potential bot)
		if userAgent == "" && path != "/register" && path != "/login" {
			logSecurityEvent("MISSING_USER_AGENT", "", ip, fmt.Sprintf("Request to %s without User-Agent", path), "LOW")
		}

		// Track failed authentication attempts across all endpoints
		if strings.Contains(path, "/api-keys") || strings.Contains(path, "/balance") {
			// These endpoints require authentication
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				authFailKey := fmt.Sprintf("auth_fail:ip:%s", ip)
				failCount, _ := rdb.Incr(context.Background(), authFailKey).Result()
				rdb.Expire(context.Background(), authFailKey, time.Hour)
				
				if failCount > 5 {
					logSecurityEvent("MULTIPLE_AUTH_FAILURES", "", ip, fmt.Sprintf("Multiple auth failures (%d)", failCount), "MEDIUM")
				}
			}
		}

		next.ServeHTTP(w, r)
	})
}

// Security headers middleware
func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Content Security Policy
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:;")

		// XSS Protection
		w.Header().Set("X-XSS-Protection", "1; mode=block")

		// Clickjacking protection
		w.Header().Set("X-Frame-Options", "DENY")

		// MIME type sniffing protection
		w.Header().Set("X-Content-Type-Options", "nosniff")

		// Referrer Policy
		w.Header().Set("Referrer-Policy", "no-referrer")

		// Permissions Policy
		w.Header().Set("Permissions-Policy",
			"geolocation=(), microphone=(), camera=(), payment=()")

		// Strict Transport Security
		w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")

		next.ServeHTTP(w, r)
	})
}

func getUserAPIKeys(userID string) []map[string]interface{} {
	var keys []APIKey
	if err := db.Where("user_id = ?", userID).Find(&keys).Error; err != nil {
		logger.Error().Err(err).Str("user_id", userID).Msg("Failed to get API keys")
		return []map[string]interface{}{ }
	}

	var result []map[string]interface{}
	for _, k := range keys {
		result = append(result, map[string]interface{}{
			"id":      k.ID,
			"name":    k.Name,
			"key":     maskAPIKey(k.Key), // Mask the key
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
			userID, ok := claims["user_id"].(string)
			if !ok {
				logger.Warn().Msg("Invalid user_id in JWT token")
				http.Error(w, "invalid token", 401)
				httpDuration.WithLabelValues(r.Method, r.URL.Path, "401").Observe(time.Since(start).Seconds())
				return
			}
			if err := db.First(&user, "id = ?", userID).Error; err != nil {
				logger.Warn().Str("user_id", userID).Msg("User not found")
				http.Error(w, "user not found", 401)
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
	// Redis Lua script for atomic rate limiting
	luaScript := `
		local key = KEYS[1]
		local limit = tonumber(ARGV[1])
		local expire = tonumber(ARGV[2])
		
		local current = redis.call('GET', key)
		if current and tonumber(current) >= limit then
			return 0
		end
		
		local newCount = redis.call('INCR', key)
		if newCount == 1 then
			redis.call('EXPIRE', key, expire)
		end
		
		return newCount
	`

	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ip := r.RemoteAddr
		key := fmt.Sprintf("rate_limit:%s", ip)

		// Execute Lua script atomically
		result, err := rdb.Eval(context.Background(), luaScript, []string{key}, 5, 300).Result()
		if err != nil {
			logger.Error().Err(err).Msg("Failed to execute rate limit check")
			// Fail open - allow request if Redis is down
			next.ServeHTTP(w, r)
			return
		}

		if result.(int64) == 0 {
			logger.Warn().Str("ip", ip).Msg("Rate limit exceeded")
			http.Error(w, RateLimitExceededError, 429)
			httpDuration.WithLabelValues(r.Method, r.URL.Path, "429").Observe(time.Since(start).Seconds())
			return
		}

		next.ServeHTTP(w, r)
	}
}

func HealthCheck(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// Check database
	var result struct{}
	if err := db.Raw("SELECT 1").Scan(&result).Error; err != nil {
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
	// RFC 5322 compliant regex
	emailRegex := `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`
	matched, err := regexp.MatchString(emailRegex, email)
	return matched && err == nil
}

func isStrongPassword(password string) bool {
	// Enhanced password strength check
	if len(password) < 12 {
		return false
	}

	hasUpper := false
	hasLower := false
	hasNumber := false
	hasSpecial := false

	for _, char := range password {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsNumber(char):
			hasNumber = true
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSpecial = true
		}
	}

	return hasUpper && hasLower && hasNumber && hasSpecial
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

// === Security Management Endpoints ===

// SecurityStatus returns current security status for the user
func SecurityStatus(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	ip := r.RemoteAddr
	email := r.URL.Query().Get("email")
	
	if email == "" {
		http.Error(w, "email parameter required", 400)
		httpDuration.WithLabelValues("GET", "/security/status", "400").Observe(time.Since(start).Seconds())
		return
	}

	// Check account lock status
	locked, remaining := isAccountLocked(email)
	
	// Get failed attempts count
	emailKey := fmt.Sprintf("attempts:email:%s", email)
	attemptsData, _ := rdb.Get(context.Background(), emailKey).Result()
	
	var failedAttempts int
	var lastAttempt time.Time
	if attemptsData != "" {
		var attempt LoginAttempt
		if json.Unmarshal([]byte(attemptsData), &attempt) == nil {
			failedAttempts = attempt.FailedAttempts
			lastAttempt = attempt.LastAttempt
		}
	}

	// Check if CAPTCHA is required
	captchaRequired := failedAttempts >= CaptchaThreshold
	
	response := map[string]interface{}{
		"email":           logSafeEmail(email),
		"locked":          locked,
		"remaining_time":  remaining.Seconds(),
		"failed_attempts": failedAttempts,
		"last_attempt":    lastAttempt,
		"captcha_required": captchaRequired,
		"ip_blocked":      isIPBlocked(ip),
	}

	logger.Info().Str("email", logSafeEmail(email)).Str("ip", ip).Bool("locked", locked).Int("attempts", failedAttempts).Msg("Security status check")
	httpDuration.WithLabelValues("GET", "/security/status", "200").Observe(time.Since(start).Seconds())
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// UnlockAccount allows user to unlock their account (with verification)
func UnlockAccount(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	user := r.Context().Value("user").(User)
	ip := r.RemoteAddr

	var req struct {
		Email string `json:"email"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid input", 400)
		httpDuration.WithLabelValues("POST", "/security/unlock", "400").Observe(time.Since(start).Seconds())
		return
	}

	// Verify the email matches the authenticated user
	if req.Email != user.Email {
		logger.Warn().Str("user_id", user.ID).Str("email", logSafeEmail(req.Email)).Str("ip", ip).Msg("Unlock attempt with mismatched email")
		http.Error(w, "unauthorized", 403)
		httpDuration.WithLabelValues("POST", "/security/unlock", "403").Observe(time.Since(start).Seconds())
		return
	}

	// Check if account is actually locked
	if !isAccountLocked(req.Email) {
		logger.Info().Str("email", logSafeEmail(req.Email)).Str("ip", ip).Msg("Unlock requested for unlocked account")
		httpDuration.WithLabelValues("POST", "/security/unlock", "200").Observe(time.Since(start).Seconds())
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "already_unlocked"})
		return
	}

	// Reset failed attempts and remove lockout
	resetFailedAttempts(req.Email)
	
	// Log unlock event
	logSecurityEvent("ACCOUNT_UNLOCKED", req.Email, ip, "Account unlocked by user", "LOW")

	logger.Info().Str("email", logSafeEmail(req.Email)).Str("ip", ip).Msg("Account unlocked successfully")
	httpDuration.WithLabelValues("POST", "/security/unlock", "200").Observe(time.Since(start).Seconds())
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "unlocked"})
}

// GetSecurityEvents returns recent security events for the user
func GetSecurityEvents(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	user := r.Context().Value("user").(User)
	
	limit := 50 // Limit to last 50 events
	events, err := rdb.LRange(context.Background(), "security:events", 0, int64(limit-1)).Result()
	if err != nil {
		logger.Error().Err(err).Str("user_id", user.ID).Msg("Failed to get security events")
		http.Error(w, "failed to get events", 500)
		httpDuration.WithLabelValues("GET", "/security/events", "500").Observe(time.Since(start).Seconds())
		return
	}

	var userEvents []SecurityEvent
	for _, eventData := range events {
		var event SecurityEvent
		if json.Unmarshal([]byte(eventData), &event) == nil {
			// Filter events for this user
			if event.Email == user.Email {
				userEvents = append(userEvents, event)
			}
		}
	}

	logger.Info().Str("user_id", user.ID).Int("events_count", len(userEvents)).Msg("Security events retrieved")
	httpDuration.WithLabelValues("GET", "/security/events", "200").Observe(time.Since(start).Seconds())
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(userEvents)
}

// ReportSuspiciousActivity allows users to report suspicious activities
func ReportSuspiciousActivity(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	user := r.Context().Value("user").(User)
	ip := r.RemoteAddr

	var req struct {
		Type        string `json:"type"`
		Description string `json:"description"`
		Context     string `json:"context,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid input", 400)
		httpDuration.WithLabelValues("POST", "/security/report", "400").Observe(time.Since(start).Seconds())
		return
	}

	if req.Type == "" || req.Description == "" {
		http.Error(w, "type and description are required", 400)
		httpDuration.WithLabelValues("POST", "/security/report", "400").Observe(time.Since(start).Seconds())
		return
	}

	// Create security event for the report
	reportEvent := SecurityEvent{
		EventType: "USER_REPORT",
		Email:     user.Email,
		IPAddress: ip,
		Reason:    fmt.Sprintf("Type: %s, Description: %s, Context: %s", req.Type, req.Description, req.Context),
		Timestamp: time.Now(),
		RiskLevel: "MEDIUM",
	}

	// Store the report
	reportData, _ := json.Marshal(reportEvent)
	rdb.LPush(context.Background(), "security:reports", reportData)
	rdb.LTrim(context.Background(), "security:reports", 0, 99) // Keep last 100 reports
	rdb.Expire(context.Background(), "security:reports", 30*24*time.Hour) // Keep for 30 days

	// Log the report
	logger.Warn().Str("user_id", user.ID).Str("email", logSafeEmail(user.Email)).Str("type", req.Type).Str("ip", ip).Msg("Suspicious activity reported by user")
	
	// Also add to general security events
	eventData, _ := json.Marshal(reportEvent)
	rdb.LPush(context.Background(), "security:events", eventData)

	httpDuration.WithLabelValues("POST", "/security/report", "200").Observe(time.Since(start).Seconds())
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "reported"})
}
