package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// BalanceRepository интерфейс для работы с балансами пользователей
type BalanceRepository interface {
	GetBalance(ctx context.Context, userID string) (*Balance, error)
	AdjustBalance(ctx context.Context, userID string, amount float64, reason string) (*Balance, error)
	ReserveBalance(ctx context.Context, userID string, amount float64, reservationID string) error
	CommitReservation(ctx context.Context, userID string, reservationID string, finalAmount float64) error
	CancelReservation(ctx context.Context, reservationID string) error
	GetReservation(ctx context.Context, reservationID string) (*Reservation, error)
	GetUserBalances(ctx context.Context, limit int) ([]*Balance, error)
}

// RedisBalanceRepository реализация на Redis
type RedisBalanceRepository struct {
	client *redis.Client
}

// Balance структура баланса
type Balance struct {
	UserID       string    `json:"user_id"`
	Balance      float64   `json:"balance"`
	Currency     string    `json:"currency"`
	UpdatedAt    time.Time `json:"updated_at"`
	TotalCharged float64   `json:"total_charged"`
	TotalSpent   float64   `json:"total_spent"`
}

// Reservation структура резерва
type Reservation struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	Amount     float64   `json:"amount"`
	ReservedAt time.Time `json:"reserved_at"`
	ExpiresAt  time.Time `json:"expires_at"`
	Status     string    `json:"status"` // pending, committed, cancelled
}

// NewRedisBalanceRepository создание нового репозитория
func NewRedisBalanceRepository(client *redis.Client) *RedisBalanceRepository {
	return &RedisBalanceRepository{
		client: client,
	}
}

// GetBalance получение баланса пользователя
func (r *RedisBalanceRepository) GetBalance(ctx context.Context, userID string) (*Balance, error) {
	key := fmt.Sprintf("balance:%s", userID)

	data, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return &Balance{
			UserID:    userID,
			Balance:   0.0,
			Currency:  "USD",
			UpdatedAt: time.Now(),
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get balance: %w", err)
	}

	var balance Balance
	if err := json.Unmarshal([]byte(data), &balance); err != nil {
		return nil, fmt.Errorf("failed to unmarshal balance: %w", err)
	}

	return &balance, nil
}

// AdjustBalance корректировка баланса
func (r *RedisBalanceRepository) AdjustBalance(ctx context.Context, userID string, amount float64, reason string) (*Balance, error) {
	balance, err := r.GetBalance(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get current balance: %w", err)
	}

	balance.Balance += amount
	balance.UpdatedAt = time.Now()

	// Обновляем статистику
	if amount > 0 {
		balance.TotalCharged += amount
	} else {
		balance.TotalSpent += -amount
	}

	// Сохраняем в Redis
	key := fmt.Sprintf("balance:%s", userID)
	data, err := json.Marshal(balance)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal balance: %w", err)
	}

	// Используем транзакцию для атомарности
	err = r.client.Watch(ctx, func(tx *redis.Tx) error {
		currentBalance, err := tx.Get(ctx, key).Result()
		if err != nil && err != redis.Nil {
			return err
		}

		// Проверяем, что баланс не стал отрицательным при списании
		if amount < 0 && currentBalance != "" {
			var current Balance
			if err := json.Unmarshal([]byte(currentBalance), &current); err != nil {
				return err
			}
			if current.Balance+amount < 0 {
				return fmt.Errorf("insufficient balance")
			}
		}

		_, err := tx.Set(ctx, key, string(data), 0).Result()
		return err
	}, key)

	if err != nil {
		return nil, fmt.Errorf("failed to update balance: %w", err)
	}

	// Логируем операцию
	r.logBalanceOperation(ctx, userID, amount, reason, balance.Balance)

	return balance, nil
}

// ReserveBalance резервирование средств
func (r *RedisBalanceRepository) ReserveBalance(ctx context.Context, userID string, amount float64, reservationID string) error {
	// Создаем резерв
	reservation := &Reservation{
		ID:         reservationID,
		UserID:     userID,
		Amount:     amount,
		ReservedAt: time.Now(),
		ExpiresAt:  time.Now().Add(24 * time.Hour), // Резерв действует 24 часа
		Status:     "pending",
	}

	reservationKey := fmt.Sprintf("reservation:%s", reservationID)
	reservationData, err := json.Marshal(reservation)
	if err != nil {
		return fmt.Errorf("failed to marshal reservation: %w", err)
	}

	// Используем транзакцию
	err = r.client.Watch(ctx, func(tx *redis.Tx) error {
		// Получаем текущий баланс
		currentBalance, err := tx.Get(ctx, fmt.Sprintf("balance:%s", userID)).Result()
		if err != nil && err != redis.Nil {
			return err
		}

		var current Balance
		if currentBalance != "" {
			if err := json.Unmarshal([]byte(currentBalance), &current); err != nil {
				return err
			}
			if current.Balance < amount {
				return fmt.Errorf("insufficient balance")
			}
		} else {
			// Если баланса нет, создаем новый с нулевым балансом
			return fmt.Errorf("insufficient balance")
		}

		// Сохраняем резерв
		_, err = tx.Set(ctx, reservationKey, string(reservationData), 24*time.Hour).Result()
		if err != nil {
			return err
		}

		// Обновляем баланс
		current.Balance -= amount
		current.UpdatedAt = time.Now()

		balanceData, err := json.Marshal(current)
		if err != nil {
			return err
		}

		_, err = tx.Set(ctx, fmt.Sprintf("balance:%s", userID), string(balanceData), 0).Result()
		return err
	}, fmt.Sprintf("balance:%s", userID), reservationKey)

	if err != nil {
		return fmt.Errorf("failed to reserve balance: %w", err)
	}

	return nil
}

// CommitReservation подтверждение резерва
func (r *RedisBalanceRepository) CommitReservation(ctx context.Context, userID string, reservationID string, finalAmount float64) error {
	reservation, err := r.GetReservation(ctx, reservationID)
	if err != nil {
		return fmt.Errorf("failed to get reservation: %w", err)
	}

	if reservation.Status != "pending" {
		return fmt.Errorf("reservation already processed")
	}

	if time.Now().After(reservation.ExpiresAt) {
		return fmt.Errorf("reservation expired")
	}

	reservation.Status = "committed"

	// Обновляем резерв и баланс в транзакции
	reservationKey := fmt.Sprintf("reservation:%s", reservationID)
	balanceKey := fmt.Sprintf("balance:%s", userID)

	reservationData, _ := json.Marshal(reservation)

	err = r.client.Watch(ctx, func(tx *redis.Tx) error {
		// Если финальная сумма меньше зарезервированной, возвращаем разницу
		if finalAmount < reservation.Amount {
			difference := reservation.Amount - finalAmount

			// Получаем текущий баланс
			currentBalance, err := tx.Get(ctx, balanceKey).Result()
			if err != nil && err != redis.Nil {
				return err
			}

			var balance Balance
			if currentBalance != "" {
				if err := json.Unmarshal([]byte(currentBalance), &balance); err != nil {
					return err
				}
			} else {
				balance = Balance{
					UserID:    userID,
					Balance:   0.0,
					Currency:  "USD",
					UpdatedAt: time.Now(),
				}
			}

			balance.Balance += difference
			balance.UpdatedAt = time.Now()

			balanceData, _ := json.Marshal(balance)
			_, err = tx.Set(ctx, balanceKey, string(balanceData), 0).Result()
			if err != nil {
				return err
			}
		}

		// Обновляем статус резерва
		_, err = tx.Set(ctx, reservationKey, string(reservationData), 24*time.Hour).Result()
		return err
	}, balanceKey, reservationKey)

	if err != nil {
		return fmt.Errorf("failed to commit reservation: %w", err)
	}

	return nil
}

// CancelReservation отмена резерва
func (r *RedisBalanceRepository) CancelReservation(ctx context.Context, reservationID string) error {
	reservation, err := r.GetReservation(ctx, reservationID)
	if err != nil {
		return fmt.Errorf("failed to get reservation: %w", err)
	}

	if reservation.Status != "pending" {
		return fmt.Errorf("reservation already processed")
	}

	// Возвращаем средства
	balance, err := r.GetBalance(ctx, reservation.UserID)
	if err != nil {
		return fmt.Errorf("failed to get balance: %w", err)
	}

	balance.Balance += reservation.Amount
	balance.UpdatedAt = time.Now()

	// Обновляем статус резерва
	reservation.Status = "cancelled"

	// Сохраняем изменения в транзакции
	reservationKey := fmt.Sprintf("reservation:%s", reservationID)
	balanceKey := fmt.Sprintf("balance:%s", reservation.UserID)

	reservationData, _ := json.Marshal(reservation)
	balanceData, _ := json.Marshal(balance)

	err = r.client.Watch(ctx, func(tx *redis.Tx) error {
		_, err := tx.Set(ctx, reservationKey, string(reservationData), 24*time.Hour).Result()
		if err != nil {
			return err
		}
		_, err = tx.Set(ctx, balanceKey, string(balanceData), 0).Result()
		return err
	}, reservationKey, balanceKey)

	if err != nil {
		return fmt.Errorf("failed to cancel reservation: %w", err)
	}

	return nil
}

// GetReservation получение резерва
func (r *RedisBalanceRepository) GetReservation(ctx context.Context, reservationID string) (*Reservation, error) {
	key := fmt.Sprintf("reservation:%s", reservationID)

	data, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("reservation not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get reservation: %w", err)
	}

	var reservation Reservation
	if err := json.Unmarshal([]byte(data), &reservation); err != nil {
		return nil, fmt.Errorf("failed to unmarshal reservation: %w", err)
	}

	return &reservation, nil
}

// GetUserBalances получение балансов пользователей (для админки)
func (r *RedisBalanceRepository) GetUserBalances(ctx context.Context, limit int) ([]*Balance, error) {
	pattern := "balance:*"

	keys, err := r.client.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get balance keys: %w", err)
	}

	// Ограничиваем количество
	if limit > 0 && len(keys) > limit {
		keys = keys[:limit]
	}

	balances := make([]*Balance, 0, len(keys))

	for _, key := range keys {
		data, err := r.client.Get(ctx, key).Result()
		if err != nil {
			continue
		}

		var balance Balance
		if err := json.Unmarshal([]byte(data), &balance); err != nil {
			continue
		}

		balances = append(balances, &balance)
	}

	return balances, nil
}

// logBalanceOperation логирование операций с балансом
func (r *RedisBalanceRepository) logBalanceOperation(ctx context.Context, userID string, amount float64, reason string, newBalance float64) {
	operation := map[string]interface{}{
		"user_id":     userID,
		"amount":      amount,
		"reason":      reason,
		"new_balance": newBalance,
		"timestamp":   time.Now().Unix(),
		"type":        "balance_operation",
	}

	data, _ := json.Marshal(operation)
	r.client.LPush(ctx, "balance:operations", string(data))
	r.client.LTrim(ctx, "balance:operations", 0, 999) // Храним только последние 1000 операций
}
