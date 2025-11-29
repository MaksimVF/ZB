





package main

import (
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"llm-gateway-pro/services/gateway/internal/handlers"
	"llm-gateway-pro/services/gateway/internal/billing"
)

var logger zerolog.Logger

func init() {
	// Initialize structured logger
	logger = zerolog.New(os.Stdout).With().
		Timestamp().
		Str("service", "gateway").
		Logger()
}

func main() {
	// Initialize Prometheus metrics
	handlers.InitMetrics()

	// Initialize billing system
	dbConnString := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
		os.Getenv("DB_PORT"),
	)

	err := billing.Init(dbConnString)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize billing system")
	}
	defer billing.Close()

	r := mux.NewRouter()

	// LangChain-specific endpoint
	r.HandleFunc("/v1/langchain/chat/completions", handlers.LangChainCompletion).Methods("POST")

	// Standard OpenAI-compatible endpoint
	r.HandleFunc("/v1/chat/completions", handlers.ChatCompletion).Methods("POST")

	// Health check
	r.HandleFunc("/health", HealthCheck).Methods("GET")

	// Metrics endpoint
	r.Handle("/metrics", promhttp.Handler())

	logger.Info().Msg("Gateway service starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}

func HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"healthy"}`))
}



