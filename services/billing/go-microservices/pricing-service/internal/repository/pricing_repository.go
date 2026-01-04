package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// PricingRepository интерфейс для работы с ценами
type PricingRepository interface {
	GetPricing(ctx context.Context, modelID string) (*PricingModel, error)
	GetAllPricing(ctx context.Context) (map[string]*PricingModel, error)
	CreatePricing(ctx context.Context, model *PricingModel) error
	UpdatePricing(ctx context.Context, modelID string, model *PricingModel) error
	DeletePricing(ctx context.Context, modelID string) error
	GetModelsByProvider(ctx context.Context, provider string) ([]*PricingModel, error)
	GetActiveModels(ctx context.Context) ([]*PricingModel, error)
	CalculateCost(ctx context.Context, modelID string, inputTokens, outputTokens int) (*CostCalculation, error)
	GetPricingHistory(ctx context.Context, modelID string, limit int) ([]*PricingHistory, error)
	HealthCheck(ctx context.Context) error
}

// RedisPricingRepository реализация на Redis
type RedisPricingRepository struct {
	client *redis.Client
	ttl    time.Duration
}

// PricingModel представляет модель ценообразования
type PricingModel struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Provider  string                 `json:"provider"`
	Pricing   PricingType            `json:"pricing"`
	Active    bool                   `json:"active"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
	Source    string                 `json:"source"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// PricingType представляет тип ценообразования
type PricingType struct {
	Input  float64 `json:"input"`  // Стоимость за входной токен
	Output float64 `json:"output"` // Стоимость за выходной токен
	Embed  float64 `json:"embed"`  // Стоимость за embed токен
}

// CostCalculation представляет расчет стоимости
type CostCalculation struct {
	ModelID       string    `json:"model_id"`
	InputTokens   int       `json:"input_tokens"`
	OutputTokens  int       `json:"output_tokens"`
	InputCost     float64   `json:"input_cost"`
	OutputCost    float64   `json:"output_cost"`
	TotalCost     float64   `json:"total_cost"`
	Currency      string    `json:"currency"`
	CalculationAt time.Time `json:"calculation_at"`
}

// PricingHistory представляет историю изменения цены
type PricingHistory struct {
	ModelID    string      `json:"model_id"`
	OldPricing PricingType `json:"old_pricing"`
	NewPricing PricingType `json:"new_pricing"`
	ChangedBy  string      `json:"changed_by"`
	ChangedAt  time.Time   `json:"changed_at"`
	Source     string      `json:"source"`
	Reason     string      `json:"reason,omitempty"`
}

// NewRedisPricingRepository создает новый репозиторий
func NewRedisPricingRepository(client *redis.Client, ttl time.Duration) *RedisPricingRepository {
	return &RedisPricingRepository{
		client: client,
		ttl:    ttl,
	}
}

// GetPricing получение цены модели
func (r *RedisPricingRepository) GetPricing(ctx context.Context, modelID string) (*PricingModel, error) {
	key := fmt.Sprintf("pricing:model:%s", modelID)

	data, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("pricing model not found: %s", modelID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get pricing: %w", err)
	}

	var model PricingModel
	if err := json.Unmarshal([]byte(data), &model); err != nil {
		return nil, fmt.Errorf("failed to unmarshal pricing model: %w", err)
	}

	return &model, nil
}

// GetAllPricing получение всех цен
func (r *RedisPricingRepository) GetAllPricing(ctx context.Context) (map[string]*PricingModel, error) {
	pattern := "pricing:model:*"
	keys, err := r.client.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get pricing keys: %w", err)
	}

	models := make(map[string]*PricingModel)
	if len(keys) == 0 {
		return models, nil
	}

	values, err := r.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get pricing values: %w", err)
	}

	for i, key := range keys {
		if i < len(values) && values[i] != nil {
			var model PricingModel
			if err := json.Unmarshal([]byte(values[i].(string)), &model); err == nil {
				modelID := extractModelID(key)
				models[modelID] = &model
			}
		}
	}

	return models, nil
}

// CreatePricing создание новой цены
func (r *RedisPricingRepository) CreatePricing(ctx context.Context, model *PricingModel) error {
	if model.CreatedAt.IsZero() {
		model.CreatedAt = time.Now()
	}
	model.UpdatedAt = time.Now()

	key := fmt.Sprintf("pricing:model:%s", model.ID)
	data, err := json.Marshal(model)
	if err != nil {
		return fmt.Errorf("failed to marshal pricing model: %w", err)
	}

	err = r.client.Set(ctx, key, data, r.ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to create pricing: %w", err)
	}

	// Обновляем индекс провайдера
	if err := r.updateProviderIndex(ctx, model.Provider, model.ID, true); err != nil {
		return fmt.Errorf("failed to update provider index: %w", err)
	}

	return nil
}

// UpdatePricing обновление цены
func (r *RedisPricingRepository) UpdatePricing(ctx context.Context, modelID string, model *PricingModel) error {
	model.UpdatedAt = time.Now()

	key := fmt.Sprintf("pricing:model:%s", modelID)
	data, err := json.Marshal(model)
	if err != nil {
		return fmt.Errorf("failed to marshal pricing model: %w", err)
	}

	err = r.client.Set(ctx, key, data, r.ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to update pricing: %w", err)
	}

	return nil
}

// DeletePricing удаление цены
func (r *RedisPricingRepository) DeletePricing(ctx context.Context, modelID string) error {
	key := fmt.Sprintf("pricing:model:%s", modelID)

	err := r.client.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to delete pricing: %w", err)
	}

	return nil
}

// GetModelsByProvider получение моделей по провайдеру
func (r *RedisPricingRepository) GetModelsByProvider(ctx context.Context, provider string) ([]*PricingModel, error) {
	indexKey := fmt.Sprintf("pricing:provider:%s", provider)

	// Получаем список моделей для провайдера
	modelIDs, err := r.client.SMembers(ctx, indexKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get provider models: %w", err)
	}

	var models []*PricingModel
	for _, modelID := range modelIDs {
		model, err := r.GetPricing(ctx, modelID)
		if err == nil {
			models = append(models, model)
		}
	}

	return models, nil
}

// GetActiveModels получение активных моделей
func (r *RedisPricingRepository) GetActiveModels(ctx context.Context) ([]*PricingModel, error) {
	models, err := r.GetAllPricing(ctx)
	if err != nil {
		return nil, err
	}

	var activeModels []*PricingModel
	for _, model := range models {
		if model.Active {
			activeModels = append(activeModels, model)
		}
	}

	return activeModels, nil
}

// CalculateCost расчет стоимости
func (r *RedisPricingRepository) CalculateCost(ctx context.Context, modelID string, inputTokens, outputTokens int) (*CostCalculation, error) {
	model, err := r.GetPricing(ctx, modelID)
	if err != nil {
		return nil, err
	}

	inputCost := float64(inputTokens) * model.Pricing.Input
	outputCost := float64(outputTokens) * model.Pricing.Output
	totalCost := inputCost + outputCost

	return &CostCalculation{
		ModelID:       modelID,
		InputTokens:   inputTokens,
		OutputTokens:  outputTokens,
		InputCost:     inputCost,
		OutputCost:    outputCost,
		TotalCost:     totalCost,
		Currency:      "USD", // По умолчанию
		CalculationAt: time.Now(),
	}, nil
}

// GetPricingHistory получение истории цен
func (r *RedisPricingRepository) GetPricingHistory(ctx context.Context, modelID string, limit int) ([]*PricingHistory, error) {
	if limit <= 0 || limit > 100 {
		limit = 100
	}

	historyKey := fmt.Sprintf("pricing:history:%s", modelID)
	historyData, err := r.client.LRange(ctx, historyKey, 0, int64(limit-1)).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get pricing history: %w", err)
	}

	var history []*PricingHistory
	for _, item := range historyData {
		var entry PricingHistory
		if err := json.Unmarshal([]byte(item), &entry); err == nil {
			history = append(history, &entry)
		}
	}

	return history, nil
}

// HealthCheck проверка здоровья репозитория
func (r *RedisPricingRepository) HealthCheck(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// updateProviderIndex обновление индекса провайдера
func (r *RedisPricingRepository) updateProviderIndex(ctx context.Context, provider, modelID string, add bool) error {
	indexKey := fmt.Sprintf("pricing:provider:%s", provider)

	if add {
		return r.client.SAdd(ctx, indexKey, modelID).Err()
	} else {
		return r.client.SRem(ctx, indexKey, modelID).Err()
	}
}

// extractModelID извлекает ID модели из ключа Redis
func extractModelID(key string) string {
	// key format: "pricing:model:{modelID}"
	parts := splitKey(key)
	if len(parts) >= 3 {
		return parts[2]
	}
	return ""
}

// splitKey разделяет ключ на части
func splitKey(key string) []string {
	var parts []string
	current := ""

	for _, char := range key {
		if char == ':' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(char)
		}
	}

	if current != "" {
		parts = append(parts, current)
	}

	return parts
}
