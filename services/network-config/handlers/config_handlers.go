package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"

	"github.com/MaksimVF/ZB/services/network-config/models"
	"github.com/MaksimVF/ZB/services/network-config/services"
)

// ConfigHandlers provides HTTP handlers for configuration management
type ConfigHandlers struct {
	redisService *services.RedisService
	validator    *validator.Validate
	logger       *zap.Logger
}

// NewConfigHandlers creates new configuration handlers
func NewConfigHandlers(redisService *services.RedisService, logger *zap.Logger) *ConfigHandlers {
	return &ConfigHandlers{
		redisService: redisService,
		validator:    validator.New(),
		logger:       logger,
	}
}

// GetConfig handles GET /api/v1/config
func (h *ConfigHandlers) GetConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	configKey := chi.URLParam(r, "id")
	if configKey == "" {
		configKey = "current"
	}

	var config models.NetworkConfig
	err := h.redisService.GetConfig(ctx, "config:"+configKey, &config)
	if err != nil {
		h.logger.Error("Failed to get config", zap.Error(err))
		SendErrorResponse(w, http.StatusNotFound, "Configuration not found", err.Error())
		return
	}

	h.SendSuccessResponse(w, http.StatusOK, config)
}

// GetConfigs handles GET /api/v1/configs (list all configurations)
func (h *ConfigHandlers) GetConfigs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get pagination parameters
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}

	// Get all config keys
	keys, err := h.redisService.GetKeysByPattern(ctx, "config:*")
	if err != nil {
		h.logger.Error("Failed to get config keys", zap.Error(err))
		SendErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve configurations", err.Error())
		return
	}

	// Get configurations
	var configs []models.NetworkConfig
	for _, key := range keys {
		var config models.NetworkConfig
		err := h.redisService.GetConfig(ctx, key, &config)
		if err == nil {
			configs = append(configs, config)
		}
	}

	// Apply pagination
	start := (page - 1) * perPage
	end := start + perPage
	if start > len(configs) {
		start = len(configs)
	}
	if end > len(configs) {
		end = len(configs)
	}

	paginatedConfigs := configs[start:end]

	// Create response with pagination
	response := models.APIResponse{
		Success: true,
		Data:    paginatedConfigs,
		Pagination: &models.Pagination{
			Page:       page,
			PerPage:    perPage,
			Total:      len(configs),
			TotalPages: (len(configs) + perPage - 1) / perPage,
			HasNext:    end < len(configs),
			HasPrev:    page > 1,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// CreateConfig handles POST /api/v1/config
func (h *ConfigHandlers) CreateConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req models.ConfigUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Warn("Invalid request body", zap.Error(err))
		SendErrorResponse(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	// Validate configuration
	if err := h.validator.Struct(req.Config); err != nil {
		h.logger.Warn("Validation failed", zap.Error(err))
		SendValidationErrorResponse(w, err)
		return
	}

	// Set metadata
	now := time.Now()
	req.Config.ID = generateID()
	req.Config.CreatedAt = now
	req.Config.UpdatedAt = now
	req.Config.Version = 1
	req.Config.Status = "active"

	// Store configuration
	configKey := "config:" + req.Config.ID
	err := h.redisService.StoreConfig(ctx, configKey, req.Config, 0)
	if err != nil {
		h.logger.Error("Failed to store config", zap.Error(err))
		SendErrorResponse(w, http.StatusInternalServerError, "Failed to store configuration", err.Error())
		return
	}

	// Store in history
	history := models.ConfigHistory{
		ID:         generateID(),
		ConfigID:   req.Config.ID,
		Version:    1,
		Changes:    "Initial configuration created",
		CreatedBy:  "system",
		CreatedAt:  now,
		ConfigData: marshalConfig(req.Config),
	}

	err = h.redisService.StoreConfigHistory(ctx, req.Config.ID, history)
	if err != nil {
		h.logger.Warn("Failed to store history", zap.Error(err))
	}

	h.logger.Info("Configuration created",
		zap.String("config_id", req.Config.ID),
		zap.String("name", req.Config.Name))

	h.SendSuccessResponse(w, http.StatusCreated, req.Config)
}

// UpdateConfig handles PUT /api/v1/config/{id}
func (h *ConfigHandlers) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	configID := chi.URLParam(r, "id")

	if configID == "" {
		SendErrorResponse(w, http.StatusBadRequest, "Configuration ID is required", "")
		return
	}

	var req models.ConfigUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Warn("Invalid request body", zap.Error(err))
		SendErrorResponse(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	// Validate configuration
	if err := h.validator.Struct(req.Config); err != nil {
		h.logger.Warn("Validation failed", zap.Error(err))
		SendValidationErrorResponse(w, err)
		return
	}

	// Get existing configuration
	var existingConfig models.NetworkConfig
	err := h.redisService.GetConfig(ctx, "config:"+configID, &existingConfig)
	if err != nil {
		h.logger.Error("Config not found", zap.String("config_id", configID))
		SendErrorResponse(w, http.StatusNotFound, "Configuration not found", err.Error())
		return
	}

	// Check if force update is required
	if existingConfig.Version != req.Config.Version && !req.Force {
		SendErrorResponse(w, http.StatusConflict,
			"Configuration version mismatch. Use force=true to override", "")
		return
	}

	// Update metadata
	req.Config.ID = configID
	req.Config.UpdatedAt = time.Now()
	req.Config.Version = existingConfig.Version + 1
	req.Config.CreatedAt = existingConfig.CreatedAt

	// Store updated configuration
	configKey := "config:" + configID
	err = h.redisService.StoreConfig(ctx, configKey, req.Config, 0)
	if err != nil {
		h.logger.Error("Failed to store updated config", zap.Error(err))
		SendErrorResponse(w, http.StatusInternalServerError, "Failed to update configuration", err.Error())
		return
	}

	// Store in history
	history := models.ConfigHistory{
		ID:         generateID(),
		ConfigID:   configID,
		Version:    req.Config.Version,
		Changes:    "Configuration updated",
		CreatedBy:  "system",
		CreatedAt:  time.Now(),
		ConfigData: marshalConfig(req.Config),
	}

	err = h.redisService.StoreConfigHistory(ctx, configID, history)
	if err != nil {
		h.logger.Warn("Failed to store history", zap.Error(err))
	}

	h.logger.Info("Configuration updated",
		zap.String("config_id", configID),
		zap.Int("version", req.Config.Version))

	h.SendSuccessResponse(w, http.StatusOK, req.Config)
}

// DeleteConfig handles DELETE /api/v1/config/{id}
func (h *ConfigHandlers) DeleteConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	configID := chi.URLParam(r, "id")

	if configID == "" {
		SendErrorResponse(w, http.StatusBadRequest, "Configuration ID is required", "")
		return
	}

	// Check if config exists
	var config models.NetworkConfig
	err := h.redisService.GetConfig(ctx, "config:"+configID, &config)
	if err != nil {
		h.logger.Error("Config not found", zap.String("config_id", configID))
		SendErrorResponse(w, http.StatusNotFound, "Configuration not found", err.Error())
		return
	}

	// Soft delete - mark as deprecated instead of deleting
	config.Status = "deprecated"
	config.UpdatedAt = time.Now()

	err = h.redisService.StoreConfig(ctx, "config:"+configID, config, 0)
	if err != nil {
		h.logger.Error("Failed to soft delete config", zap.Error(err))
		SendErrorResponse(w, http.StatusInternalServerError, "Failed to delete configuration", err.Error())
		return
	}

	h.logger.Info("Configuration deprecated", zap.String("config_id", configID))
	h.SendSuccessResponse(w, http.StatusOK, map[string]string{"status": "deprecated"})
}

// GetConfigHistory handles GET /api/v1/config/{id}/history
func (h *ConfigHandlers) GetConfigHistory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	configID := chi.URLParam(r, "id")

	if configID == "" {
		SendErrorResponse(w, http.StatusBadRequest, "Configuration ID is required", "")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 100 {
		limit = 20
	}

	entries, err := h.redisService.GetConfigHistory(ctx, configID, limit)
	if err != nil {
		h.logger.Error("Failed to get config history", zap.Error(err))
		SendErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve history", err.Error())
		return
	}

	// Parse history entries
	var history []models.ConfigHistory
	for _, entry := range entries {
		var hist models.ConfigHistory
		if err := json.Unmarshal([]byte(entry), &hist); err == nil {
			history = append(history, hist)
		}
	}

	h.SendSuccessResponse(w, http.StatusOK, history)
}

// GetNetworkStatus handles GET /api/v1/status
func (h *ConfigHandlers) GetNetworkStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var status models.NetworkStatus
	err := h.redisService.GetNetworkStatus(ctx, &status)
	if err != nil {
		h.logger.Warn("Network status not available", zap.Error(err))
		// Return empty status instead of error
		status = models.NetworkStatus{
			Status:    "unknown",
			Message:   "Status not available",
			LastCheck: time.Now(),
		}
	}

	h.SendSuccessResponse(w, http.StatusOK, status)
}

// Helper functions
func (h *ConfigHandlers) SendSuccessResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := models.APIResponse{
		Success: true,
		Data:    data,
		Meta: map[string]interface{}{
			"timestamp": time.Now().Unix(),
		},
	}

	json.NewEncoder(w).Encode(response)
}

func SendErrorResponse(w http.ResponseWriter, statusCode int, message, error string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := models.APIResponse{
		Success: false,
		Error: &models.ErrorResponse{
			Code:    statusCode,
			Message: message,
			Error:   error,
			Time:    time.Now(),
		},
	}

	json.NewEncoder(w).Encode(response)
}

func SendValidationErrorResponse(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)

	var validationErrors []models.ValidationError
	for _, err := range err.(validator.ValidationErrors) {
		validationErrors = append(validationErrors, models.ValidationError{
			Field:   err.Field(),
			Message: err.Error(),
			Code:    err.Tag(),
		})
	}

	response := models.APIResponse{
		Success: false,
		Error: &models.ErrorResponse{
			Code:    http.StatusBadRequest,
			Message: "Validation failed",
			Details: map[string]interface{}{
				"validation_errors": validationErrors,
			},
			Time: time.Now(),
		},
	}

	json.NewEncoder(w).Encode(response)
}

func generateID() string {
	// Simple ID generation - in production use UUID
	return "config_" + strconv.FormatInt(time.Now().UnixNano(), 36)
}

func marshalConfig(config *models.NetworkConfig) json.RawMessage {
	data, _ := json.Marshal(config)
	return data
}
