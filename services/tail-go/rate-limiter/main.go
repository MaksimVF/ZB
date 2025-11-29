
// services/rate-limiter/main.go
package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
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
	// Load TLS credentials with proper certificate validation
	creds, err := loadTLSCredentials()
	if err != nil {
		log.Fatalf("Failed to load TLS credentials: %v", err)
	}

	// gRPC сервер
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	s := grpc.NewServer(grpc.Creds(creds))
	pb.RegisterRateLimiterServer(s, &limiter.Server{})

	// HTTP админка (для UI) - also with TLS
	go func() {
		http.HandleFunc("/admin/api/rate-limits", limiter.AdminHandler)

		// Load admin server TLS credentials
		adminCreds, err := loadAdminTLSCredentials()
		if err != nil {
			log.Fatalf("Failed to load admin TLS credentials: %v", err)
		}

		server := &http.Server{
			Addr:      ":8081",
			TLSConfig: adminCreds,
		}

		log.Println("Admin HTTP server starting on :8081 (TLS)")
		if err := server.ListenAndServeTLS("/certs/admin.pem", "/certs/admin-key.pem"); err != nil {
			log.Fatalf("Admin server failed: %v", err)
		}
	}()

	log.Println("Rate-limiter: gRPC :50051 (mTLS), Admin HTTPS :8081")
	log.Fatal(s.Serve(lis))
}

// loadTLSCredentials loads gRPC TLS credentials with proper certificate validation
func loadTLSCredentials() (credentials.TransportCredentials, error) {
	// Load server certificate and key
	serverCert, err := tls.LoadX509KeyPair("/certs/rate-limiter.pem", "/certs/rate-limiter-key.pem")
	if err != nil {
		return nil, fmt.Errorf("failed to load server certificate: %w", err)
	}

	// Load CA certificate for client verification
	caCert, err := os.ReadFile("/certs/ca.pem")
	if err != nil {
		return nil, fmt.Errorf("failed to load CA certificate: %w", err)
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to add CA certificate to pool")
	}

	// Create TLS config with proper validation
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    certPool,
		MinVersion:   tls.VersionTLS12,
	}

	return credentials.NewTLS(tlsConfig), nil
}

// loadAdminTLSCredentials loads HTTP server TLS credentials
func loadAdminTLSCredentials() (*tls.Config, error) {
	// Load server certificate and key
	serverCert, err := tls.LoadX509KeyPair("/certs/admin.pem", "/certs/admin-key.pem")
	if err != nil {
		return nil, fmt.Errorf("failed to load admin certificate: %w", err)
	}

	// Load CA certificate for client verification (optional for admin)
	caCert, err := os.ReadFile("/certs/ca.pem")
	if err != nil {
		return nil, fmt.Errorf("failed to load CA certificate: %w", err)
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to add CA certificate to pool")
	}

	return &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.VerifyClientCertIfGiven,
		ClientCAs:    certPool,
		MinVersion:   tls.VersionTLS12,
	}, nil
}
