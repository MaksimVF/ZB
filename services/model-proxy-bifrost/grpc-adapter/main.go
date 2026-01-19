

package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	model "github.com/MaksimVF/ZB/services/head-go/gen_model"
)

// BifrostGRPCAdapter implements the ModelService gRPC interface and translates to Bifrost HTTP API
type BifrostGRPCAdapter struct {
	bifrostURL string
	httpClient *http.Client
	model.UnimplementedModelServiceServer
}

// NewBifrostGRPCAdapter creates a new adapter instance
func NewBifrostGRPCAdapter(bifrostURL string) *BifrostGRPCAdapter {
	return &BifrostGRPCAdapter{
		bifrostURL: bifrostURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Generate implements the gRPC Generate method by calling Bifrost HTTP API
func (s *BifrostGRPCAdapter) Generate(ctx context.Context, req *model.GenRequest) (*model.GenResponse, error) {
	// Convert gRPC request to Bifrost HTTP request format
	bifrostReq := map[string]interface{}{
		"model":       req.Model,
		"messages":    convertMessages(req.Messages),
		"temperature": req.Temperature,
		"max_tokens":  int(req.MaxTokens),
		"stream":      req.Stream,
	}

	// Call Bifrost HTTP API
	url := fmt.Sprintf("%s/v1/chat/completions", s.bifrostURL)
	resp, err := s.callBifrostAPI(ctx, url, bifrostReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Bifrost API call failed: %v", err)
	}

	// Convert Bifrost response to gRPC response
	grpcResp := &model.GenResponse{
		RequestId: req.RequestId,
		Text:      extractTextFromBifrostResponse(resp),
		TokensUsed: int32(extractTokenCountFromBifrostResponse(resp)),
		Metadata:  make(map[string]string),
	}

	// Add metadata if available
	if metadata, ok := resp["metadata"].(map[string]interface{}); ok {
		for k, v := range metadata {
			grpcResp.Metadata[k] = fmt.Sprintf("%v", v)
		}
	}

	return grpcResp, nil
}

// GenerateStream implements the gRPC GenerateStream method
func (s *BifrostGRPCAdapter) GenerateStream(req *model.GenRequest, stream model.ModelService_GenerateStreamServer) error {
	// For streaming, we need to implement a more complex translation
	// This is a simplified version - in production you'd need proper streaming handling

	// Convert gRPC request to Bifrost HTTP request format
	bifrostReq := map[string]interface{}{
		"model":       req.Model,
		"messages":    convertMessages(req.Messages),
		"temperature": req.Temperature,
		"max_tokens":  int(req.MaxTokens),
		"stream":      true,
	}

	// Call Bifrost HTTP API (non-streaming for simplicity)
	url := fmt.Sprintf("%s/v1/chat/completions", s.bifrostURL)
	resp, err := s.callBifrostAPI(stream.Context(), url, bifrostReq)
	if err != nil {
		return status.Errorf(codes.Internal, "Bifrost API call failed: %v", err)
	}

	// Send single response chunk (in real implementation, this would be streaming)
	grpcResp := &model.GenResponse{
		RequestId: req.RequestId,
		Text:      extractTextFromBifrostResponse(resp),
		TokensUsed: int32(extractTokenCountFromBifrostResponse(resp)),
		Metadata:  make(map[string]string),
	}

	if err := stream.Send(grpcResp); err != nil {
		return status.Errorf(codes.Internal, "Failed to send stream response: %v", err)
	}

	return nil
}

// BatchGenerate implements the gRPC BatchGenerate method
func (s *BifrostGRPCAdapter) BatchGenerate(ctx context.Context, req *model.BatchGenRequest) (*model.BatchGenResponse, error) {
	responses := make([]*model.GenResponse, 0, len(req.Requests))

	for _, singleReq := range req.Requests {
		// Call Generate for each request in the batch
		resp, err := s.Generate(ctx, singleReq)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Batch request failed for request %s: %v", singleReq.RequestId, err)
		}
		responses = append(responses, resp)
	}

	return &model.BatchGenResponse{
		Responses: responses,
		BatchMetadata: req.BatchMetadata,
	}, nil
}

// Helper functions

func convertMessages(messages []string) []map[string]string {
	// Convert simple string messages to Bifrost format
	// This is a simplified conversion - real implementation would need proper role handling
	bifrostMessages := make([]map[string]string, len(messages))
	for i, msg := range messages {
		role := "user"
		if i%2 == 1 { // Simple alternation for demo
			role = "assistant"
		}
		bifrostMessages[i] = map[string]string{
			"role":    role,
			"content": msg,
		}
	}
	return bifrostMessages
}

func extractTextFromBifrostResponse(resp map[string]interface{}) string {
	if choices, ok := resp["choices"].([]interface{}); ok && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]interface{}); ok {
			if message, ok := choice["message"].(map[string]interface{}); ok {
				if content, ok := message["content"].(string); ok {
					return content
				}
			}
			// Fallback for text completion format
			if text, ok := choice["text"].(string); ok {
				return text
			}
		}
	}
	return "No response text available"
}

func extractTokenCountFromBifrostResponse(resp map[string]interface{}) int {
	if usage, ok := resp["usage"].(map[string]interface{}); ok {
		if totalTokens, ok := usage["total_tokens"].(float64); ok {
			return int(totalTokens)
		}
	}
	// Fallback: estimate token count from text length
	if text := extractTextFromBifrostResponse(resp); text != "" {
		return len(text)/4 + 1 // Rough estimate
	}
	return 1
}

func (s *BifrostGRPCAdapter) callBifrostAPI(ctx context.Context, url string, req map[string]interface{}) (map[string]interface{}, error) {
	// Convert request to JSON
	reqJSON, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(reqJSON)))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %v", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	// Send request
	httpResp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer httpResp.Body.Close()

	// Check status code
	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Bifrost API returned status %d", httpResp.StatusCode)
	}

	// Parse response
	var resp map[string]interface{}
	if err := json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	return resp, nil
}

// loadTLSCredentials loads TLS certificates for mTLS
func loadTLSCredentials() (credentials.TransportCredentials, error) {
	// Server certificate and key (model-proxy)
	cert, err := tls.LoadX509KeyPair("/certs/model-proxy.pem", "/certs/model-proxy-key.pem")
	if err != nil {
		return nil, fmt.Errorf("failed to load server certificate: %v", err)
	}

	// CA for client verification (head service)
	caCert, err := ioutil.ReadFile("/certs/ca.pem")
	if err != nil {
		return nil, fmt.Errorf("failed to load CA certificate: %v", err)
	}
	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to parse CA certificate")
	}

	// Configure TLS with mutual authentication
	config := &tls.Config{
		ServerName:   "model-proxy",
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    caCertPool,
	}

	return credentials.NewTLS(config), nil
}

func main() {
	// Get Bifrost URL from environment or use default
	bifrostURL := os.Getenv("BIFROST_URL")
	if bifrostURL == "" {
		bifrostURL = "http://localhost:8100"
	}

	// Create gRPC server with TLS credentials
	lis, err := net.Listen("tcp", ":50061")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	tlsCreds, err := loadTLSCredentials()
	if err != nil {
		log.Fatalf("Failed to load TLS credentials: %v", err)
	}

	grpcServer := grpc.NewServer(
		grpc.Creds(tlsCreds),
	)
	adapter := NewBifrostGRPCAdapter(bifrostURL)
	model.RegisterModelServiceServer(grpcServer, adapter)

	log.Printf("Bifrost gRPC Adapter started with mTLS, listening on :50061, forwarding to Bifrost at %s", bifrostURL)

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}


