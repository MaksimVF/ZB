


package main

import (
	"fmt"
)

// HeadService represents a head service in the routing system
type HeadService struct {
	HeadID        string
	Endpoint      string
	Status        string
	CurrentLoad   int32
	Region        string
	ModelType     string
	Version       string
	Metadata      map[string]string
	LastHeartbeat int64
	// Optimization fields
	LoadHistory    []int32
	ResponseTimes []int64
	Capacity      int32
	Utilization   float64
}

// GetRoutingDecisionRequest represents a routing decision request
type GetRoutingDecisionRequest struct {
	ModelType       string
	RegionPreference string
	Metadata        map[string]string
}

// canHandleLoad checks if a head can handle additional load
func canHandleLoad(head *HeadService) bool {
	// Calculate utilization percentage
	if head.Capacity == 0 {
		return true // If capacity not set, assume it can handle load
	}

	utilization := float64(head.CurrentLoad) / float64(head.Capacity) * 100
	// Use default threshold
	threshold := 80.0
	return utilization < threshold
}

// applyPredictiveLoadBalancing selects a head based on predicted future load
func applyPredictiveLoadBalancing(heads []HeadService) *HeadService {
	if len(heads) == 0 {
		return nil
	}

	// Find the head with the best predicted future load
	var bestHead *HeadService
	var lowestPredictedLoad int32 = -1

	for i, head := range heads {
		// Predict future load for this head
		predictedLoad := predictFutureLoad(head)

		// Initialize with first head
		if bestHead == nil || predictedLoad < lowestPredictedLoad {
			bestHead = &heads[i]
			lowestPredictedLoad = predictedLoad
		}
	}

	return bestHead
}

// predictFutureLoad predicts future load based on historical data
func predictFutureLoad(head HeadService) int32 {
	// Simple moving average prediction
	if len(head.LoadHistory) == 0 {
		return head.CurrentLoad
	}

	// Calculate moving average (last 5 data points or all available)
	start := 0
	if len(head.LoadHistory) > 5 {
		start = len(head.LoadHistory) - 5
	}

	sum := int64(0)
	count := 0
	for i := start; i < len(head.LoadHistory); i++ {
		sum += int64(head.LoadHistory[i])
		count++
	}

	if count == 0 {
		return head.CurrentLoad
	}

	average := sum / int64(count)

	// Apply growth factor
	growthFactor := 1.1 // 10% growth prediction
	predicted := int32(float64(average) * growthFactor)

	// Don't predict lower than current load
	if predicted < head.CurrentLoad {
		return head.CurrentLoad
	}

	return predicted
}

// applyAdaptiveRouting selects a head based on real-time conditions and performance
func applyAdaptiveRouting(heads []HeadService, req *GetRoutingDecisionRequest) *HeadService {
	if len(heads) == 0 {
		return nil
	}

	// First check for model-specific requirements
	modelHead := applyEnhancedModelSpecificStrategy(heads, req.Metadata)
	if modelHead != nil && canHandleLoad(modelHead) {
		return modelHead
	}

	// Check geo-preference
	geoHead := applyGeoPreferredStrategy(heads, req.RegionPreference)
	if geoHead != nil && canHandleLoad(geoHead) {
		return geoHead
	}

	// Use predictive load balancing
	return applyPredictiveLoadBalancing(heads)
}

// applyGeoPreferredStrategy selects a head in the preferred region
func applyGeoPreferredStrategy(heads []HeadService, preferredRegion string) *HeadService {
	if len(heads) == 0 {
		return nil
	}

	// First try to find a head in the preferred region
	for _, head := range heads {
		if head.Region == preferredRegion {
			return &head
		}
	}

	// If no head in preferred region, fall back to first head
	if len(heads) > 0 {
		return &heads[0]
	}
	return nil
}

// applyEnhancedModelSpecificStrategy selects based on detailed model-specific criteria
func applyEnhancedModelSpecificStrategy(heads []HeadService, metadata map[string]string) *HeadService {
	if len(heads) == 0 {
		return nil
	}

	// Extract model-specific requirements from metadata
	modelVersion := metadata["model_version"]
	modelSize := metadata["model_size"]
	requiredCapabilities := metadata["capabilities"]

	// Score heads based on model compatibility
	var bestHead *HeadService
	var highestScore int

	for i, head := range heads {
		score := 0

		// Check version compatibility
		if head.Version == modelVersion {
			score += 3
		} else if len(head.Version) >= len(modelVersion) && head.Version[:len(modelVersion)] == modelVersion {
			score += 2
		}

		// Check capacity for model size
		if head.Metadata["max_model_size"] >= modelSize {
			score += 2
		}

		// Check required capabilities
		if head.Metadata["capabilities"] == requiredCapabilities {
			score += 2
		}

		// Check current load
		if canHandleLoad(&head) {
			score += 1
		}

		if score > highestScore {
			bestHead = &heads[i]
			highestScore = score
		}
	}

	return bestHead
}

func main() {
	// Run tests manually
	fmt.Println("Running predictive load balancing test...")
	TestPredictiveLoadBalancing()

	fmt.Println("Running adaptive routing test...")
	TestAdaptiveRouting()

	fmt.Println("Running enhanced model-specific strategy test...")
	TestEnhancedModelSpecificStrategy()

	fmt.Println("All tests completed!")
}

func TestPredictiveLoadBalancing() {
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
		fmt.Println("FAIL: Expected a head to be selected, got nil")
		return
	}

	// Should select head1 as it has lower predicted load
	if selectedHead.HeadID != "head1" {
		fmt.Printf("FAIL: Expected head1 to be selected, got %s\n", selectedHead.HeadID)
		return
	}

	fmt.Println("PASS: Predictive load balancing test")
}

func TestAdaptiveRouting() {
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
		fmt.Println("FAIL: Expected a head to be selected, got nil")
		return
	}

	// Should select head1 as it's in the preferred region and can handle load
	if selectedHead.HeadID != "head1" {
		fmt.Printf("FAIL: Expected head1 to be selected, got %s\n", selectedHead.HeadID)
		return
	}

	fmt.Println("PASS: Adaptive routing test")
}

func TestEnhancedModelSpecificStrategy() {
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
		fmt.Println("FAIL: Expected a head to be selected, got nil")
		return
	}

	// Should select head2 as it has better capabilities and version match
	if selectedHead.HeadID != "head2" {
		fmt.Printf("FAIL: Expected head2 to be selected, got %s\n", selectedHead.HeadID)
		return
	}

	fmt.Println("PASS: Enhanced model-specific strategy test")
}




