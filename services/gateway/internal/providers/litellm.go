










package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	pb "path/to/generated/model_pb2"  // Update with actual path
)

var (
	providerCache     = make(map[string]ProviderConfig)
	cacheMutex        = &sync.RWMutex{}
	logger            = zerolog.New(os.Stdout).With().Timestamp().Str("service", "litellm-proxy").Logger()
	grpcClients        = make(map[string]*grpc.ClientConn)
)

type ProviderConfig struct {
	BaseURL    string
	APIKey     string
	ModelNames []string
	GRPCAddress string // New field for gRPC address
	UseGRPC    bool     // Flag to use gRPC instead of HTTP
}

type LiteLLMConfig struct {
	Providers map[string]ProviderConfig
}

func Init(config LiteLLMConfig) {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	providerCache = config.Providers
	logger.Info().Msgf("Initialized LiteLLM with %d providers", len(providerCache))

	// Initialize gRPC clients for providers that use gRPC
	for provider, config := range providerCache {
		if config.UseGRPC {
			conn, err := grpc.Dial(config.GRPCAddress,
				grpc.WithTransportCredentials(insecure.NewCredentials()),
				grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(1024*1024*100))) // 100MB max
			if err != nil {
				logger.Error().Str("provider", provider).Err(err).Msg("Failed to connect to gRPC server")
				continue
			}
			grpcClients[provider] = conn
			logger.Info().Str("provider", provider).Str("address", config.GRPCAddress).Msg("Connected to gRPC server")
		}
	}
}

func GetProviderForModel(model string) (ProviderConfig, error) {
	cacheMutex.RLock()
	defer cacheMutex.RUnlock()

	for provider, config := range providerCache {
		for _, modelName := range config.ModelNames {
			if strings.EqualFold(model, modelName) {
				return config, nil
			}
		}
	}

	return ProviderConfig{}, errors.New("no provider found for model")
}

func ProxyRequest(providerConfig ProviderConfig, method, path string, body interface{}) ([]byte, error) {
	// Use gRPC if configured, otherwise fall back to HTTP
	if providerConfig.UseGRPC {
		return proxyGRPCRequest(providerConfig, body)
	}
	return proxyHTTPRequest(providerConfig, method, path, body)
}

func proxyHTTPRequest(providerConfig ProviderConfig, method, path string, body interface{}) ([]byte, error) {
	url := providerConfig.BaseURL + path

	// Create request
	reqBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequest(method, url, strings.NewReader(string(reqBody)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+providerConfig.APIKey)

	// Execute request
	client := &http.Client{Timeout: 180 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("provider returned status %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

func proxyGRPCRequest(providerConfig ProviderConfig, body interface{}) ([]byte, error) {
	// Find the provider in cache to get the gRPC client
	cacheMutex.RLock()
	client, ok := grpcClients[providerConfig.BaseURL] // Using BaseURL as key for now
	cacheMutex.RUnlock()

	if !ok {
		return nil, errors.New("no gRPC client found for provider")
	}

	// Convert body to gRPC request
	reqBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	// Parse the request body to extract parameters
	var reqData map[string]interface{}
	if err := json.Unmarshal(reqBody, &reqData); err != nil {
		return nil, fmt.Errorf("failed to parse request body: %w", err)
	}

	// Create gRPC request
	grpcClient := pb.NewModelServiceClient(client)
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	// Convert messages to gRPC format
	var messages []*pb.Message
	if msgs, ok := reqData["messages"].([]interface{}); ok {
		for _, msg := range msgs {
			if msgMap, ok := msg.(map[string]interface{}); ok {
				role, _ := msgMap["role"].(string)
				content, _ := msgMap["content"].(string)
				messages = append(messages, &pb.Message{
					Role:    role,
					Content: content,
				})
			}
		}
	}

	// Create and send gRPC request
	resp, err := grpcClient.Generate(ctx, &pb.GenRequest{
		Model:       reqData["model"].(string),
		Messages:    messages,
		Temperature:  float32(reqData["temperature"].(float64)),
		MaxTokens:   int32(reqData["max_tokens"].(float64)),
		RequestId:  "gateway-" + time.Now().Format("20060102-150405"),
	})
	if err != nil {
		return nil, fmt.Errorf("gRPC request failed: %w", err)
	}

	// Convert gRPC response to JSON
	response := map[string]interface{}{
		"text":        resp.Text,
		"tokens_used":   resp.TokensUsed,
		"request_id":   resp.RequestId,
		"model":        reqData["model"],
	}

	return json.Marshal(response)
}

func ListAvailableModels() []string {
	cacheMutex.RLock()
	defer cacheMutex.RUnlock()

	var models []string
	for _, config := range providerCache {
		models = append(models, config.ModelNames...)
	}
	return models
}

func AddProvider(provider string, config ProviderConfig) {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	providerCache[provider] = config
	logger.Info().Str("provider", provider).Msg("Added new provider")

	// Initialize gRPC client if needed
	if config.UseGRPC {
		conn, err := grpc.Dial(config.GRPCAddress,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(1024*1024*100)))
		if err != nil {
			logger.Error().Str("provider", provider).Err(err).Msg("Failed to connect to gRPC server")
			return
		}
		grpcClients[provider] = conn
		logger.Info().Str("provider", provider).Str("address", config.GRPCAddress).Msg("Connected to gRPC server")
	}
}

func RemoveProvider(provider string) {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	// Close gRPC connection if exists
	if conn, ok := grpcClients[provider]; ok {
		conn.Close()
		delete(grpcClients, provider)
	}

	delete(providerCache, provider)
	logger.Info().Str("provider", provider).Msg("Removed provider")
}

func GetAllProviders() map[string]ProviderConfig {
	cacheMutex.RLock()
	defer cacheMutex.RUnlock()

	// Return a copy to prevent modification
	copy := make(map[string]ProviderConfig)
	for k, v := range providerCache {
		copy[k] = v
	}
	return copy
}

func CloseAllConnections() {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	for provider, conn := range grpcClients {
		conn.Close()
		delete(grpcClients, provider)
		logger.Info().Str("provider", provider).Msg("Closed gRPC connection")
	}
}






