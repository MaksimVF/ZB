package main

import (
"log"
"net/http"
"llm-gateway-pro/services/gateway/handlers"
"llm-gateway-pro/services/gateway/internal/grpc"
)

func main() {
headClient := grpc.NewHeadClient("head:50055")
rateClient := grpc.NewRateLimiterClient("rate-limiter:50051")

mux := http.NewServeMux()
mux.HandleFunc("POST /v1/chat/completions", rateClient.Middleware(handlers.ChatCompletion(headClient)))
mux.HandleFunc("POST /v1/batch", rateClient.Middleware(handlers.BatchSubmit(headClient)))

log.Println("Gateway HTTPS listening on :8443")
log.Fatal(http.ListenAndServeTLS(":8443", "/certs/gateway.pem", "/certs/gateway-key.pem", mux))
}
