


package secrets

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"google.golang.org/grpc"

	"llm-gateway-pro/services/agentic-service/internal/grpc"
	pb "llm-gateway-pro/services/secret-service/pb"
)

var (
	secretClient  pb.SecretServiceClient
	secretConn    *grpc.ClientConn
	secretsCache sync.Map
)

func init() {
	var err error
	secretConn, err = grpc.Dial(
		"secret-service:50053",
		grpc.WithTransportCredentials(loadClientTLSCredentials()),
	)
	if err != nil {
		panic(fmt.Sprintf("Failed to connect to secret-service: %v", err))
	}

	secretClient = pb.NewSecretServiceClient(secretConn)
}

func Get(name string) (string, error) {
	if val, ok := secretsCache.Load(name); ok {
		if cached, ok := val.(struct {
			value string
			exp   time.Time
		}); ok && time.Now().Before(cached.exp) {
			return cached.value, nil
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := secretClient.GetSecret(ctx, &pb.GetSecretRequest{Name: name})
	if err != nil {
		return "", fmt.Errorf("failed to get secret %s: %w", name, err)
	}

	secretsCache.Store(name, struct {
		value string
		exp   time.Time
	}{value: resp.Value, exp: time.Now().Add(30 * time.Second)})

	return resp.Value, nil
}

func loadClientTLSCredentials() grpc.TransportCredentials {
	// Implementation would load TLS credentials
	return nil // Placeholder
}


