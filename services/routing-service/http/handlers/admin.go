package handlers

import (
	"encoding/json"
	"net/http"

	routing "github.com/MaksimVF/ZB/services/routing-service/routing"
	"github.com/MaksimVF/ZB/services/routing-service/http/middleware"
)

// AdminHandlers handles admin API endpoints
type AdminHandlers struct {
	routingEngine *routing.RoutingEngine
	registry      routing.HeadRegistry
	policyMgr     routing.PolicyManager
}

// NewAdminHandlers creates new admin handlers
func NewAdminHandlers(engine *routing.RoutingEngine, registry routing.HeadRegistry, policyMgr routing.PolicyManager) *AdminHandlers {
	return &AdminHandlers{
		routingEngine: engine,
		registry:      registry,
		policyMgr:     policyMgr,
	}
}

// GetRoutingPolicy handles GET /api/routing/policy
func (h *AdminHandlers) GetRoutingPolicy(w http.ResponseWriter, r *http.Request) {
	policy := h.policyMgr.Get()
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(policy)
}

// UpdateRoutingPolicy handles PUT /api/routing/policy
func (h *AdminHandlers) UpdateRoutingPolicy(w http.ResponseWriter, r *http.Request) {
	var policy routing.RoutingPolicy
	if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	if err := h.policyMgr.Update(&policy); err != nil {
		http.Error(w, "Failed to update policy", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(policy)
}

// GetAllHeads handles GET /api/routing/heads
func (h *AdminHandlers) GetAllHeads(w http.ResponseWriter, r *http.Request) {
	heads := h.registry.GetAll()
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(heads)
}

// RegisterHead handles POST /api/routing/heads
func (h *AdminHandlers) RegisterHead(w http.ResponseWriter, r *http.Request) {
	var head routing.HeadService
	if err := json.NewDecoder(r.Body).Decode(&head); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	if err := h.registry.Register(head); err != nil {
		http.Error(w, "Failed to register head", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "registered", "head_id": head.HeadID})
}

// GetRoutingDecision handles GET /api/routing/decision
func (h *AdminHandlers) GetRoutingDecision(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	modelType := r.URL.Query().Get("model_type")
	if modelType == "" {
		http.Error(w, "model_type parameter is required", http.StatusBadRequest)
		return
	}

	regionPreference := r.URL.Query().Get("region_preference")
	routingStrategy := r.URL.Query().Get("strategy")

	req := &routing.RoutingRequest{
		ModelType:        modelType,
		RegionPreference: regionPreference,
		RoutingStrategy:  routingStrategy,
		Metadata:         make(map[string]string),
	}

	response, err := h.routingEngine.GetDecision(req)
	if err != nil {
		http.Error(w, "Failed to get routing decision", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HealthCheck handles GET /health
func (h *AdminHandlers) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

// RequireRole creates middleware that requires specific role
func (h *AdminHandlers) RequireRole(role middleware.UserRole) func(http.Handler) http.Handler {
	return middleware.RBACMiddleware(role)
}

// GetUserContext retrieves user context from request
func (h *AdminHandlers) GetUserContext(r *http.Request) (middleware.UserContext, bool) {
	return middleware.GetUserContext(r)
}