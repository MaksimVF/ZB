




package middleware

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"
	"strconv"

	pb "llm-gateway-pro/services/rate-limiter/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var rateLimiterClient pb.RateLimiterClient

func init() {
	// Try to establish secure connection first
	creds, err := credentials.NewClientTLSFromFile("/certs/rate-limiter.pem", "")
	if err != nil {
		log.Println("Failed to load rate-limiter TLS cert, falling back to insecure connection:", err)
		conn, err := grpc.Dial("rate-limiter:50051", grpc.WithInsecure())
		if err != nil {
			panic("cannot connect to rate-limiter: " + err.Error())
		}
		rateLimiterClient = pb.NewRateLimiterClient(conn)
		return
	}

	conn, err := grpc.Dial("rate-limiter:50051", grpc.WithTransportCredentials(creds))
	if err != nil {
		log.Println("Failed to connect with TLS, falling back to insecure connection:", err)
		conn, err := grpc.Dial("rate-limiter:50051", grpc.WithInsecure())
		if err != nil {
			panic("cannot connect to rate-limiter: " + err.Error())
		}
		rateLimiterClient = pb.NewRateLimiterClient(conn)
		return
	}
	rateLimiterClient = pb.NewRateLimiterClient(conn)
}

func RateLimiter(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Пропускаем health-check и статические файлы
		if strings.HasPrefix(path, "/health") || strings.HasPrefix(path, "/static") {
			next(w, r)
			return
		}

		auth := r.Header.Get("Authorization")

		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		resp, err := rateLimiterClient.Check(ctx, &pb.CheckRequest{
			Authorization: auth,
			Path:          path,
		})

		if err != nil || !resp.Allowed {
			retryAfter := int(resp.RetryAfterSecs)
			if retryAfter == 0 {
				retryAfter = 30
			}
			w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
			http.Error(w, `{"error": "rate limit exceeded"}`, http.StatusTooManyRequests)
			return
		}

		next(w, r)
	}
}



