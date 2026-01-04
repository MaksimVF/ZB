package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"balance-service/internal/service"
)

// BalanceHandlers структура для обработчиков баланса
type BalanceHandlers struct {
	balanceService *service.BalanceService
}

// NewBalanceHandlers создает новые обработчики баланса
func NewBalanceHandlers(balanceService *service.BalanceService) *BalanceHandlers {
	return &BalanceHandlers{
		balanceService: balanceService,
	}
}

// GetBalanceHandler обработчик получения баланса
func (h *BalanceHandlers) GetBalanceHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Получаем user_id из URL параметров
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}

	// Получаем баланс
	balance, err := h.balanceService.GetBalance(ctx, userID)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get balance: %v", err), http.StatusInternalServerError)
		return
	}

	// Возвращаем ответ
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"user_id":       balance.UserID,
		"balance":       balance.Balance,
		"currency":      balance.Currency,
		"updated_at":    balance.UpdatedAt,
		"total_charged": balance.TotalCharged,
		"total_spent":   balance.TotalSpent,
	})
}

// AdjustBalanceHandler обработчик корректировки баланса
func (h *BalanceHandlers) AdjustBalanceHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req service.AdjustBalanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Проверяем админский ключ
	adminKey := r.Header.Get("X-Admin-Key")
	if adminKey == "" {
		http.Error(w, "admin key required", http.StatusUnauthorized)
		return
	}

	if !h.balanceService.ValidateAdminKey(adminKey) {
		http.Error(w, "invalid admin key", http.StatusUnauthorized)
		return
	}

	// Корректируем баланс
	balance, err := h.balanceService.AdjustBalance(ctx, req)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to adjust balance: %v", err), http.StatusInternalServerError)
		return
	}

	// Возвращаем ответ
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":           true,
		"user_id":           balance.UserID,
		"balance":           balance.Balance,
		"currency":          balance.Currency,
		"updated_at":        balance.UpdatedAt,
		"adjustment_amount": req.Amount,
		"reason":            req.Reason,
	})
}

// ReserveBalanceHandler обработчик резервирования средств
func (h *BalanceHandlers) ReserveBalanceHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req struct {
		UserID        string  `json:"user_id"`
		Amount        float64 `json:"amount"`
		ReservationID string  `json:"reservation_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Резервируем средства
	err := h.balanceService.ReserveBalance(ctx, req.UserID, req.Amount, req.ReservationID)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to reserve balance: %v", err), http.StatusInternalServerError)
		return
	}

	// Возвращаем ответ
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":         true,
		"user_id":         req.UserID,
		"reserved_amount": req.Amount,
		"reservation_id":  req.ReservationID,
		"reserved_at":     time.Now(),
	})
}

// CommitReservationHandler обработчик подтверждения резерва
func (h *BalanceHandlers) CommitReservationHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req struct {
		UserID        string  `json:"user_id"`
		ReservationID string  `json:"reservation_id"`
		FinalAmount   float64 `json:"final_amount"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Подтверждаем резерв
	err := h.balanceService.CommitReservation(ctx, req.UserID, req.ReservationID, req.FinalAmount)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to commit reservation: %v", err), http.StatusInternalServerError)
		return
	}

	// Возвращаем ответ
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":        true,
		"user_id":        req.UserID,
		"reservation_id": req.ReservationID,
		"final_amount":   req.FinalAmount,
		"committed_at":   time.Now(),
	})
}

// CancelReservationHandler обработчик отмены резерва
func (h *BalanceHandlers) CancelReservationHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req struct {
		ReservationID string `json:"reservation_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Отменяем резерв
	err := h.balanceService.CancelReservation(ctx, req.ReservationID)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to cancel reservation: %v", err), http.StatusInternalServerError)
		return
	}

	// Возвращаем ответ
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":        true,
		"reservation_id": req.ReservationID,
		"cancelled_at":   time.Now(),
	})
}

// GetUserBalancesHandler обработчик получения балансов пользователей (для админки)
func (h *BalanceHandlers) GetUserBalancesHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Проверяем админский ключ
	adminKey := r.Header.Get("X-Admin-Key")
	if adminKey == "" {
		http.Error(w, "admin key required", http.StatusUnauthorized)
		return
	}

	if !h.balanceService.ValidateAdminKey(adminKey) {
		http.Error(w, "invalid admin key", http.StatusUnauthorized)
		return
	}

	// Получаем лимит
	limit := 100
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l := parseInt(limitStr); l > 0 && l <= 1000 {
			limit = l
		}
	}

	// Получаем балансы
	balances, err := h.balanceService.GetUserBalances(ctx, limit)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get user balances: %v", err), http.StatusInternalServerError)
		return
	}

	// Возвращаем ответ
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"balances": balances,
		"count":    len(balances),
		"limit":    limit,
	})
}

// HealthHandler обработчик проверки здоровья сервиса
func (h *BalanceHandlers) HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	healthy := h.balanceService.IsHealthy()
	status := "healthy"
	if !healthy {
		status = "unhealthy"
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    status,
		"timestamp": time.Now(),
		"service":   "balance-service",
		"version":   "1.0.0",
	})
}

// Utility function to parse integers safely
func parseInt(s string) int {
	if s == "" {
		return 0
	}
	var result int
	fmt.Sscanf(s, "%d", &result)
	return result
}
