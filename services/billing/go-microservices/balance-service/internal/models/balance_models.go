package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"
)

// Currency represents supported currencies
type Currency string

const (
	USD Currency = "USD"
	RUB Currency = "RUB"
	EUR Currency = "EUR"
)

// SupportedCurrencies returns list of supported currencies
func SupportedCurrencies() []Currency {
	return []Currency{USD, RUB, EUR}
}

// Money represents money in cents
type Money struct {
	Amount   *big.Int `json:"amount"`   // Amount in cents
	Currency Currency `json:"currency"` // Currency code
}

// NewMoney creates a new Money instance
func NewMoney(amount *big.Int, currency Currency) *Money {
	return &Money{
		Amount:   amount,
		Currency: currency,
	}
}

// String returns a string representation of the money
func (m *Money) String() string {
	if m == nil || m.Amount == nil {
		return "0.00"
	}

	// Convert cents to dollars
	amount := m.Amount.Int64()
	dollars := amount / 100
	cents := amount % 100

	return fmt.Sprintf("%d.%02d %s", dollars, cents, m.Currency)
}

// ToFloat converts money to float64
func (m *Money) ToFloat() float64 {
	if m == nil || m.Amount == nil {
		return 0.0
	}

	amount := m.Amount.Int64()
	return float64(amount) / 100.0
}

// MarshalJSON implements custom JSON marshaling
func (m *Money) MarshalJSON() ([]byte, error) {
	if m == nil {
		return []byte("null"), nil
	}

	return json.Marshal(map[string]interface{}{
		"amount":           m.Amount.String(),
		"currency":         m.Currency,
		"amount_formatted": m.String(),
		"amount_decimal":   m.ToFloat(),
	})
}

// UnmarshalJSON implements custom JSON unmarshaling
func (m *Money) UnmarshalJSON(data []byte) error {
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	// Try to unmarshal as string format "amount currency"
	if str, ok := raw["amount_formatted"]; ok {
		if strVal, ok := str.(string); ok {
			parts := strings.Split(strVal, " ")
			if len(parts) >= 2 {
				amountStr := parts[0]
				currency := Currency(parts[len(parts)-1])

				// Parse amount string
				amountFloat, err := strconv.ParseFloat(amountStr, 64)
				if err != nil {
					return err
				}

				amount := new(big.Int)
				amount.SetString(fmt.Sprintf("%.0f", amountFloat*100), 10)

				*m = Money{Amount: amount, Currency: currency}
				return nil
			}
		}
	}

	return nil
}

// Value implements driver.Valuer for database
func (m *Money) Value() (driver.Value, error) {
	if m == nil {
		return nil, nil
	}
	return json.Marshal(m)
}

// Scan implements sql.Scanner for database
func (m *Money) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("invalid value type for Money")
	}

	return json.Unmarshal(bytes, m)
}

// Balance represents user balance
type Balance struct {
	ID        int       `json:"id"`
	UserID    string    `json:"user_id"`
	Amount    *big.Int  `json:"amount"` // Amount in cents
	Currency  Currency  `json:"currency"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// GetAmountFloat returns balance amount as float64
func (b *Balance) GetAmountFloat() float64 {
	if b == nil || b.Amount == nil {
		return 0.0
	}
	return float64(b.Amount.Int64()) / 100.0
}

// GetAmountFormatted returns formatted amount string
func (b *Balance) GetAmountFormatted() string {
	if b == nil || b.Amount == nil {
		return "0.00"
	}

	amount := b.Amount.Int64()
	dollars := amount / 100
	cents := amount % 100

	return fmt.Sprintf("%d.%02d", dollars, cents)
}

// BalanceAdjustmentRequest represents a balance adjustment request
type BalanceAdjustmentRequest struct {
	UserID          string                 `json:"user_id"`
	Amount          float64                `json:"amount"` // Positive for credit, negative for debit
	Currency        Currency               `json:"currency"`
	Reason          string                 `json:"reason"`
	ForceAdjustment bool                   `json:"force_adjustment,omitempty"` // Allow negative balance
	ReferenceID     string                 `json:"reference_id,omitempty"`     // External reference ID
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// BalanceAdjustmentResponse represents a balance adjustment response
type BalanceAdjustmentResponse struct {
	Success       bool   `json:"success"`
	NewBalance    *Money `json:"new_balance,omitempty"`
	Error         string `json:"error,omitempty"`
	TransactionID string `json:"transaction_id,omitempty"`
}

// Transaction represents a balance transaction
type Transaction struct {
	ID          int                    `json:"id"`
	UserID      string                 `json:"user_id"`
	Amount      *big.Int               `json:"amount"` // Positive for credit, negative for debit
	Currency    Currency               `json:"currency"`
	Type        TransactionType        `json:"type"`
	Status      TransactionStatus      `json:"status"`
	Reason      string                 `json:"reason"`
	ReferenceID string                 `json:"reference_id,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	ProcessedAt *time.Time             `json:"processed_at,omitempty"`
}

// TransactionType represents transaction type
type TransactionType string

const (
	TransactionTypeCredit     TransactionType = "credit"
	TransactionTypeDebit      TransactionType = "debit"
	TransactionTypeTransfer   TransactionType = "transfer"
	TransactionTypeAdjustment TransactionType = "adjustment"
)

// TransactionStatus represents transaction status
type TransactionStatus string

const (
	TransactionStatusPending   TransactionStatus = "pending"
	TransactionStatusCompleted TransactionStatus = "completed"
	TransactionStatusFailed    TransactionStatus = "failed"
	TransactionStatusReversed  TransactionStatus = "reversed"
)

// PaginatedTransactions represents paginated transaction list
type PaginatedTransactions struct {
	Transactions []*Transaction `json:"transactions"`
	Total        int64          `json:"total"`
	Page         int            `json:"page"`
	Limit        int            `json:"limit"`
	HasNext      bool           `json:"has_next"`
	HasPrev      bool           `json:"has_prev"`
}

// BalanceServiceError represents balance service error
type BalanceServiceError struct {
	Code    BalanceServiceErrorCode `json:"code"`
	Message string                  `json:"message"`
}

func (e *BalanceServiceError) Error() string {
	return fmt.Sprintf("BalanceServiceError: %s - %s", e.Code, e.Message)
}

// BalanceServiceErrorCode represents balance service error codes
type BalanceServiceErrorCode string

const (
	ErrInvalidUserID     BalanceServiceErrorCode = "INVALID_USER_ID"
	ErrInvalidAmount     BalanceServiceErrorCode = "INVALID_AMOUNT"
	ErrInvalidCurrency   BalanceServiceErrorCode = "INVALID_CURRENCY"
	ErrDatabaseError     BalanceServiceErrorCode = "DATABASE_ERROR"
	ErrInsufficientFunds BalanceServiceErrorCode = "INSUFFICIENT_FUNDS"
	ErrTransactionFailed BalanceServiceErrorCode = "TRANSACTION_FAILED"
	ErrUserNotFound      BalanceServiceErrorCode = "USER_NOT_FOUND"
	ErrBalanceNotFound   BalanceServiceErrorCode = "BALANCE_NOT_FOUND"
)

// BalanceHistory represents balance history entry
type BalanceHistory struct {
	UserID    string    `json:"user_id"`
	Amount    *big.Int  `json:"amount"`
	Currency  Currency  `json:"currency"`
	Balance   *big.Int  `json:"balance"` // Current balance after this change
	Change    *big.Int  `json:"change"`  // Amount changed (positive for credit, negative for debit)
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
}

// User represents a user in the system
type User struct {
	ID        string     `json:"id"`
	Email     string     `json:"email,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	Status    UserStatus `json:"status"`
}

// UserStatus represents user status
type UserStatus string

const (
	UserStatusActive    UserStatus = "active"
	UserStatusInactive  UserStatus = "inactive"
	UserStatusSuspended UserStatus = "suspended"
)

// BalanceRequest represents balance query request
type BalanceRequest struct {
	UserID   string   `json:"user_id"`
	Currency Currency `json:"currency"`
}

// BalanceResponse represents balance query response
type BalanceResponse struct {
	Balance *Balance `json:"balance"`
	Error   string   `json:"error,omitempty"`
}

// TransferRequest represents balance transfer request
type TransferRequest struct {
	FromUserID  string   `json:"from_user_id"`
	ToUserID    string   `json:"to_user_id"`
	Amount      float64  `json:"amount"`
	Currency    Currency `json:"currency"`
	Reason      string   `json:"reason"`
	ReferenceID string   `json:"reference_id,omitempty"`
}

// TransferResponse represents balance transfer response
type TransferResponse struct {
	Success       bool   `json:"success"`
	FromBalance   *Money `json:"from_balance,omitempty"`
	ToBalance     *Money `json:"to_balance,omitempty"`
	Error         string `json:"error,omitempty"`
	TransactionID string `json:"transaction_id,omitempty"`
}

// Health represents service health check response
type Health struct {
	Status    string            `json:"status"`
	Timestamp time.Time         `json:"timestamp"`
	Version   string            `json:"version"`
	Checks    map[string]string `json:"checks"`
}
