





package main

import (
	"log"
	"llm-gateway-pro/services/rate-limiter/internal/server"
)

func main() {
	rateLimiter := server.NewRateLimiterServer()
	if err := rateLimiter.Run(); err != nil {
		log.Fatalf("Failed to run rate limiter: %v", err)
	}
}





