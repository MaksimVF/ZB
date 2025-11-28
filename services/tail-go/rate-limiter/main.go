// services/rate-limiter/main.go
package main

import (
"log"
"net"
"net/http"
"os"

"google.golang.org/grpc"
"google.golang.org/grpc/credentials"
pb "llm-gateway-pro/services/rate-limiter/pb"
"llm-gateway-pro/services/rate-limiter/internal/limiter"
)

func main() {
// gRPC сервер
lis, _ := net.Listen("tcp", ":50051")
creds, _ := credentials.NewServerTLSFromFile("/certs/rate-limiter.pem", "/certs/rate-limiter-key.pem")
s := grpc.NewServer(grpc.Creds(creds))
pb.RegisterRateLimiterServer(s, &limiter.Server{})

// HTTP админка (для UI)
go func() {
http.HandleFunc("/admin/api/rate-limits", limiter.AdminHandler)
log.Fatal(http.ListenAndServe(":8081", nil))
}()

log.Println("Rate-limiter: gRPC :50051 (mTLS), Admin HTTP :8081")
log.Fatal(s.Serve(lis))
}
