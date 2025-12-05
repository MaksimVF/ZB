


package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMonitoringMetrics(t *testing.T) {
	// Create a test router
	router := mux.NewRouter()

	// Apply middlewares
	router.Use(middleware.AuditLoggingMiddleware)

	// Add test endpoint
	router.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}).Methods("GET")

	// Test request
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer viewer-token")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "OK", w.Body.String())

	// Wait a bit for metrics to be recorded
	time.Sleep(100 * time.Millisecond)

	// Test metrics endpoint
	metricsReq := httptest.NewRequest("GET", "/metrics", nil)
	metricsW := httptest.NewRecorder()
	router.ServeHTTP(metricsW, metricsReq)

	// Check that metrics are present
	metricsBody := metricsW.Body.String()
	assert.Contains(t, metricsBody, "http_requests_total")
	assert.Contains(t, metricsBody, "http_request_duration_seconds")
	assert.Contains(t, metricsBody, "routing_decisions_total")
}

func TestAuditLogging(t *testing.T) {
	// Create a test router
	router := mux.NewRouter()

	// Apply audit middleware
	router.Use(middleware.AuditLoggingMiddleware)

	// Add test endpoint
	router.HandleFunc("/v1/admin/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}).Methods("POST")

	// Test sensitive operation (should trigger audit logging)
	req := httptest.NewRequest("POST", "/v1/admin/test", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "OK", w.Body.String())
}

