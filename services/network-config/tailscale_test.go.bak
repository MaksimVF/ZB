

package main

import (
	"testing"
	"net/http"
	"net/http/httptest"
	"encoding/json"
	"bytes"
)

func TestTailscaleConfiguration(t *testing.T) {
	// Test the Tailscale configuration endpoint
	t.Run("Test Tailscale Configuration", func(t *testing.T) {
		// Create a test request
		configJSON := `{
			"auth_key": "test-auth-key",
			"hostname": "test-hostname",
			"advertise_routes": "10.0.0.0/24"
		}`

		req, err := http.NewRequest("POST", "/api/tailscale/configure", bytes.NewBuffer([]byte(configJSON)))
		if err != nil {
			t.Fatal(err)
		}

		// Create a ResponseRecorder to record the response
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(configureTailscale)

		// Call the handler
		handler.ServeHTTP(rr, req)

		// Check the status code
		if status := rr.Code; status != http.StatusOK {
			t.Errorf("handler returned wrong status code: got %v want %v",
				status, http.StatusOK)
		}

		// Check the response body
		expected := "Tailscale configured successfully"
		if rr.Body.String() != expected {
			t.Errorf("handler returned unexpected body: got %v want %v",
				rr.Body.String(), expected)
		}
	})

	// Test the Tailscale status endpoint
	t.Run("Test Tailscale Status", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/api/tailscale/status", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(getTailscaleStatus)

		handler.ServeHTTP(rr, req)

		// If Tailscale is not installed, we expect an error
		if status := rr.Code; status != http.StatusInternalServerError {
			t.Errorf("handler returned wrong status code: got %v want %v",
				status, http.StatusInternalServerError)
		}
	})
}


