package handlers

import (
	"fmt"
	"net/http"

	"github.com/MaksimVF/ZB/services/routing-service/routing"
)

// SSEHandlers handles Server-Sent Events
type SSEHandlers struct {
	routingEngine *routing.RoutingEngine
	registry      routing.HeadRegistry
}

// NewSSEHandlers creates new SSE handlers
func NewSSEHandlers(engine *routing.RoutingEngine, registry routing.HeadRegistry) *SSEHandlers {
	return &SSEHandlers{
		routingEngine: engine,
		registry:      registry,
	}
}

// HeadStatusEvents handles GET /events/head-status
func (h *SSEHandlers) HeadStatusEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Send initial data
	heads := h.registry.GetAll()
	for _, head := range heads {
		fmt.Fprintf(w, "data: %s\n\n", fmt.Sprintf(`{"type": "head_status", "data": {"head_id": "%s", "status": "%s", "load": %d}}`, head.HeadID, head.Status, head.CurrentLoad))
		w.(http.Flusher).Flush()
	}

	// Keep connection alive (in production, this would be more sophisticated)
	select {}
}

// RoutingDecisionEvents handles GET /events/routing-decisions
func (h *SSEHandlers) RoutingDecisionEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Send initial message
	fmt.Fprintf(w, "data: %s\n\n", `{"type": "connected", "message": "Connected to routing decision events"}`)
	w.(http.Flusher).Flush()

	// Keep connection alive
	select {}
}

// SendHeartbeat sends heartbeat to all SSE clients
func (h *SSEHandlers) SendHeartbeat() {
	// In production, this would broadcast to all connected clients
}
