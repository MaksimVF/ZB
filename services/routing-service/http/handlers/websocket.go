package handlers

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	
	routing "github.com/MaksimVF/ZB/services/routing-service/routing"
	"github.com/MaksimVF/ZB/services/routing-service/http/middleware"
)

// WebSocketUpgrader config
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// TODO: Replace with a configurable list of allowed origins
		allowedOrigins := []string{"http://localhost:3000", "http://example.com"}
		origin := r.Header.Get("Origin")
		for _, allowedOrigin := range allowedOrigins {
			if origin == allowedOrigin {
				return true
			}
		}
		return false
	},
}

// WebSocketHandlers manages WebSocket connections
type WebSocketHandlers struct {
	routingEngine    *routing.RoutingEngine
	registry         routing.HeadRegistry
	headClients      map[*websocket.Conn]bool
	decisionClients  map[*websocket.Conn]bool
	clientsMutex     sync.Mutex
}

// NewWebSocketHandlers creates new WebSocket handlers
func NewWebSocketHandlers(engine *routing.RoutingEngine, registry routing.HeadRegistry) *WebSocketHandlers {
	return &WebSocketHandlers{
		routingEngine:   engine,
		registry:        registry,
		headClients:     make(map[*websocket.Conn]bool),
		decisionClients: make(map[*websocket.Conn]bool),
	}
}

// HeadManagementWebSocket handles WebSocket connections for head management
func (h *WebSocketHandlers) HeadManagementWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return // Connection upgrade failed
	}
	defer conn.Close()

	// Register client
	h.clientsMutex.Lock()
	h.headClients[conn] = true
	h.clientsMutex.Unlock()

	// Remove client on disconnect
	defer func() {
		h.clientsMutex.Lock()
		delete(h.headClients, conn)
		h.clientsMutex.Unlock()
	}()

	// Handle WebSocket messages
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			break // Connection closed or error
		}

		var request struct {
			Type    string                 `json:"type"`
			Payload map[string]interface{} `json:"payload"`
		}

		if err := json.Unmarshal(message, &request); err != nil {
			h.sendError(conn, "Failed to parse message")
			continue
		}

		// Handle different message types
		switch request.Type {
		case "register_head":
			h.handleWebSocketHeadRegistration(conn, request.Payload)
		case "update_status":
			h.handleWebSocketStatusUpdate(conn, request.Payload)
		case "get_heads":
			h.handleWebSocketGetHeads(conn)
		case "ping":
			h.sendPong(conn)
		default:
			h.sendError(conn, "Unknown message type")
		}
	}
}

// RoutingDecisionsWebSocket handles WebSocket connections for routing decisions
func (h *WebSocketHandlers) RoutingDecisionsWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return // Connection upgrade failed
	}
	defer conn.Close()

	// Register client
	h.clientsMutex.Lock()
	h.decisionClients[conn] = true
	h.clientsMutex.Unlock()

	// Remove client on disconnect
	defer func() {
		h.clientsMutex.Lock()
		delete(h.decisionClients, conn)
		h.clientsMutex.Unlock()
	}()

	// Handle WebSocket messages
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			break // Connection closed or error
		}

		var request struct {
			Type    string                 `json:"type"`
			Payload map[string]interface{} `json:"payload"`
		}

		if err := json.Unmarshal(message, &request); err != nil {
			h.sendError(conn, "Failed to parse message")
			continue
		}

		// Handle different message types
		switch request.Type {
		case "get_routing_decision":
			h.handleWebSocketRoutingDecision(conn, request.Payload)
		case "get_routing_strategies":
			h.handleWebSocketGetRoutingStrategies(conn)
		case "ping":
			h.sendPong(conn)
		default:
			h.sendError(conn, "Unknown message type")
		}
	}
}

// handleWebSocketHeadRegistration handles head registration via WebSocket
func (h *WebSocketHandlers) handleWebSocketHeadRegistration(conn *websocket.Conn, payload map[string]interface{}) {
	// Validate payload
	headID, ok := payload["head_id"].(string)
	if !ok {
		h.sendError(conn, "Invalid head_id")
		return
	}

	endpoint, ok := payload["endpoint"].(string)
	if !ok {
		h.sendError(conn, "Invalid endpoint")
		return
	}

	modelType, ok := payload["model_type"].(string)
	if !ok {
		h.sendError(conn, "Invalid model_type")
		return
	}

	region, ok := payload["region"].(string)
	if !ok {
		h.sendError(conn, "Invalid region")
		return
	}

	status, ok := payload["status"].(string)
	if !ok {
		status = "active"
	}

	// Create head service
	head := routing.HeadService{
		HeadID:        headID,
		Endpoint:      endpoint,
		ModelType:     modelType,
		Region:        region,
		Status:        status,
		LastHeartbeat: time.Now().Unix(),
		Metadata:      make(map[string]string),
	}

	// Convert metadata if present
	if metadata, ok := payload["metadata"].(map[string]interface{}); ok {
		for k, v := range metadata {
			if strValue, ok := v.(string); ok {
				head.Metadata[k] = strValue
			}
		}
	}

	// Register the head
	err := h.registry.Register(head)
	if err != nil {
		h.sendError(conn, "Failed to register head: "+err.Error())
		return
	}

	// Send success response
	response := map[string]interface{}{
		"type":    "register_head_response",
		"success": true,
		"message": "Head registered successfully",
		"head_id": headID,
	}
	conn.WriteJSON(response)
}

// handleWebSocketStatusUpdate handles status update via WebSocket
func (h *WebSocketHandlers) handleWebSocketStatusUpdate(conn *websocket.Conn, payload map[string]interface{}) {
	// Validate payload
	headID, ok := payload["head_id"].(string)
	if !ok {
		h.sendError(conn, "Invalid head_id")
		return
	}

	status, ok := payload["status"].(string)
	if !ok {
		h.sendError(conn, "Invalid status")
		return
	}

	currentLoad, ok := payload["current_load"].(float64)
	if !ok {
		h.sendError(conn, "Invalid current_load")
		return
	}

	timestamp := int64(payload["timestamp"].(float64))
	if timestamp == 0 {
		timestamp = time.Now().Unix()
	}

	// Update head status
	err := h.registry.UpdateStatus(headID, status, int32(currentLoad), timestamp)
	if err != nil {
		h.sendError(conn, "Failed to update status: "+err.Error())
		return
	}

	// Send success response
	response := map[string]interface{}{
		"type":    "update_status_response",
		"success": true,
		"message": "Status updated successfully",
		"head_id": headID,
	}
	conn.WriteJSON(response)
}

// handleWebSocketGetHeads handles get heads request via WebSocket
func (h *WebSocketHandlers) handleWebSocketGetHeads(conn *websocket.Conn) {
	heads := h.registry.GetAll()
	
	response := map[string]interface{}{
		"type":  "get_heads_response",
		"heads": heads,
	}
	conn.WriteJSON(response)
}

// handleWebSocketRoutingDecision handles routing decision request via WebSocket
func (h *WebSocketHandlers) handleWebSocketRoutingDecision(conn *websocket.Conn, payload map[string]interface{}) {
	// Validate payload
	modelType, ok := payload["model_type"].(string)
	if !ok {
		h.sendError(conn, "Invalid model_type")
		return
	}

	regionPreference := ""
	if rp, ok := payload["region_preference"].(string); ok {
		regionPreference = rp
	}

	routingStrategy := ""
	if rs, ok := payload["routing_strategy"].(string); ok {
		routingStrategy = rs
	}

	// Create routing request
	req := &routing.RoutingRequest{
		ModelType:        modelType,
		RegionPreference: regionPreference,
		RoutingStrategy:  routingStrategy,
		Metadata:         make(map[string]string),
	}

	// Convert metadata if present
	if metadata, ok := payload["metadata"].(map[string]interface{}); ok {
		for k, v := range metadata {
			if strValue, ok := v.(string); ok {
				req.Metadata[k] = strValue
			}
		}
	}

	// Get routing decision
	decision, err := h.routingEngine.GetDecision(req)
	if err != nil {
		h.sendError(conn, "Failed to get routing decision: "+err.Error())
		return
	}

	// Send success response
	response := map[string]interface{}{
		"type":           "routing_decision_response",
		"head_id":        decision.HeadID,
		"endpoint":       decision.Endpoint,
		"strategy_used":  decision.StrategyUsed,
		"reason":         decision.Reason,
		"metadata":       decision.Metadata,
	}
	conn.WriteJSON(response)
}

// handleWebSocketGetRoutingStrategies handles get routing strategies request
func (h *WebSocketHandlers) handleWebSocketGetRoutingStrategies(conn *websocket.Conn) {
	policy := h.registry.GetAll() // This would be replaced with actual policy retrieval
	
	response := map[string]interface{}{
		"type":             "get_routing_strategies_response",
		"default_strategy": "adaptive",
		"available_strategies": []string{
			"round_robin",
			"least_loaded", 
			"geo_preferred",
			"model_specific",
			"predictive",
			"adaptive",
			"hybrid",
		},
	}
	conn.WriteJSON(response)
}

// sendError sends an error message to the WebSocket client
func (h *WebSocketHandlers) sendError(conn *websocket.Conn, message string) {
	response := map[string]interface{}{
		"type":    "error",
		"message": message,
	}
	conn.WriteJSON(response)
}

// sendPong sends a pong response to keep the connection alive
func (h *WebSocketHandlers) sendPong(conn *websocket.Conn) {
	conn.WriteMessage(websocket.PongMessage, []byte("pong"))
}

// BroadcastHeadUpdate broadcasts head status update to all connected clients
func (h *WebSocketHandlers) BroadcastHeadUpdate(headID, status string) {
	h.clientsMutex.Lock()
	defer h.clientsMutex.Unlock()

	message := map[string]interface{}{
		"type":     "head_update",
		"head_id":  headID,
		"status":   status,
		"timestamp": time.Now().Unix(),
	}

	for client := range h.headClients {
		client.WriteJSON(message)
	}
}

// BroadcastRoutingDecision broadcasts routing decision to all connected clients
func (h *WebSocketHandlers) BroadcastRoutingDecision(decision *routing.RoutingResponse) {
	h.clientsMutex.Lock()
	defer h.clientsMutex.Unlock()

	message := map[string]interface{}{
		"type":           "routing_decision",
		"head_id":        decision.HeadID,
		"endpoint":       decision.Endpoint,
		"strategy_used":  decision.StrategyUsed,
		"reason":         decision.Reason,
		"timestamp":      time.Now().Unix(),
	}

	for client := range h.decisionClients {
		client.WriteJSON(message)
	}
}