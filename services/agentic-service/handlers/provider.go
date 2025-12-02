









package handlers

import (
	"encoding/json"
	"net/http"

	"llm-gateway-pro/services/agentic-service/internal"
)

// GetProviders returns the list of all configured providers
func GetProviders(w http.ResponseWriter, r *http.Request) {
	providers, err := internal.GetAllProviders()
	if err != nil {
		http.Error(w, `{"error":"failed to get providers"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(providers)
}

// GetProviderHealth returns the health status of all providers
func GetProviderHealth(w http.ResponseWriter, r *http.Request) {
	health, err := internal.GetProviderHealth()
	if err != nil {
		http.Error(w, `{"error":"failed to get provider health"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

// AddProvider adds a new provider configuration
func AddProvider(w http.ResponseWriter, r *http.Request) {
	var config internal.ProviderConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, `{"error":"invalid provider configuration"}`, http.StatusBadRequest)
		return
	}

	provider := r.URL.Query().Get("provider")
	if provider == "" {
		http.Error(w, `{"error":"provider name required"}`, http.StatusBadRequest)
		return
	}

	if err := internal.AddProvider(provider, config); err != nil {
		http.Error(w, `{"error":"failed to add provider"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "provider added"})
}

// RemoveProvider removes a provider configuration
func RemoveProvider(w http.ResponseWriter, r *http.Request) {
	provider := r.URL.Query().Get("provider")
	if provider == "" {
		http.Error(w, `{"error":"provider name required"}`, http.StatusBadRequest)
		return
	}

	if err := internal.RemoveProvider(provider); err != nil {
		http.Error(w, `{"error":"failed to remove provider"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "provider removed"})
}

// UpdateProviderAPIKey updates the API key for a provider
func UpdateProviderAPIKey(w http.ResponseWriter, r *http.Request) {
	provider := r.URL.Query().Get("provider")
	if provider == "" {
		http.Error(w, `{"error":"provider name required"}`, http.StatusBadRequest)
		return
	}

	var req struct {
		APIKey string `json:"api_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	if err := internal.SetProviderAPIKey(provider, req.APIKey); err != nil {
		http.Error(w, `{"error":"failed to update API key"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "API key updated"})
}









