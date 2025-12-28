package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/MaksimVF/ZB/services/routing-service/routing"
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
	var payload struct {
		HeadID      string `json:"head_id"`
		Status      string `json:"status"`
		CurrentLoad int32  `json:"current_load"`
		Timestamp   int64  `json:"timestamp"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	if err := h.registry.UpdateStatus(payload.HeadID, payload.Status, payload.CurrentLoad, payload.Timestamp); err != nil {
		http.Error(w, "Failed to update head status", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// RoutingDecisionWebhook handles POST /webhook/routing-decision
func (h *WebhookHandlers) RoutingDecisionWebhook(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		ModelType        string            `json:"model_type"`
		RegionPreference string            `json:"region_preference"`
		RoutingStrategy  string            `json:"routing_strategy"`
		Metadata         map[string]string `json:"metadata"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	req := &routing.RoutingRequest{
		ClientID:         "",
		ModelType:        payload.ModelType,
		RegionPreference: payload.RegionPreference,
		RoutingStrategy:  payload.RoutingStrategy,
		Metadata:         payload.Metadata,
	}

	resp, err := h.routingEngine.GetDecision(req)
	if err != nil {
		http.Error(w, "Failed to get routing decision", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"head_id":       resp.HeadID,
		"endpoint":      resp.Endpoint,
		"strategy_used": resp.StrategyUsed,
		"reason":        resp.Reason,
		"metadata":      resp.Metadata,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
