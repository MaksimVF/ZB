


package main

import (
	"context"
	"net"
	"testing"
	"time"

	pb "github.com/MaksimVF/ZB/gen/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
)

func TestRoutingService(t *testing.T) {
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

	// Test RegisterHead
	t.Run("RegisterHead", func(t *testing.T) {
		resp, err := client.RegisterHead(context.Background(), &pb.RegisterHeadRequest{
			HeadId:    "test-head-1",
			Endpoint:  "grpc://head1:50055",
			Region:    "us-east-1",
			ModelType: "llama-3",
			Version:   "1.0.0",
			Metadata:  map[string]string{"env": "test"},
		})
		if err != nil {
			t.Errorf("RegisterHead failed: %v", err)
		}
		if !resp.Success {
			t.Errorf("RegisterHead failed: %s", resp.Message)
		}
	})

	// Test UpdateHeadStatus
	t.Run("UpdateHeadStatus", func(t *testing.T) {
		resp, err := client.UpdateHeadStatus(context.Background(), &pb.UpdateHeadStatusRequest{
			HeadId:      "test-head-1",
			Status:      "active",
			CurrentLoad: 30,
			Timestamp:    time.Now().Unix(),
		})
		if err != nil {
			t.Errorf("UpdateHeadStatus failed: %v", err)
		}
		if !resp.Success {
			t.Errorf("UpdateHeadStatus failed: %s", resp.Message)
		}
	})

	// Test GetRoutingDecision
	t.Run("GetRoutingDecision", func(t *testing.T) {
		resp, err := client.GetRoutingDecision(context.Background(), &pb.GetRoutingDecisionRequest{
			ClientId:        "test-client",
			ModelType:       "llama-3",
			RegionPreference: "us-east-1",
			RoutingStrategy: "round_robin",
			Metadata:        map[string]string{"priority": "high"},
		})
		if err != nil {
			t.Errorf("GetRoutingDecision failed: %v", err)
		}
		if resp.HeadId == "" {
			t.Errorf("GetRoutingDecision returned empty head ID")
		}
	})

	// Test GetAllHeads
	t.Run("GetAllHeads", func(t *testing.T) {
		resp, err := client.GetAllHeads(context.Background(), &pb.GetAllHeadsRequest{})
		if err != nil {
			t.Errorf("GetAllHeads failed: %v", err)
		}
		if len(resp.Heads) == 0 {
			t.Errorf("GetAllHeads returned empty list")
		}
	})

	server.Stop()
}


