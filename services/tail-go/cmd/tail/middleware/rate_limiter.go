




package middleware

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log"
	"net/http"
	"os"
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
	creds, err := loadTLSCredentials()
	if err != nil {
		log.Println("Failed to load TLS credentials, falling back to insecure connection:", err)
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

func loadTLSCredentials() (credentials.TransportCredentials, error) {
	// Load the certificate from file
	certPEMBlock, err := os.ReadFile("/certs/rate-limiter.pem")
	if err != nil {
		return nil, fmt.Errorf("failed to read certificate: %w", err)
	}

	// Parse the certificate
	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(certPEMBlock) {
		return nil, fmt.Errorf("failed to parse certificate")
	}

	// Create TLS config
	tlsConfig := &tls.Config{
		RootCAs: certPool,
	}

	return credentials.NewTLS(tlsConfig), nil
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

                // Handle gRPC errors first
                if err != nil {
                        log.Printf("Rate limiter error: %v", err)
                        w.Header().Set("Retry-After", "30")
                        http.Error(w, `{"error": "rate limiter unavailable"}`, http.StatusServiceUnavailable)
                        return
                }

                // Handle rate limiting response
                if !resp.Allowed {
                        retryAfter := int(resp.RetryAfterSecs)
                        if retryAfter == 0 {
                                retryAfter = 30 // Default retry-after in seconds (should be configurable)
                        }
                        w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
                        http.Error(w, `{"error": "rate limit exceeded"}`, http.StatusTooManyRequests)
                        return
                }

		next(w, r)
	}
}



