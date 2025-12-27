package grpc

import (
	"context"
	"crypto/tls"
	"log"

	"github.com/MaksimVF/ZB/services/tail-go/cmd/tail/internal/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type HeadClient struct {
	Conn          *grpc.ClientConn
	configManager *config.NetworkConfigManager
}

func NewHeadClient(addr string, configManager *config.NetworkConfigManager) *HeadClient {
	creds := credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(creds))
	if err != nil {
		log.Fatal(err)
	}
	return &HeadClient{Conn: conn, configManager: configManager}
}

func (c *HeadClient) reconnect() error {
	if c.configManager == nil {
		return nil
	}

	networkConfig := c.configManager.GetConfig()
	if networkConfig.HeadEndpoint == "" {
		return nil
	}

	creds := credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})

	// Close existing connection
	if c.Conn != nil {
		c.Conn.Close()
	}

	conn, err := grpc.Dial(networkConfig.HeadEndpoint, grpc.WithTransportCredentials(creds))
	if err != nil {
		log.Printf("Failed to reconnect to head service: %v", err)
		return err
	}

	c.Conn = conn
	log.Printf("Successfully reconnected to head service at %s", networkConfig.HeadEndpoint)
	return nil
}

func (c *HeadClient) Completion(ctx context.Context, model string, msgs []any) (any, error) {
	// Check if we need to reconnect
	if c.configManager != nil {
		err := c.reconnect()
		if err != nil {
			log.Printf("Failed to reconnect: %v", err)
		}
	}

	// Placeholder - in real implementation, call head service
	return map[string]string{"content": "Hello from head service!"}, nil
}

func (c *HeadClient) Stream(ctx context.Context, model string, msgs []any) (<-chan string, error) {
	// Check if we need to reconnect
	if c.configManager != nil {
		err := c.reconnect()
		if err != nil {
			log.Printf("Failed to reconnect: %v", err)
		}
	}

	ch := make(chan string, 10)
	go func() {
		ch <- "Hello"
		ch <- "from"
		ch <- "head"
		ch <- "service"
		close(ch)
	}()
	return ch, nil
}
