



package handlers

import (
	"bytes"
	"io"
	"net/http"
	"time"
)

func ProxyAgenticRequest(w http.ResponseWriter, r *http.Request) {
	// Create a timeout context for the request
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Forward the request to the agentic service
	req, err := http.NewRequestWithContext(ctx, "POST", "http://agentic-service:8081/v1/agentic", r.Body)
	if err != nil {
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	// Copy headers
	for name, values := range r.Header {
		for _, value := range values {
			req.Header.Add(name, value)
		}
	}

	// Make the request to the agentic service
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, `{"error":"agentic service unavailable"}`, http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for name, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}

	// Copy status code
	w.WriteHeader(resp.StatusCode)

	// Copy response body
	io.Copy(w, resp.Body)
}



