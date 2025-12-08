package grpc

import (
"context"
"crypto/tls"
"log"
"llm-gateway-pro/services/rate-limiter/pb"
"google.golang.org/grpc"
"google.golang.org/grpc/credentials"
)

type HeadClient struct {
Conn *grpc.ClientConn
configManager *config.NetworkConfigManager
}

type RateLimiterClient struct {
Client pb.RateLimiterClient
configManager *config.NetworkConfigManager
}

func NewHeadClient(addr string, configManager *config.NetworkConfigManager) *HeadClient {
creds := credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})
conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(creds))
if err != nil { log.Fatal(err) }
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

// вызов твоего head-сервиса
return map[string]string{"content": "Hello!"}, nil
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
go func() { ch <- "Hello"; ch <- "World"; close(ch) }()
return ch, nil
}

func NewRateLimiterClient(addr string, configManager *config.NetworkConfigManager) *RateLimiterClient {
creds := credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})
conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(creds))
if err != nil { log.Fatal(err) }
return &RateLimiterClient{pb.NewRateLimiterClient(conn), configManager}
}

func (c *RateLimiterClient) reconnect() error {
if c.configManager == nil {
return nil
}

networkConfig := c.configManager.GetConfig()
if networkConfig.HeadEndpoint == "" {
return nil
}

creds := credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})

// Close existing connection
if c.Client != nil {
c.Client.Close()
}

conn, err := grpc.Dial(networkConfig.HeadEndpoint, grpc.WithTransportCredentials(creds))
if err != nil {
log.Printf("Failed to reconnect to rate limiter: %v", err)
return err
}

c.Client = pb.NewRateLimiterClient(conn)
log.Printf("Successfully reconnected to rate limiter at %s", networkConfig.HeadEndpoint)
return nil
}

func (c *RateLimiterClient) Middleware(next http.HandlerFunc) http.HandlerFunc {
return func(w http.ResponseWriter, r *http.Request) {
// Check if we need to reconnect
if c.configManager != nil {
err := c.reconnect()
if err != nil {
log.Printf("Failed to reconnect rate limiter: %v", err)
}
}

// Заглушка — в реальности вызов Check()
next(w, r)
}
}
