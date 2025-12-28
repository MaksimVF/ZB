package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/MaksimVF/ZB/services/routing-service/routing"
)

// AdminHandlers handles admin API endpoints
type AdminHandlers struct {
	routingEngine *routing.RoutingEngine
	registry      routing.HeadRegistry
	policyManager routing.PolicyManager
}

// NewAdminHandlers creates new admin handlers
func NewAdminHandlers(engine *routing.RoutingEngine, registry routing.HeadRegistry, policyManager routing.PolicyManager) *AdminHandlers {
	return &AdminHandlers{
		routingEngine: engine,
		registry:      registry,
		policyManager: policyManager,
	}
}

// GetRoutingPolicy handles GET /api/routing/policy
func (h *AdminHandlers) GetRoutingPolicy(w http.ResponseWriter, r *http.Request) {
	policy := h.policyManager.Get()
	json.NewEncoder(w).Encode(policy)
}

// UpdateRoutingPolicy handles PUT /api/routing/policy
func (h *AdminHandlers) UpdateRoutingPolicy(w http.ResponseWriter, r *http.Request) {
	var policy routing.RoutingPolicy
	if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.policyManager.Update(&policy); err != nil {
		http.Error(w, "Failed to update policy", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(policy)
}

// GetAllHeads handles GET /api/routing/heads
func (h *AdminHandlers) GetAllHeads(w http.ResponseWriter, r *http.Request) {
	heads := h.registry.GetAll()
	json.NewEncoder(w).Encode(heads)
}

// RegisterHead handles POST /api/routing/heads
func (h *AdminHandlers) RegisterHead(w http.ResponseWriter, r *http.Request) {
	var head routing.HeadService
	if err := json.NewDecoder(r.Body).Decode(&head); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.registry.Register(head); err != nil {
		http.Error(w, "Failed to register head", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(head)
}

// HealthCheck handles GET /health
func (h *AdminHandlers) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

// RequireRole creates a middleware for role-based access
func (h *AdminHandlers) RequireRole(requiredRole string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Simple role check - in production, extract from JWT
			if requiredRole == "admin" {
				next.ServeHTTP(w, r)
			} else {
				http.Error(w, "Forbidden", http.StatusForbidden)
			}
		})
	}
}
