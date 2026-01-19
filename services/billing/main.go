

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"billing-service/gen"
)

const (
	DefaultPort     = 50052
	DefaultRedisURL = "localhost:6379"
	DefaultEnv      = "development"
)

type Config struct {
	Port        int
	RedisURL    string
	Env         string
	ExternalURL string
}

type BillingService struct {
	redis  *redis.Client
	config Config
	logger *logrus.Logger
	gen.UnimplementedBillingServiceServer
}

func NewBillingService(config Config) *BillingService {
	// Initialize logger
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	logger.SetLevel(logrus.InfoLevel)

	// Initialize Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr: config.RedisURL,
	})

	return &BillingService{
		redis:  redisClient,
		config: config,
		logger: logger,
	}
}

func (s *BillingService) Charge(ctx context.Context, req *gen.BillRequest) (*gen.BillResponse, error) {
	startTime := time.Now()
	defer func() {
		processingTime.WithLabelValues("charge").Observe(time.Since(startTime).Seconds())
	}()

	// Validate request
	if req.UserId == "" {
		chargeRequests.WithLabelValues("false").Inc()
		return nil, status.Errorf(codes.InvalidArgument, "user_id is required")
	}

	if req.Cost <= 0 {
		chargeRequests.WithLabelValues("false").Inc()
		return nil, status.Errorf(codes.InvalidArgument, "cost must be positive")
	}

	// Get current balance
	balanceKey := fmt.Sprintf("user:%s:balance", req.UserId)
	balanceStr, err := s.redis.Get(ctx, balanceKey).Result()
	if err != nil && err != redis.Nil {
		return nil, status.Errorf(codes.Internal, "failed to get user balance: %v", err)
	}

	var currentBalance float64
	if balanceStr == "" {
		currentBalance = 0.0
	} else {
		currentBalance, err = strconv.ParseFloat(balanceStr, 64)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to parse balance: %v", err)
		}
	}

	// Check if user has sufficient funds
	if currentBalance < req.Cost {
		return &gen.BillResponse{
			Success: false,
			Error:   "insufficient funds",
		}, nil
	}

	// Deduct the cost
	newBalance := currentBalance - req.Cost

	// Update balance in Redis
	err = s.redis.Set(ctx, balanceKey, newBalance, 0).Err()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update balance: %v", err)
	}

	// Record transaction
	transaction := map[string]interface{}{
		"user_id":      req.UserId,
		"request_id":   req.RequestId,
		"model":        req.Model,
		"tokens_used":  req.TokensUsed,
		"cost":         req.Cost,
		"new_balance":  newBalance,
		"timestamp":    time.Now().Unix(),
		"transaction_type": "charge",
	}

	transactionJSON, err := json.Marshal(transaction)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to marshal transaction: %v", err)
	}

	transactionKey := fmt.Sprintf("transaction:%s:%s", req.UserId, req.RequestId)
	err = s.redis.Set(ctx, transactionKey, transactionJSON, 0).Err()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to record transaction: %v", err)
	}

	s.logger.WithFields(logrus.Fields{
		"user_id":     req.UserId,
		"request_id":  req.RequestId,
		"model":       req.Model,
		"cost":        req.Cost,
		"new_balance": newBalance,
	}).Info("Charge successful")

	return &gen.BillResponse{
		Success:    true,
		NewBalance: newBalance,
	}, nil
}

func (s *BillingService) Reserve(ctx context.Context, req *gen.ReserveRequest) (*gen.ReserveResponse, error) {
	// Validate request
	if req.UserId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "user_id is required")
	}

	if req.Model == "" {
		return nil, status.Errorf(codes.InvalidArgument, "model is required")
	}

	if req.Endpoint == "" {
		return nil, status.Errorf(codes.InvalidArgument, "endpoint is required")
	}

	// Get current balance
	balanceKey := fmt.Sprintf("user:%s:balance", req.UserId)
	balanceStr, err := s.redis.Get(ctx, balanceKey).Result()
	if err != nil && err != redis.Nil {
		return nil, status.Errorf(codes.Internal, "failed to get user balance: %v", err)
	}

	var currentBalance float64
	if balanceStr == "" {
		currentBalance = 0.0
	} else {
		currentBalance, err = strconv.ParseFloat(balanceStr, 64)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to parse balance: %v", err)
		}
	}

	// Calculate estimated cost (this would use pricing service in production)
	// For now, we'll use a simple calculation based on token estimates
	estimatedCost := calculateEstimatedCost(req.Model, req.Endpoint, req.InputTokensEstimate, req.OutputTokensEstimate)

	// Check if user has sufficient funds
	if currentBalance < estimatedCost {
		return &gen.ReserveResponse{
			Success: false,
			Error:   "insufficient funds for reservation",
		}, nil
	}

	// Create reservation ID
	reservationID := fmt.Sprintf("res:%s:%s:%d", req.UserId, req.Model, time.Now().UnixNano())

	// Create reservation record
	reservation := map[string]interface{}{
		"user_id":              req.UserId,
		"model":                req.Model,
		"endpoint":             req.Endpoint,
		"input_tokens_estimate": req.InputTokensEstimate,
		"output_tokens_estimate": req.OutputTokensEstimate,
		"reserved_amount":      estimatedCost,
		"status":               "reserved",
		"created_at":           time.Now().Unix(),
		"expires_at":           time.Now().Add(15 * time.Minute).Unix(), // 15 minute reservation
	}

	reservationJSON, err := json.Marshal(reservation)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to marshal reservation: %v", err)
	}

	// Store reservation in Redis
	reservationKey := fmt.Sprintf("reservation:%s", reservationID)
	err = s.redis.Set(ctx, reservationKey, reservationJSON, 15*time.Minute).Err()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to store reservation: %v", err)
	}

	// Update user's reserved amount
	reservedKey := fmt.Sprintf("user:%s:reserved", req.UserId)
	err = s.redis.IncrByFloat(ctx, reservedKey, estimatedCost).Err()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update reserved amount: %v", err)
	}

	s.logger.WithFields(logrus.Fields{
		"user_id":         req.UserId,
		"reservation_id":  reservationID,
		"model":           req.Model,
		"estimated_cost":  estimatedCost,
		"remaining_balance": currentBalance - estimatedCost,
	}).Info("Reservation created")

	return &gen.ReserveResponse{
		Success:         true,
		ReservationId:   reservationID,
		ReservedAmount: estimatedCost,
		RemainingBalance: currentBalance - estimatedCost,
	}, nil
}

func (s *BillingService) Commit(ctx context.Context, req *gen.CommitRequest) (*gen.CommitResponse, error) {
	// Validate request
	if req.ReservationId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "reservation_id is required")
	}

	// Get reservation
	reservationKey := fmt.Sprintf("reservation:%s", req.ReservationId)
	reservationJSON, err := s.redis.Get(ctx, reservationKey).Result()
	if err != nil {
		if err == redis.Nil {
			return &gen.CommitResponse{
				Success: false,
				Error:   "reservation not found or expired",
			}, nil
		}
		return nil, status.Errorf(codes.Internal, "failed to get reservation: %v", err)
	}

	var reservation map[string]interface{}
	err = json.Unmarshal([]byte(reservationJSON), &reservation)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to unmarshal reservation: %v", err)
	}

	// Check reservation status
	if status, ok := reservation["status"].(string); !ok || status != "reserved" {
		return &gen.CommitResponse{
			Success: false,
			Error:   "reservation is not in reserved state",
		}, nil
	}

	userID, ok := reservation["user_id"].(string)
	if !ok {
		return nil, status.Errorf(codes.Internal, "invalid reservation data: missing user_id")
	}

	reservedAmount, ok := reservation["reserved_amount"].(float64)
	if !ok {
		return nil, status.Errorf(codes.Internal, "invalid reservation data: missing reserved_amount")
	}

	// Calculate actual cost based on actual token usage
	actualCost := calculateActualCost(
		reservation["model"].(string),
		reservation["endpoint"].(string),
		req.InputTokensActual,
		req.OutputTokensActual,
	)

	// Update reservation status
	reservation["status"] = "completed"
	reservation["input_tokens_actual"] = req.InputTokensActual
	reservation["output_tokens_actual"] = req.OutputTokensActual
	reservation["final_cost"] = actualCost
	reservation["completed_at"] = time.Now().Unix()

	updatedReservationJSON, err := json.Marshal(reservation)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to marshal updated reservation: %v", err)
	}

	// Update reservation in Redis (no expiration for completed reservations)
	err = s.redis.Set(ctx, reservationKey, updatedReservationJSON, 0).Err()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update reservation: %v", err)
	}

	// Get current balance
	balanceKey := fmt.Sprintf("user:%s:balance", userID)
	balanceStr, err := s.redis.Get(ctx, balanceKey).Result()
	if err != nil && err != redis.Nil {
		return nil, status.Errorf(codes.Internal, "failed to get user balance: %v", err)
	}

	var currentBalance float64
	if balanceStr == "" {
		currentBalance = 0.0
	} else {
		currentBalance, err = strconv.ParseFloat(balanceStr, 64)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to parse balance: %v", err)
		}
	}

	// Update balance: deduct actual cost, refund any over-reserved amount
	newBalance := currentBalance - actualCost

	// Update balance in Redis
	err = s.redis.Set(ctx, balanceKey, newBalance, 0).Err()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update balance: %v", err)
	}

	// Update reserved amount (decrement by reserved amount)
	reservedKey := fmt.Sprintf("user:%s:reserved", userID)
	err = s.redis.IncrByFloat(ctx, reservedKey, -reservedAmount).Err()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update reserved amount: %v", err)
	}

	s.logger.WithFields(logrus.Fields{
		"user_id":         userID,
		"reservation_id":  req.ReservationId,
		"final_cost":      actualCost,
		"remaining_balance": newBalance,
	}).Info("Reservation committed")

	return &gen.CommitResponse{
		Success:          true,
		FinalCost:        actualCost,
		RemainingBalance: newBalance,
	}, nil
}

func (s *BillingService) GetBalance(ctx context.Context, req *gen.GetBalanceRequest) (*gen.GetBalanceResponse, error) {
	// Validate request
	if req.UserId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "user_id is required")
	}

	// Get balance in USD
	balanceKey := fmt.Sprintf("user:%s:balance", req.UserId)
	balanceStr, err := s.redis.Get(ctx, balanceKey).Result()
	if err != nil && err != redis.Nil {
		return nil, status.Errorf(codes.Internal, "failed to get user balance: %v", err)
	}

	var balanceUSD float64
	if balanceStr == "" {
		balanceUSD = 0.0
	} else {
		balanceUSD, err = strconv.ParseFloat(balanceStr, 64)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to parse balance: %v", err)
		}
	}

	// For now, we'll return the same balance for all currencies
	// In a production system, this would use exchange rate service
	return &gen.GetBalanceResponse{
		BalanceUsd: balanceUSD,
		BalanceRub: balanceUSD * 90, // Approximate conversion
		BalanceEur: balanceUSD * 0.9, // Approximate conversion
	}, nil
}

func (s *BillingService) AdjustBalance(ctx context.Context, req *gen.AdjustBalanceRequest) (*gen.AdjustBalanceResponse, error) {
	// Validate request
	if req.UserId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "user_id is required")
	}

	if req.Reason == "" {
		return nil, status.Errorf(codes.InvalidArgument, "reason is required")
	}

	// Get current balance
	balanceKey := fmt.Sprintf("user:%s:balance", req.UserId)
	balanceStr, err := s.redis.Get(ctx, balanceKey).Result()
	if err != nil && err != redis.Nil {
		return nil, status.Errorf(codes.Internal, "failed to get user balance: %v", err)
	}

	var currentBalance float64
	if balanceStr == "" {
		currentBalance = 0.0
	} else {
		currentBalance, err = strconv.ParseFloat(balanceStr, 64)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to parse balance: %v", err)
		}
	}

	// Apply adjustment
	newBalance := currentBalance + req.AmountUsd

	// Update balance in Redis
	err = s.redis.Set(ctx, balanceKey, newBalance, 0).Err()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update balance: %v", err)
	}

	// Record adjustment
	adjustment := map[string]interface{}{
		"user_id":     req.UserId,
		"amount":      req.AmountUsd,
		"reason":      req.Reason,
		"old_balance": currentBalance,
		"new_balance": newBalance,
		"timestamp":   time.Now().Unix(),
	}

	adjustmentJSON, err := json.Marshal(adjustment)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to marshal adjustment: %v", err)
	}

	adjustmentKey := fmt.Sprintf("adjustment:%s:%d", req.UserId, time.Now().UnixNano())
	err = s.redis.Set(ctx, adjustmentKey, adjustmentJSON, 0).Err()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to record adjustment: %v", err)
	}

	s.logger.WithFields(logrus.Fields{
		"user_id":     req.UserId,
		"amount":      req.AmountUsd,
		"reason":      req.Reason,
		"new_balance": newBalance,
	}).Info("Balance adjusted")

	return &gen.AdjustBalanceResponse{
		Success:        true,
		NewBalanceUsd: newBalance,
	}, nil
}

// Helper functions

func calculateEstimatedCost(model, endpoint string, inputTokens, outputTokens int32) float64 {
	// Simple pricing model - this would be replaced with actual pricing service calls
	// Pricing per 1K tokens
	pricing := map[string]map[string]float64{
		"gpt-4o": {
			"chat":  0.005,
			"embed": 0.0001,
		},
		"gpt-4o-mini": {
			"chat":  0.00015,
			"embed": 0.00001,
		},
		"claude-3-sonnet": {
			"chat":  0.003,
			"embed": 0.0003,
		},
	}

	if prices, exists := pricing[model]; exists {
		if price, exists := prices[endpoint]; exists {
			totalTokens := float64(inputTokens + outputTokens)
			return (totalTokens / 1000) * price
		}
	}

	// Default pricing if model/endpoint not found
	return 0.001
}

func calculateActualCost(model, endpoint string, inputTokens, outputTokens int32) float64 {
	// Same as estimated cost for now
	return calculateEstimatedCost(model, endpoint, inputTokens, outputTokens)
}

func main() {
	// Initialize metrics
	initMetrics()

	// Load configuration
	config := Config{
		Port:     getEnvInt("PORT", DefaultPort),
		RedisURL: getEnv("REDIS_URL", DefaultRedisURL),
		Env:      getEnv("ENV", DefaultEnv),
	}

	// Create billing service
	service := NewBillingService(config)

	// Create gRPC server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", config.Port))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()

	// Register billing service
	gen.RegisterBillingServiceServer(grpcServer, service)

	log.Printf("Billing Service starting on port %d", config.Port)
	log.Printf("Environment: %s", config.Env)
	log.Printf("Redis URL: %s", config.RedisURL)

	// Start metrics server
	StartMetricsServer(9090)

	// Graceful shutdown
	go func() {
		<-shutdown()
		log.Println("Shutting down Billing Service...")
		grpcServer.GracefulStop()
	}()

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}

// Helper functions for configuration

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

// graceful shutdown handling
func shutdown() <-chan struct{} {
	shutdown := make(chan struct{})

	go func() {
		defer close(shutdown)
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
	}()

	return shutdown
}

