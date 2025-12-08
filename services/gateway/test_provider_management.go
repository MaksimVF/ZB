


package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestProviderManagement(t *testing.T) {
	// Start the server in a separate goroutine
	go main()

	// Wait for server to start
	time.Sleep(2 * time.Second)

	// Test 1: List providers
	resp, err := http.Get("http://localhost:8080/v1/providers")
	if err != nil {
		t.Errorf("Failed to list providers: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Test 2: Add a new provider
	newProvider := map[string]interface{}{
		"base_url":       "https://api.newprovider.com",
		"api_key":        "test-api-key",
		"model_names":     []string{"new-model-1", "new-model-2"},
		"weight":          2,
		"max_concurrency": 5,
	}

	body, _ := json.Marshal(newProvider)
	resp, err = http.Post("http://localhost:8080/v1/providers", "application/json", bytes.NewBuffer(body))
	if err != nil {
		t.Errorf("Failed to add provider: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", resp.StatusCode)
	}

	// Test 3: List providers again to verify the new provider was added
	resp, err = http.Get("http://localhost:8080/v1/providers")
	if err != nil {
		t.Errorf("Failed to list providers: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Test 4: Remove the new provider
	req, _ := http.NewRequest("DELETE", "http://localhost:8080/v1/providers/newprovider", nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Errorf("Failed to remove provider: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	fmt.Println("All provider management tests passed!")
}


