

package main

import (
	"testing"
)

func TestPredictiveLoadBalancing(t *testing.T) {
	// Create test heads
	heads := []HeadService{
		{
			HeadID:      "head1",
			Endpoint:    "grpc://head1:50055",
			Status:      "active",
			CurrentLoad: 30,
			Region:      "us-east-1",
			ModelType:   "llama-3",
			LoadHistory: []int32{20, 25, 30, 35, 40},
		},
		{
			HeadID:      "head2",
			Endpoint:    "grpc://head2:50055",
			Status:      "active",
			CurrentLoad: 50,
			Region:      "us-east-1",
			ModelType:   "llama-3",
			LoadHistory: []int32{40, 45, 50, 55, 60},
		},
	}

	// Test predictive load balancing
	selectedHead := applyPredictiveLoadBalancing(heads)
	if selectedHead == nil {
		t.Errorf("Expected a head to be selected, got nil")
	}

	// Should select head1 as it has lower predicted load
	if selectedHead.HeadID != "head1" {
		t.Errorf("Expected head1 to be selected, got %s", selectedHead.HeadID)
	}
}

func TestAdaptiveRouting(t *testing.T) {
	// Create test heads
	heads := []HeadService{
		{
			HeadID:      "head1",
			Endpoint:    "grpc://head1:50055",
			Status:      "active",
			CurrentLoad: 30,
			Region:      "us-east-1",
			ModelType:   "llama-3",
			Version:     "1.0.0",
			Metadata:    map[string]string{"capabilities": "gpu", "max_model_size": "large"},
		},
		{
			HeadID:      "head2",
			Endpoint:    "grpc://head2:50055",
			Status:      "active",
			CurrentLoad: 50,
			Region:      "us-west-2",
			ModelType:   "llama-3",
			Version:     "1.0.0",
			Metadata:    map[string]string{"capabilities": "gpu", "max_model_size": "large"},
		},
	}

	// Test adaptive routing with geo preference
	metadata := map[string]string{
		"model_version": "1.0.0",
		"model_size":   "large",
		"capabilities": "gpu",
	}

	selectedHead := applyAdaptiveRouting(heads, &GetRoutingDecisionRequest{
		ModelType:       "llama-3",
		RegionPreference: "us-east-1",
		Metadata:        metadata,
	})
	if selectedHead == nil {
		t.Errorf("Expected a head to be selected, got nil")
	}

	// Should select head1 as it's in the preferred region and can handle load
	if selectedHead.HeadID != "head1" {
		t.Errorf("Expected head1 to be selected, got %s", selectedHead.HeadID)
	}
}

func TestEnhancedModelSpecificStrategy(t *testing.T) {
	// Create test heads
	heads := []HeadService{
		{
			HeadID:      "head1",
			Endpoint:    "grpc://head1:50055",
			Status:      "active",
			CurrentLoad: 30,
			Region:      "us-east-1",
			ModelType:   "llama-3",
			Version:     "1.0.0",
			Metadata:    map[string]string{"capabilities": "gpu", "max_model_size": "large"},
		},
		{
			HeadID:      "head2",
			Endpoint:    "grpc://head2:50055",
			Status:      "active",
			CurrentLoad: 50,
			Region:      "us-east-1",
			ModelType:   "llama-3",
			Version:     "2.0.0",
			Metadata:    map[string]string{"capabilities": "gpu,tpu", "max_model_size": "xlarge"},
		},
	}

	// Test enhanced model-specific strategy
	metadata := map[string]string{
		"model_version": "2.0.0",
		"model_size":   "xlarge",
		"capabilities": "tpu",
	}

	selectedHead := applyEnhancedModelSpecificStrategy(heads, metadata)
	if selectedHead == nil {
		t.Errorf("Expected a head to be selected, got nil")
	}

	// Should select head2 as it has better capabilities and version match
	if selectedHead.HeadID != "head2" {
		t.Errorf("Expected head2 to be selected, got %s", selectedHead.HeadID)
	}
}

// Simple mock struct for testing
type GetRoutingDecisionRequest struct {
	ModelType       string
	RegionPreference string
	Metadata        map[string]string
}

