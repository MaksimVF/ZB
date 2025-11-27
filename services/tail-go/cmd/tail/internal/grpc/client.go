package grpc

import (
"context"
"crypto/tls"
"log"
"llm-gateway-pro/services/rate-limiter/pb"
"google.golang.org/grpc"
"google.golang.org/grpc/credentials"
)

type HeadClient struct{ Conn *grpc.ClientConn }
type RateLimiterClient struct{ Client pb.RateLimiterClient }

func NewHeadClient(addr string) *HeadClient {
creds := credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})
conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(creds))
if err != nil { log.Fatal(err) }
return &HeadClient{Conn: conn}
}

func (c *HeadClient) Completion(ctx context.Context, model string, msgs []any) (any, error) {
// вызов твоего head-сервиса
return map[string]string{"content": "Hello!"}, nil
}

func (c *HeadClient) Stream(ctx context.Context, model string, msgs []any) (<-chan string, error) {
ch := make(chan string, 10)
go func() { ch <- "Hello"; ch <- "World"; close(ch) }()
return ch, nil
}

func NewRateLimiterClient(addr string) *RateLimiterClient {
creds := credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})
conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(creds))
if err != nil { log.Fatal(err) }
return &RateLimiterClient{pb.NewRateLimiterClient(conn)}
}

func (c *RateLimiterClient) Middleware(next http.HandlerFunc) http.HandlerFunc {
return func(w http.ResponseWriter, r *http.Request) {
// Заглушка — в реальности вызов Check()
next(w, r)
}
}
