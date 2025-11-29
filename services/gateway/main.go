



package main

import (
	"log"
	"net/http"
	"llm-gateway-pro/services/gateway/handlers"
	"llm-gateway-pro/services/gateway/middleware"
)

func main() {
	mux := http.NewServeMux()

	// Все роуты с rate-limiter
	mux.HandleFunc("POST /v1/chat/completions", middleware.RateLimiter(handlers.ChatCompletion))
	mux.HandleFunc("POST /v1/completions", middleware.RateLimiter(handlers.ChatCompletion))
	mux.HandleFunc("POST /v1/embeddings", middleware.RateLimiter(handlers.Embeddings))
	mux.HandleFunc("POST /v1/agentic", middleware.RateLimiter(handlers.AgenticHandler))

	// Health-check без лимита
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	log.Println("Gateway running on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}



