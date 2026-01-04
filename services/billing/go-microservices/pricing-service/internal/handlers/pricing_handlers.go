package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/mux"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"pricing-service/internal/models"
	pb "pricing-service/proto"
)

// PricingService - структура для работы с ценообразованием
type PricingService struct {
	redisClient *redis.Client
}

// NewPricingService - конструктор для PricingService
func NewPricingService(redisClient *redis.Client) *PricingService {
	return &PricingService{
		redisClient: redisClient,
	}
}

// GetModels - получение списка доступных моделей
func (ps *PricingService) GetModels(ctx context.Context, req *pb.GetModelsRequest) (*pb.GetModelsResponse, error) {
	modelsData, err := ps.redisClient.Get(ctx, "pricing:models").Result()
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to get models")
	}

	var models []models.Model
	if err := json.Unmarshal([]byte(modelsData), &models); err != nil {
		return nil, status.Error(codes.Internal, "failed to parse models")
	}

	pbModels := make([]*pb.ModelInfo, len(models))
	for i, model := range models {
		pbModels[i] = &pb.ModelInfo{
			Id:          model.ID,
			Name:        model.Name,
			Type:        model.Type,
			Description: model.Description,
		}
	}

	return &pb.GetModelsResponse{
		Models: pbModels,
	}, nil
}

// GetPricing - получение цены для конкретной модели
func (ps *PricingService) GetPricing(ctx context.Context, req *pb.GetPricingRequest) (*pb.GetPricingResponse, error) {
	pricingData, err := ps.redisClient.HGet(ctx, "pricing:models", req.ModelId).Result()
	if err != nil {
		return nil, status.Error(codes.NotFound, "model not found")
	}

	var pricing models.Pricing
	if err := json.Unmarshal([]byte(pricingData), &pricing); err != nil {
		return nil, status.Error(codes.Internal, "failed to parse pricing")
	}

	return &pb.GetPricingResponse{
		ModelId:    req.ModelId,
		InputCost:  pricing.InputCost,
		OutputCost: pricing.OutputCost,
		Currency:   pricing.Currency,
		UpdatedAt:  pricing.UpdatedAt.Format(time.RFC3339),
	}, nil
}

// UpdatePricing - обновление цены для модели (только для администраторов)
func (ps *PricingService) UpdatePricing(ctx context.Context, req *pb.UpdatePricingRequest) (*pb.UpdatePricingResponse, error) {
	// Проверка админ прав (здесь должна быть реализована авторизация)
	// В реальном приложении нужно проверить JWT токен и роли

	pricing := models.Pricing{
		ModelID:    req.ModelId,
		InputCost:  req.InputCost,
		OutputCost: req.OutputCost,
		Currency:   req.Currency,
		UpdatedAt:  time.Now(),
	}

	pricingJSON, err := json.Marshal(pricing)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to marshal pricing")
	}

	err = ps.redisClient.HSet(ctx, "pricing:models", req.ModelId, pricingJSON).Err()
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to update pricing")
	}

	return &pb.UpdatePricingResponse{
		Success:   true,
		UpdatedAt: pricing.UpdatedAt.Format(time.RFC3339),
	}, nil
}

// BulkUpdatePricing - массовое обновление цен
func (ps *PricingService) BulkUpdatePricing(ctx context.Context, req *pb.BulkUpdatePricingRequest) (*pb.BulkUpdatePricingResponse, error) {
	updated := 0
	failed := 0

	for _, pricingReq := range req.PricingUpdates {
		pricing := models.Pricing{
			ModelID:    pricingReq.ModelId,
			InputCost:  pricingReq.InputCost,
			OutputCost: pricingReq.OutputCost,
			Currency:   pricingReq.Currency,
			UpdatedAt:  time.Now(),
		}

		pricingJSON, err := json.Marshal(pricing)
		if err != nil {
			failed++
			continue
		}

		err = ps.redisClient.HSet(ctx, "pricing:models", pricingReq.ModelId, pricingJSON).Err()
		if err != nil {
			failed++
			continue
		}

		updated++
	}

	return &pb.BulkUpdatePricingResponse{
		Updated: updated,
		Failed:  failed,
		Total:   len(req.PricingUpdates),
	}, nil
}

// GetPricingHistory - получение истории изменения цен
func (ps *PricingService) GetPricingHistory(ctx context.Context, req *pb.GetPricingHistoryRequest) (*pb.GetPricingHistoryResponse, error) {
	historyData, err := ps.redisClient.LRange(ctx, "pricing:history:"+req.ModelId, 0, -1).Result()
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to get pricing history")
	}

	var history []models.PricingHistory
	for _, item := range historyData {
		var entry models.PricingHistory
		if err := json.Unmarshal([]byte(item), &entry); err != nil {
			continue
		}
		history = append(history, entry)
	}

	pbHistory := make([]*pb.PricingHistoryEntry, len(history))
	for i, entry := range history {
		pbHistory[i] = &pb.PricingHistoryEntry{
			ModelId:    entry.ModelID,
			InputCost:  entry.InputCost,
			OutputCost: entry.OutputCost,
			Currency:   entry.Currency,
			ChangedAt:  entry.ChangedAt.Format(time.RFC3339),
			ChangedBy:  entry.ChangedBy,
		}
	}

	return &pb.GetPricingHistoryResponse{
		History: pbHistory,
	}, nil
}

// CalculateCost - расчет стоимости использования модели
func (ps *PricingService) CalculateCost(ctx context.Context, req *pb.CalculateCostRequest) (*pb.CalculateCostResponse, error) {
	pricingData, err := ps.redisClient.HGet(ctx, "pricing:models", req.ModelId).Result()
	if err != nil {
		return nil, status.Error(codes.NotFound, "model not found")
	}

	var pricing models.Pricing
	if err := json.Unmarshal([]byte(pricingData), &pricing); err != nil {
		return nil, status.Error(codes.Internal, "failed to parse pricing")
	}

	totalCost := float64(req.InputTokens)*pricing.InputCost + float64(req.OutputTokens)*pricing.OutputCost

	return &pb.CalculateCostResponse{
		ModelId:       req.ModelId,
		InputTokens:   req.InputTokens,
		OutputTokens:  req.OutputTokens,
		TotalCost:     totalCost,
		Currency:      pricing.Currency,
		InputCost:     pricing.InputCost,
		OutputCost:    pricing.OutputCost,
		CalculationAt: time.Now().Format(time.RFC3339),
	}, nil
}

// HTTP Handlers

// GetModelsHTTP - HTTP обработчик для получения списка моделей
func (ps *PricingService) GetModelsHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	modelsData, err := ps.redisClient.Get(ctx, "pricing:models").Result()
	if err != nil {
		http.Error(w, "Failed to get models", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(modelsData))
}

// GetPricingHTTP - HTTP обработчик для получения цены модели
func (ps *PricingService) GetPricingHTTP(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	modelID := vars["modelId"]

	if modelID == "" {
		http.Error(w, "Model ID is required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	pricingData, err := ps.redisClient.HGet(ctx, "pricing:models", modelID).Result()
	if err != nil {
		http.Error(w, "Model not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(pricingData))
}

// UpdatePricingHTTP - HTTP обработчик для обновления цены
func (ps *PricingService) UpdatePricingHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	vars := mux.Vars(r)
	modelID := vars["modelId"]

	if modelID == "" {
		http.Error(w, "Model ID is required", http.StatusBadRequest)
		return
	}

	var pricing models.Pricing
	if err := json.NewDecoder(r.Body).Decode(&pricing); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	pricing.ModelID = modelID
	pricing.UpdatedAt = time.Now()

	pricingJSON, err := json.Marshal(pricing)
	if err != nil {
		http.Error(w, "Failed to marshal pricing", http.StatusInternalServerError)
		return
	}

	ctx := r.Context()
	err = ps.redisClient.HSet(ctx, "pricing:models", modelID, pricingJSON).Err()
	if err != nil {
		http.Error(w, "Failed to update pricing", http.StatusInternalServerError)
		return
	}

	// Сохранение в историю
	historyEntry := models.PricingHistory{
		ModelID:    modelID,
		InputCost:  pricing.InputCost,
		OutputCost: pricing.OutputCost,
		Currency:   pricing.Currency,
		ChangedAt:  time.Now(),
		ChangedBy:  "admin", // В реальном приложении брать из JWT
	}

	historyJSON, _ := json.Marshal(historyEntry)
	ps.redisClient.LPush(ctx, "pricing:history:"+modelID, historyJSON)

	w.Header().Set("Content-Type", "application/json")
	w.Write(pricingJSON)
}

// CalculateCostHTTP - HTTP обработчик для расчета стоимости
func (ps *PricingService) CalculateCostHTTP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ModelId      string `json:"model_id"`
		InputTokens  int    `json:"input_tokens"`
		OutputTokens int    `json:"output_tokens"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	pricingData, err := ps.redisClient.HGet(ctx, "pricing:models", req.ModelId).Result()
	if err != nil {
		http.Error(w, "Model not found", http.StatusNotFound)
		return
	}

	var pricing models.Pricing
	if err := json.Unmarshal([]byte(pricingData), &pricing); err != nil {
		http.Error(w, "Failed to parse pricing", http.StatusInternalServerError)
		return
	}

	totalCost := float64(req.InputTokens)*pricing.InputCost + float64(req.OutputTokens)*pricing.OutputCost

	response := struct {
		ModelId       string  `json:"model_id"`
		InputTokens   int     `json:"input_tokens"`
		OutputTokens  int     `json:"output_tokens"`
		TotalCost     float64 `json:"total_cost"`
		Currency      string  `json:"currency"`
		InputCost     float64 `json:"input_cost"`
		OutputCost    float64 `json:"output_cost"`
		CalculationAt string  `json:"calculation_at"`
	}{
		ModelId:       req.ModelId,
		InputTokens:   req.InputTokens,
		OutputTokens:  req.OutputTokens,
		TotalCost:     totalCost,
		Currency:      pricing.Currency,
		InputCost:     pricing.InputCost,
		OutputCost:    pricing.OutputCost,
		CalculationAt: time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HealthCheck - проверка здоровья сервиса
func (ps *PricingService) HealthCheck(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	err := ps.redisClient.Ping(ctx).Err()
	if err != nil {
		http.Error(w, "Redis connection failed", http.StatusServiceUnavailable)
		return
	}

	response := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
		"service":   "pricing-service",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
