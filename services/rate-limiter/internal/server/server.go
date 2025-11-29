




package server

import (
	"context"
	"log"
	"net"
	"strings"
	"time"

	pb "llm-gateway-pro/services/rate-limiter/pb"
	"google.golang.org/grpc"
)

type RateLimiterServer struct {
	pb.UnimplementedRateLimiterServer
	limits map[string]map[string]int // path -> authPrefix -> limit
	usage  map[string]map[string]int // path -> authPrefix -> currentUsage
}

func NewRateLimiterServer() *RateLimiterServer {
	return &RateLimiterServer{
		limits: make(map[string]map[string]int),
		usage:  make(map[string]map[string]int),
	}
}

func (s *RateLimiterServer) Run() error {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		return err
	}

	grpcServer := grpc.NewServer()
	pb.RegisterRateLimiterServer(grpcServer, s)

	log.Println("Rate limiter service running on :50051")
	return grpcServer.Serve(lis)
}

func (s *RateLimiterServer) Check(ctx context.Context, req *pb.CheckRequest) (*pb.CheckResponse, error) {
	// Default limits - in a real implementation, these would come from config
	path := req.GetPath()
	auth := req.GetAuthorization()

	// Determine the auth prefix (JWT or API key)
	var authPrefix string
	if strings.HasPrefix(auth, "Bearer ") {
		authPrefix = "jwt"
	} else if strings.HasPrefix(auth, "tvo_") {
		authPrefix = "api_key"
	} else {
		authPrefix = "anonymous"
	}

	// Initialize limits for this path if not exists
	if s.limits[path] == nil {
		s.limits[path] = make(map[string]int)
		s.usage[path] = make(map[string]int)

		// Set default limits per path
		switch path {
		case "/v1/chat/completions", "/v1/completions":
			s.limits[path]["jwt"] = 60      // 60 requests per minute for JWT
			s.limits[path]["api_key"] = 30  // 30 requests per minute for API keys
			s.limits[path]["anonymous"] = 5 // 5 requests per minute for anonymous
		case "/v1/embeddings":
			s.limits[path]["jwt"] = 120
			s.limits[path]["api_key"] = 60
			s.limits[path]["anonymous"] = 10
		case "/v1/agentic":
			s.limits[path]["jwt"] = 30
			s.limits[path]["api_key"] = 15
			s.limits[path]["anonymous"] = 3
		}
	}

	// Initialize usage tracking if not exists
	if s.usage[path][authPrefix] == 0 {
		s.usage[path][authPrefix] = 0
	}

	// Check if limit is exceeded
	if s.usage[path][authPrefix] >= s.limits[path][authPrefix] {
		return &pb.CheckResponse{
			Allowed:          false,
			RetryAfterSecs:   60, // 1 minute retry
		}, nil
	}

	// Increment usage
	s.usage[path][authPrefix]++

	// Reset usage every minute (simple implementation)
	go func() {
		time.Sleep(1 * time.Minute)
		s.usage[path][authPrefix] = 0
	}()

	return &pb.CheckResponse{
		Allowed:          true,
		RetryAfterSecs:   0,
	}, nil
}




