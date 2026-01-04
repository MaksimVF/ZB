package models

import (
	"context"
	"time"
)

// Service constants
const (
	ServiceName = "balance-service"
	ServicePort = 8081
)

// Request/Response types for HTTP handlers (unique to this service)
type (
	// Balance query request
	BalanceQueryRequest struct {
		UserID   string `json:"user_id"`
		Currency string `json:"currency,omitempty"`
	}

	// Balance query response
	BalanceQueryResponse struct {
		Balance *Balance `json:"balance,omitempty"`
		Error   string   `json:"error,omitempty"`
	}

	// Balance adjustment request
	BalanceAdjustRequest struct {
		UserID          string                 `json:"user_id"`
		Amount          float64                `json:"amount"` // Positive for credit, negative for debit
		Currency        string                 `json:"currency"`
		Reason          string                 `json:"reason"`
		ForceAdjustment bool                   `json:"force_adjustment,omitempty"`
		ReferenceID     string                 `json:"reference_id,omitempty"`
		Metadata        map[string]interface{} `json:"metadata,omitempty"`
	}

	// Balance adjustment response
	BalanceAdjustResponse struct {
		Success       bool   `json:"success"`
		NewBalance    string `json:"new_balance,omitempty"`
		Error         string `json:"error,omitempty"`
		TransactionID string `json:"transaction_id,omitempty"`
	}

	// Transaction query request
	TransactionQueryRequest struct {
		UserID string `json:"user_id"`
		Limit  int    `json:"limit,omitempty"`
		Offset int    `json:"offset,omitempty"`
	}

	// Transaction query response
	TransactionQueryResponse struct {
		Transactions []*Transaction `json:"transactions,omitempty"`
		Total        int64          `json:"total"`
		Error        string         `json:"error,omitempty"`
	}

	// Health check request
	HealthCheckRequest struct {
		Context context.Context `json:"-"`
	}

	// Health check response
	HealthCheckResponse struct {
		Status    string            `json:"status"`
		Timestamp time.Time         `json:"timestamp"`
		Version   string            `json:"version"`
		Checks    map[string]string `json:"checks"`
	}
)

// Service-specific error types (using different names to avoid conflicts)
type (
	BalanceError struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}

	BalanceErrorCode string
)

// Service-specific error codes (using different names)
const (
	BalanceErrInvalidUserID     BalanceErrorCode = "INVALID_USER_ID"
	BalanceErrInvalidAmount     BalanceErrorCode = "INVALID_AMOUNT"
	BalanceErrInvalidCurrency   BalanceErrorCode = "INVALID_CURRENCY"
	BalanceErrDatabaseError     BalanceErrorCode = "DATABASE_ERROR"
	BalanceErrInsufficientFunds BalanceErrorCode = "INSUFFICIENT_FUNDS"
	BalanceErrTransactionFailed BalanceErrorCode = "TRANSACTION_FAILED"
	BalanceErrUserNotFound      BalanceErrorCode = "USER_NOT_FOUND"
	BalanceErrBalanceNotFound   BalanceErrorCode = "BALANCE_NOT_FOUND"
)
