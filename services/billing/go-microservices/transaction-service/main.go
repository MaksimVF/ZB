package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// Порт сервиса
const (
	Port = ":50054"
)

// Redis клиент
var rdb *redis.Client

// Метрики Prometheus
var (
	totalTransactions = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "billing_transactions_total",
			Help: "Total number of transactions",
		},
		[]string{"type", "status"},
	)

	transactionDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "billing_transaction_duration_seconds",
			Help: "Duration of transactions",
		},
		[]string{"type"},
	)

	activeReservations = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "billing_active_reservations",
			Help: "Number of active reservations",
		},
	)
)

func init() {
	// Инициализация Redis
	rdb = redis.NewClient(&redis.Options{
		Addr:     "redis:6379",
		Password: "",
		DB:       0,
	})

	// Регистрация метрик
	prometheus.MustRegister(totalTransactions)
	prometheus.MustRegister(transactionDuration)
	prometheus.MustRegister(activeReservations)
}

type TransactionService struct {
	UnimplementedTransactionServiceServer
}

func (s *TransactionService) RecordTransaction(ctx context.Context, req *TransactionRequest) (*TransactionResponse, error) {
	startTime := time.Now()

	// Валидация входных данных
	if err := s.validateTransactionRequest(req); err != nil {
		totalTransactions.WithLabelValues(req.Type, "error").Inc()
		return nil, err
	}

	// Создание записи транзакции
	transaction := Transaction{
		Id:        generateID(),
		UserId:    req.UserId,
		Type:      req.Type,
		Amount:    req.Amount,
		Currency:  req.Currency,
		Status:    req.Status,
		CreatedAt: time.Now(),
	}

	// Сохранение в Redis
	if err := s.saveTransaction(transaction); err != nil {
		totalTransactions.WithLabelValues(req.Type, "error").Inc()
		return nil, err
	}

	// Обновление баланса
	if err := s.updateBalance(req.UserId, req.Amount, req.Type); err != nil {
		totalTransactions.WithLabelValues(req.Type, "error").Inc()
		return nil, err
	}

	// Обновление метрик
	totalTransactions.WithLabelValues(req.Type, "success").Inc()
	transactionDuration.WithLabelValues(req.Type).Observe(time.Since(startTime).Seconds())

	return &TransactionResponse{
		TransactionId: transaction.Id,
		Status:        "success",
		Balance:       s.getBalance(req.UserId),
	}, nil
}

func (s *TransactionService) GetTransactions(ctx context.Context, req *TransactionsRequest) (*TransactionsResponse, error) {
	transactions, err := s.getUserTransactions(req.UserId)
	if err != nil {
		return nil, err
	}

	return &TransactionsResponse{
		Transactions: transactions,
		Count:        int32(len(transactions)),
	}, nil
}

func (s *TransactionService) validateTransactionRequest(req *TransactionRequest) error {
	if req.UserId == "" {
		return fmt.Errorf("user_id is required")
	}
	if req.Amount <= 0 {
		return fmt.Errorf("amount must be positive")
	}
	if req.Type == "" {
		return fmt.Errorf("transaction type is required")
	}
	return nil
}

func (s *TransactionService) saveTransaction(transaction Transaction) error {
	key := fmt.Sprintf("transaction:%s", transaction.Id)
	data, err := json.Marshal(transaction)
	if err != nil {
		return err
	}

	return rdb.Set(context.Background(), key, data, 0).Err()
}

func (s *TransactionService) updateBalance(userID string, amount float64, transactionType string) error {
	balanceKey := fmt.Sprintf("balance:%s", userID)

	// Получение текущего баланса
	currentBalance, err := rdb.Get(context.Background(), balanceKey).Float64()
	if err != nil && err != redis.Nil {
		return err
	}

	var newBalance float64
	switch transactionType {
	case "deposit", "refund":
		newBalance = currentBalance + amount
	case "charge", "commit":
		newBalance = currentBalance - amount
	case "adjustment":
		newBalance = currentBalance + amount // Может быть как положительным, так и отрицательным
	default:
		return fmt.Errorf("unknown transaction type: %s", transactionType)
	}

	return rdb.Set(context.Background(), balanceKey, newBalance, 0).Err()
}

func (s *TransactionService) getBalance(userID string) float64 {
	balance, err := rdb.Get(context.Background(), fmt.Sprintf("balance:%s", userID)).Float64()
	if err != nil {
		return 0
	}
	return balance
}

func (s *TransactionService) getUserTransactions(userID string) ([]*Transaction, error) {
	pattern := "transaction:*"
	keys, err := rdb.Keys(context.Background(), pattern).Result()
	if err != nil {
		return nil, err
	}

	var transactions []*Transaction
	for _, key := range keys {
		data, err := rdb.Get(context.Background(), key).Result()
		if err != nil {
			continue
		}

		var transaction Transaction
		if err := json.Unmarshal([]byte(data), &transaction); err != nil {
			continue
		}

		if transaction.UserId == userID {
			transactions = append(transactions, &transaction)
		}
	}

	return transactions, nil
}

func generateID() string {
	return fmt.Sprintf("txn_%d", time.Now().UnixNano())
}

func main() {
	// Настройка логирования
	logrus.SetFormatter(&logrus.JSONFormatter{})
	logrus.SetLevel(logrus.InfoLevel)

	// Создание gRPC сервера
	lis, err := net.Listen("tcp", Port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()

	// Регистрация сервиса
	RegisterTransactionServiceServer(grpcServer, &TransactionService{})

	// Включение reflection
	reflection.Register(grpcServer)

	// Запуск HTTP сервера для метрик
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		http.ListenAndServe(":9100", nil)
	}()

	logrus.Infof("Transaction Service started on port %s", Port)
	logrus.Info("Metrics available at http://localhost:9100/metrics")

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
