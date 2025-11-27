package main

import (
"log"
"net"
"net/http"
"llm-gateway-pro/services/rate-limiter/limiter"
"google.golang.org/grpc"
)

func main() {
lis, _ := net.Listen("tcp", ":50051")
s := grpc.NewServer()
limiter.RegisterRateLimiterServer(s, &limiter.Service{})

go func() {
http.HandleFunc("/admin/api/", limiter.AdminHandler)
log.Fatal(http.ListenAndServe(":8081", nil))
}()

log.Println("Rate Limiter running")
s.Serve(lis)
}
