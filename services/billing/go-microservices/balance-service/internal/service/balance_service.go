package service

import (
	"context"
	"fmt"
	"time"

	"balance-service/internal/repository"
)

// BalanceService сервис для работы с балансами
type BalanceService struct {
	balanceRepo repository.BalanceRepository
	adminKey    string
}

// AdjustBalanceRequest структура запроса на корректировку баланса
type AdjustBalanceRequest struct {
	UserID          string  `json:"user_id"`
	Amount          float64 `json:"amount"` // Positive for credit, negative for debit
	Reason          string  `json:"reason"`
	ForceAdjustment bool    `json:"force_adjustment,omitempty"`
}

// NewBalanceService создает новый экземпляр BalanceService
func NewBalanceService(balanceRepo repository.BalanceRepository) *BalanceService {
	return &BalanceService{
		balanceRepo: balanceRepo,
		adminKey:    "", // Будет загружен из конфигурации
	}
}

// SetAdminKey устанавливает ключ администратора
func (s *BalanceService) SetAdminKey(adminKey string) {
	s.adminKey = adminKey
}

// ValidateAdminKey проверяет админский ключ
func (s *BalanceService) ValidateAdminKey(adminKey string) bool {
	return adminKey == s.adminKey
}

// GetBalance получает баланс пользователя
func (s *BalanceService) GetBalance(ctx context.Context, userID string) (*repository.Balance, error) {
	// Валидация userID
	if !isValidUserID(userID) {
		return nil, fmt.Errorf("invalid user ID: %s", userID)
	}

	// Получение баланса из репозитория
	balance, err := s.balanceRepo.GetBalance(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get balance: %w", err)
	}

	return balance, nil
}

// AdjustBalance корректирует баланс пользователя
func (s *BalanceService) AdjustBalance(ctx context.Context, req AdjustBalanceRequest) (*repository.Balance, error) {
	// Валидация входных данных
	if !isValidUserID(req.UserID) {
		return nil, fmt.Errorf("invalid user ID: %s", req.UserID)
	}

	if !isValidAmount(req.Amount) {
		return nil, fmt.Errorf("invalid amount: %f", req.Amount)
	}

	if !isValidReason(req.Reason) {
		return nil, fmt.Errorf("invalid reason: %s", req.Reason)
	}

	// Проверка админских прав для отрицательных корректировок
	if req.Amount < 0 && !req.ForceAdjustment {
		currentBalance, err := s.balanceRepo.GetBalance(ctx, req.UserID)
		if err != nil {
			return nil, fmt.Errorf("failed to get current balance: %w", err)
		}

		if currentBalance.Balance+req.Amount < 0 {
			return nil, fmt.Errorf("insufficient funds: current %.2f, required %.2f",
				currentBalance.Balance, -req.Amount)
		}
	}

	// Корректировка баланса через repository
	balance, err := s.balanceRepo.AdjustBalance(ctx, req.UserID, req.Amount, req.Reason)
	if err != nil {
		return nil, fmt.Errorf("failed to adjust balance: %w", err)
	}

	return balance, nil
}

// ReserveBalance резервирует средства на балансе
func (s *BalanceService) ReserveBalance(ctx context.Context, userID string, amount float64, reservationID string) error {
	if !isValidUserID(userID) {
		return fmt.Errorf("invalid user ID: %s", userID)
	}

	if !isValidAmount(amount) {
		return fmt.Errorf("invalid amount: %f", amount)
	}

	if reservationID == "" {
		return fmt.Errorf("reservation ID is required")
	}

	return s.balanceRepo.ReserveBalance(ctx, userID, amount, reservationID)
}

// CommitReservation подтверждает резерв
func (s *BalanceService) CommitReservation(ctx context.Context, userID string, reservationID string, finalAmount float64) error {
	if !isValidUserID(userID) {
		return fmt.Errorf("invalid user ID: %s", userID)
	}

	if reservationID == "" {
		return fmt.Errorf("reservation ID is required")
	}

	if !isValidAmount(finalAmount) {
		return fmt.Errorf("invalid final amount: %f", finalAmount)
	}

	return s.balanceRepo.CommitReservation(ctx, userID, reservationID, finalAmount)
}

// CancelReservation отменяет резерв
func (s *BalanceService) CancelReservation(ctx context.Context, reservationID string) error {
	if reservationID == "" {
		return fmt.Errorf("reservation ID is required")
	}

	return s.balanceRepo.CancelReservation(ctx, reservationID)
}

// GetReservation получает информацию о резерве
func (s *BalanceService) GetReservation(ctx context.Context, reservationID string) (*repository.Reservation, error) {
	if reservationID == "" {
		return nil, fmt.Errorf("reservation ID is required")
	}

	return s.balanceRepo.GetReservation(ctx, reservationID)
}

// GetUserBalances получает балансы пользователей (для админки)
func (s *BalanceService) GetUserBalances(ctx context.Context, limit int) ([]*repository.Balance, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	return s.balanceRepo.GetUserBalances(ctx, limit)
}

// IsHealthy проверяет здоровье сервиса
func (s *BalanceService) IsHealthy() bool {
	// Простая проверка - пытаемся получить баланс тестового пользователя
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := s.balanceRepo.GetBalance(ctx, "health-check-user")
	return err == nil || err.Error() != "redis: nil"
}

// Валидационные функции
func isValidUserID(userID string) bool {
	if userID == "" || len(userID) > 100 {
		return false
	}
	return true
}

func isValidAmount(amount float64) bool {
	return amount > -1e15 && amount < 1e15
}

func isValidReason(reason string) bool {
	return reason != "" && len(reason) <= 500
}
