





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
	"llm-gateway-pro/services/gateway/internal/providers"
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

	// Initialize LiteLLM providers
	providerConfig := providers.LiteLLMConfig{
		Providers: map[string]providers.ProviderConfig{
			"openai": {
				BaseURL:    "https://api.openai.com",
				APIKey:     os.Getenv("OPENAI_API_KEY"),
				ModelNames: []string{"gpt-4", "gpt-3.5-turbo", "gpt-4o"},
			},
			"anthropic": {
				BaseURL:    "https://api.anthropic.com",
				APIKey:     os.Getenv("ANTHROPIC_API_KEY"),
				ModelNames: []string{"claude-3", "claude-2", "claude-instant"},
			},
			"google": {
				BaseURL:    "https://api.google.com",
				APIKey:     os.Getenv("GOOGLE_API_KEY"),
				ModelNames: []string{"gemini-1.5", "gemini-1.0", "gemini-pro"},
			},
			"meta": {
				BaseURL:    "https://api.meta.com",
				APIKey:     os.Getenv("META_API_KEY"),
				ModelNames: []string{"llama-3", "llama-2", "llama-1"},
			},
		},
	}

	providers.Init(providerConfig)

	r := mux.NewRouter()

	// LangChain-specific endpoint
	r.HandleFunc("/v1/langchain/chat/completions", handlers.LangChainCompletion).Methods("POST")

	// Standard OpenAI-compatible endpoint
	r.HandleFunc("/v1/chat/completions", handlers.ChatCompletion).Methods("POST")

	// Provider management endpoints
	r.HandleFunc("/v1/providers", handlers.ListProviders).Methods("GET")
	r.HandleFunc("/v1/providers", handlers.AddProvider).Methods("POST")
	r.HandleFunc("/v1/providers/{provider}", handlers.RemoveProvider).Methods("DELETE")

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



