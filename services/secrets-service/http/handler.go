package http

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/MaksimVF/ZB/services/secrets-service/core"
)

// Handler представляет HTTP обработчик для админ API
type Handler struct {
	secretService *core.SecretService
	logger        *Logger
	config        *Config
}

// New создает новый HTTP обработчик
func New(secretService *core.SecretService, logger *Logger, config *Config) *Handler {
	return &Handler{
		secretService: secretService,
		logger:        logger,
		config:        config,
	}
}

// AdminCreateSecret обрабатывает POST запрос на создание секрета
func (h *Handler) AdminCreateSecret(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	h.logger.Info().
		Str("method", "AdminCreateSecret").
		Str("http_method", r.Method).
		Str("path", r.URL.Path).
		Msg("Received admin create secret request")

	// Декодирование запроса
	var req CreateSecretRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error().Err(err).Msg("Failed to decode request body")
		h.sendError(w, r, http.StatusBadRequest, "invalid request body: %v", err)
		return
	}

	// Создание секрета через core сервис
	err := h.secretService.AdminCreateSecret(r.Context(), req.Path, req.Value)
	if err != nil {
		h.logger.Error().Err(err).Str("secret_path", req.Path).Msg("Failed to create secret")
		h.sendError(w, r, http.StatusInternalServerError, "failed to create secret: %v", err)
		return
	}

	// Успешный ответ
	h.sendSuccess(w, r, SecretOperationResult{
		Success: true,
		Message: "Secret created successfully",
	})
}

// AdminDeleteSecret обрабатывает DELETE запрос на удаление секрета
func (h *Handler) AdminDeleteSecret(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	h.logger.Info().
		Str("method", "AdminDeleteSecret").
		Str("http_method", r.Method).
		Str("path", r.URL.Path).
		Msg("Received admin delete secret request")

	// Получение имени секрета из URL
	secretName := r.URL.Path[len("/admin/api/secrets/"):]
	if secretName == "" {
		h.logger.Error().Msg("Missing secret name in delete request")
		h.sendError(w, r, http.StatusBadRequest, "secret name is required")
		return
	}

	// Удаление секрета через core сервис
	err := h.secretService.AdminDeleteSecret(r.Context(), secretName)
	if err != nil {
		h.logger.Error().Err(err).Str("secret_name", secretName).Msg("Failed to delete secret")
		h.sendError(w, r, http.StatusInternalServerError, "failed to delete secret: %v", err)
		return
	}

	// Успешный ответ
	h.sendSuccess(w, r, SecretOperationResult{
		Success: true,
		Message: "Secret deleted successfully",
	})
}

// AdminListSecrets обрабатывает GET запрос на получение списка секретов
func (h *Handler) AdminListSecrets(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	h.logger.Info().
		Str("method", "AdminListSecrets").
		Str("http_method", r.Method).
		Str("path", r.URL.Path).
		Msg("Received admin list secrets request")

	// Получение секретов через core сервис
	secrets, err := h.secretService.AdminListSecrets(r.Context(), "llm")
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to list secrets")
		h.sendError(w, r, http.StatusInternalServerError, "failed to list secrets: %v", err)
		return
	}

	// Успешный ответ
	h.sendSuccess(w, r, SecretList{
		Keys: secrets,
	})
}

// sendSuccess отправляет успешный ответ
func (h *Handler) sendSuccess(w http.ResponseWriter, r *http.Request, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error().Err(err).Msg("Failed to encode success response")
	}
}

// sendError отправляет ответ с ошибкой
func (h *Handler) sendError(w http.ResponseWriter, r *http.Request, statusCode int, format string, args ...any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	errorMsg := format
	if len(args) > 0 {
		errorMsg = format // Простое форматирование
	}

	response := map[string]string{
		"error": errorMsg,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error().Err(err).Msg("Failed to encode error response")
	}
}
