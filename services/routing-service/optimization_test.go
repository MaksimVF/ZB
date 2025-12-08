
package main

import (
	"context"
	"testing"
	"time"

	"github.com/MaksimVF/ZB/gen/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
)

func TestOptimizationStrategies(t *testing.T) {
	// Create a buffer connection for testing
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()

	// Register the routing service
	pb.RegisterRoutingServiceServer(server, &RoutingServer{})

	go func() {
		if err := server.Serve(listener); err != nil {
			t.Fatalf("Server exited with error: %v", err)
		}
	}()

	// Create a client connection
	conn, err := grpc.Dial("bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithInsecure())
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}
	defer conn.Close()

	client := pb.NewRoutingServiceClient(conn)

	// Register a test head
	client.RegisterHead(context.Background(), &pb.RegisterHeadRequest{
		HeadId:    "test-head-1",
		Endpoint:  "grpc://head1:50055",
		Region:    "us-east-1",
		ModelType: "llama-3",
		Version:   "1.0.0",
		Metadata:  map[string]string{"env": "test", "capacity": "100", "max_model_size": "large", "capabilities": "gpu"},
	})

	// Update head status
	client.UpdateHeadStatus(context.Background(), &pb.UpdateHeadStatusRequest{
		HeadId:      "test-head-1",
		Status:      "active",
		CurrentLoad: 30,
		Timestamp:    time.Now().Unix(),
	})

	// Test predictive strategy
	t.Run("PredictiveStrategy", func(t *testing.T) {
		resp, err := client.GetRoutingDecision(context.Background(), &pb.GetRoutingDecisionRequest{
			ClientId:        "test-client",
			ModelType:       "llama-3",
			RegionPreference: "us-east-1",
			RoutingStrategy: "predictive",
			Metadata:        map[string]string{"priority": "high"},
		})
		if err != nil {
			t.Errorf("GetRoutingDecision failed: %v", err)
		}
		if resp.HeadId == "" {
			t.Errorf("GetRoutingDecision returned empty head ID")
		}
		if resp.StrategyUsed != "predictive" {
			t.Errorf("Expected predictive strategy, got %s", resp.StrategyUsed)
		}
	})

	// Test adaptive strategy
	t.Run("AdaptiveStrategy", func(t *testing.T) {
		resp, err := client.GetRoutingDecision(context.Background(), &pb.GetRoutingDecisionRequest{
			ClientId:        "test-client",
			ModelType:       "llama-3",
			RegionPreference: "us-east-1",
			RoutingStrategy: "adaptive",
			Metadata:        map[string]string{"priority": "high", "model_version": "1.0.0", "model_size": "large"},
		})
		if err != nil {
			t.Errorf("GetRoutingDecision failed: %v", err)
		}
		if resp.HeadId == "" {
			t.Errorf("GetRoutingDecision returned empty head ID")
		}
		if resp.StrategyUsed != "adaptive" {
			t.Errorf("Expected adaptive strategy, got %s", resp.StrategyUsed)
		}
	})

	// Test enhanced model-specific strategy
	t.Run("EnhancedModelSpecificStrategy", func(t *testing.T) {
		resp, err := client.GetRoutingDecision(context.Background(), &pb.GetRoutingDecisionRequest{
			ClientId:        "test-client",
			ModelType:       "llama-3",
			RegionPreference: "us-east-1",
			RoutingStrategy: "model_specific",
			Metadata:        map[string]string{"priority": "high", "model_version": "1.0.0", "model_size": "large", "capabilities": "gpu"},
		})
		if err != nil {
			t.Errorf("GetRoutingDecision failed: %v", err)
		}
		if resp.HeadId == "" {
			t.Errorf("GetRoutingDecision returned empty head ID")
		}
		if resp.StrategyUsed != "model_specific" {
			t.Errorf("Expected model_specific strategy, got %s", resp.StrategyUsed)
		}
	})

	server.Stop()
}
