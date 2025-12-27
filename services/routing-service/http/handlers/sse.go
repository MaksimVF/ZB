package handlers

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	routing "github.com/MaksimVF/ZB/services/routing-service/routing"
)

// SSEHandlers manages Server-Sent Events connections
type SSEHandlers struct {
	routingEngine       *routing.RoutingEngine
	registry            routing.HeadRegistry
	headStatusClients   []chan string
	routingDecisionClients []chan string
	clientsMutex        sync.Mutex
}

// NewSSEHandlers creates new SSE handlers
func NewSSEHandlers(engine *routing.RoutingEngine, registry routing.HeadRegistry) *SSEHandlers {
	return &SSEHandlers{
		routingEngine:       engine,
		registry:            registry,
		headStatusClients:   make([]chan string, 0),
		routingDecisionClients: make([]chan string, 0),
	}
}

// HeadStatusEvents handles SSE for head status updates
func (h *SSEHandlers) HeadStatusEvents(w http.ResponseWriter, r *http.Request) {
	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Create a channel to send events
	eventChan := make(chan string, 10) // Buffered channel

	// Register the client
	h.clientsMutex.Lock()
	h.headStatusClients = append(h.headStatusClients, eventChan)
	h.clientsMutex.Unlock()

	// Remove client on disconnect
	defer func() {
		h.clientsMutex.Lock()
		for i, client := range h.headStatusClients {
			if client == eventChan {
				h.headStatusClients = append(h.headStatusClients[:i], h.headStatusClients[i+1:]...)
				break
			}
		}
		h.clientsMutex.Unlock()
		close(eventChan)
	}()

	// Send initial connection event
	fmt.Fprintf(w, "data: %s\n\n", `{"type":"connected","timestamp":`+fmt.Sprint(time.Now().Unix())+`}`)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	// Listen for events
	for {
		select {
		case event := <-eventChan:
			fmt.Fprintf(w, "data: %s\n\n", event)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		case <-r.Context().Done():
			return
		}
	}
}

// RoutingDecisionEvents handles SSE for routing decision updates
func (h *SSEHandlers) RoutingDecisionEvents(w http.ResponseWriter, r *http.Request) {
	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Create a channel to send events
	eventChan := make(chan string, 10) // Buffered channel

	// Register the client
	h.clientsMutex.Lock()
	h.routingDecisionClients = append(h.routingDecisionClients, eventChan)
	h.clientsMutex.Unlock()

	// Remove client on disconnect
	defer func() {
		h.clientsMutex.Lock()
		for i, client := range h.routingDecisionClients {
			if client == eventChan {
				h.routingDecisionClients = append(h.routingDecisionClients[:i], h.routingDecisionClients[i+1:]...)
				break
			}
		}
		h.clientsMutex.Unlock()
		close(eventChan)
	}()

	// Send initial connection event
	fmt.Fprintf(w, "data: %s\n\n", `{"type":"connected","timestamp":`+fmt.Sprint(time.Now().Unix())+`}`)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	// Listen for events
	for {
		select {
		case event := <-eventChan:
			fmt.Fprintf(w, "data: %s\n\n", event)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		case <-r.Context().Done():
			return
		}
	}
}

// BroadcastHeadStatus broadcasts head status update to all SSE clients
func (h *SSEHandlers) BroadcastHeadStatus(headID, status string, load int32) {
	h.clientsMutex.Lock()
	defer h.clientsMutex.Unlock()

	event := fmt.Sprintf(`{
		"type": "head_status",
		"head_id": "%s",
		"status": "%s", 
		"current_load": %d,
		"timestamp": %d
	}`, headID, status, load, time.Now().Unix())

	for _, client := range h.headStatusClients {
		select {
		case client <- event:
		default:
			// Client channel is full, skip this event
		}
	}
}

// BroadcastRoutingDecision broadcasts routing decision to all SSE clients
func (h *SSEHandlers) BroadcastRoutingDecision(decision *routing.RoutingResponse) {
	h.clientsMutex.Lock()
	defer h.clientsMutex.Unlock()

	event := fmt.Sprintf(`{
		"type": "routing_decision",
		"head_id": "%s",
		"endpoint": "%s",
		"strategy_used": "%s",
		"reason": "%s",
		"timestamp": %d
	}`, decision.HeadID, decision.Endpoint, decision.StrategyUsed, decision.Reason, time.Now().Unix())

	for _, client := range h.routingDecisionClients {
		select {
		case client <- event:
		default:
			// Client channel is full, skip this event
		}
	}
}

// GetConnectedClientsCount returns the number of connected SSE clients
func (h *SSEHandlers) GetConnectedClientsCount() (int, int) {
	h.clientsMutex.Lock()
	defer h.clientsMutex.Unlock()
	
	return len(h.headStatusClients), len(h.routingDecisionClients)
}

// SendHeartbeat sends periodic heartbeat to keep connections alive
func (h *SSEHandlers) SendHeartbeat() {
	h.clientsMutex.Lock()
	defer h.clientsMutex.Unlock()

	heartbeat := fmt.Sprintf(`{"type":"heartbeat","timestamp":%d}`, time.Now().Unix())

	// Send to head status clients
	for _, client := range h.headStatusClients {
		select {
		case client <- heartbeat:
		default:
		}
	}

	// Send to routing decision clients
	for _, client := range h.routingDecisionClients {
		select {
		case client <- heartbeat:
		default:
		}
	}
}