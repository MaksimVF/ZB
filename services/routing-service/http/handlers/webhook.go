package handlers

import (
	"encoding/json"
	"net/http"

	routing "github.com/MaksimVF/ZB/services/routing-service/routing"
)

// WebhookHandlers handles webhook endpoints
type WebhookHandlers struct {
	routingEngine *routing.RoutingEngine
	registry      routing.HeadRegistry
}

// NewWebhookHandlers creates new webhook handlers
func NewWebhookHandlers(engine *routing.RoutingEngine, registry routing.HeadRegistry) *WebhookHandlers {
	return &WebhookHandlers{
		routingEngine: engine,
		registry:      registry,
	}
}

// HeadStatusWebhook handles POST /webhook/head-status
func (h *WebhookHandlers) HeadStatusWebhook(w http.ResponseWriter, r *http.Request) {
	var webhookData struct {
		HeadID     string `json:"head_id"`
		Status     string `json:"status"`
		CurrentLoad int32  `json:"current_load"`
		Timestamp  int64  `json:"timestamp"`
	}

	if err := json.NewDecoder(r.Body).Decode(&webhookData); err != nil {
		http.Error(w, "Invalid webhook payload", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if webhookData.HeadID == "" || webhookData.Status == "" {
		http.Error(w, "Missing required fields: head_id and status", http.StatusBadRequest)
		return
	}

	// Update head status
	err := h.registry.UpdateStatus(webhookData.HeadID, webhookData.Status, webhookData.CurrentLoad, webhookData.Timestamp)
	if err != nil {
		http.Error(w, "Failed to update head status", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Webhook processed successfully"))
}

// RoutingDecisionWebhook handles POST /webhook/routing-decision
func (h *WebhookHandlers) RoutingDecisionWebhook(w http.ResponseWriter, r *http.Request) {
	var webhookData struct {
		ModelType       string            `json:"model_type"`
		RegionPreference string            `json:"region_preference"`
		RoutingStrategy  string            `json:"routing_strategy"`
		Metadata        map[string]string `json:"metadata"`
	}

	if err := json.NewDecoder(r.Body).Decode(&webhookData); err != nil {
		http.Error(w, "Invalid webhook payload", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if webhookData.ModelType == "" {
		http.Error(w, "Missing required field: model_type", http.StatusBadRequest)
		return
	}

	// Make routing decision
	req := &routing.RoutingRequest{
		ModelType:        webhookData.ModelType,
		RegionPreference: webhookData.RegionPreference,
		RoutingStrategy:  webhookData.RoutingStrategy,
		Metadata:         webhookData.Metadata,
	}

	decision, err := h.routingEngine.GetDecision(req)
	if err != nil {
		http.Error(w, "Failed to make routing decision", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(decision)
}

// ValidateWebhookRequest validates webhook request
func (h *WebhookHandlers) ValidateWebhookRequest(r *http.Request) error {
	// Check content type
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		return http.ErrContentType
	}

	// Check content length (prevent large payloads)
	if r.ContentLength > 1024*1024 { // 1MB limit
		return http.ErrBodyTooLarge
	}

	return nil
}

// LogWebhookEvent logs webhook events (in production, use structured logging)
func (h *WebhookHandlers) LogWebhookEvent(eventType, headID, status string) {
	// In production, this would log to a structured logger
	// For now, just a placeholder
	_ = eventType
	_ = headID
	_ = status
}