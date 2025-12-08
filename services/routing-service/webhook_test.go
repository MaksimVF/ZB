
package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
)

func TestWebhookSecurity(t *testing.T) {
	// Create a test router
	router := mux.NewRouter()
	router.Handle("/webhook/head-status", webhookSecurityMiddleware(rateLimitMiddleware(http.HandlerFunc(handleHeadStatusWebhook)))).Methods("POST")
	router.Handle("/webhook/routing-decision", webhookSecurityMiddleware(rateLimitMiddleware(http.HandlerFunc(handleRoutingDecisionWebhook)))).Methods("POST")

	// Test case 1: Missing authorization header
	req := httptest.NewRequest("POST", "/webhook/head-status", bytes.NewReader([]byte(`{"head_id":"test","status":"active","current_load":10,"timestamp":1234567890}`)))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}

	// Test case 2: Invalid token format
	req = httptest.NewRequest("POST", "/webhook/head-status", bytes.NewReader([]byte(`{"head_id":"test","status":"active","current_load":10,"timestamp":1234567890}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer invalid-token")
	req.Header.Set("X-App-Signature", "app-sig-valid")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}

	// Test case 3: Missing app signature
	req = httptest.NewRequest("POST", "/webhook/head-status", bytes.NewReader([]byte(`{"head_id":"test","status":"active","current_load":10,"timestamp":1234567890}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer webhook-token")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}

	// Test case 4: Valid request
	req = httptest.NewRequest("POST", "/webhook/head-status", bytes.NewReader([]byte(`{"head_id":"test","status":"active","current_load":10,"timestamp":1234567890}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer webhook-token")
	req.Header.Set("X-App-Signature", "app-sig-valid")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}
