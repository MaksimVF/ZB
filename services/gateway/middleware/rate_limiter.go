



package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"
	"strconv"

	pb "llm-gateway-pro/services/rate-limiter/pb"
	"google.golang.org/grpc"
)

var rateLimiterClient pb.RateLimiterClient

func init() {
	conn, err := grpc.Dial("rate-limiter:50051", grpc.WithInsecure())
	if err != nil {
		panic("cannot connect to rate-limiter: " + err.Error())
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


