package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/MaksimVF/ZB/services/routing-service/config"
)

// UserRole represents user roles
type UserRole string

const (
	RoleAdmin    UserRole = "admin"
	RoleOperator UserRole = "operator"
	RoleViewer   UserRole = "viewer"
)

// UserContext contains user information
type UserContext struct {
	UserID string
	Role   UserRole
}

// JWTAuthMiddleware validates JWT tokens
func JWTAuthMiddleware(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip authentication for health check and metrics
			if r.URL.Path == "/health" || r.URL.Path == "/metrics" {
				next.ServeHTTP(w, r)
				return
			}

			// Start timer for request duration
			start := time.Now()

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "Missing authorization header", http.StatusUnauthorized)
				return
			}

			// In production, validate the JWT token and extract user info
			// For now, we'll simulate token validation and role extraction
			var userCtx UserContext

			// Simulate token validation
			switch authHeader {
			case "Bearer admin-token":
				userCtx = UserContext{UserID: "admin-user", Role: RoleAdmin}
			case "Bearer operator-token":
				userCtx = UserContext{UserID: "operator-user", Role: RoleOperator}
			case "Bearer viewer-token":
				userCtx = UserContext{UserID: "viewer-user", Role: RoleViewer}
			default:
				http.Error(w, "Invalid token", http.StatusUnauthorized)
				return
			}

			// Add user context to request
			ctx := context.WithValue(r.Context(), "user", userCtx)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RBACMiddleware checks user roles
func RBACMiddleware(requiredRole UserRole) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userCtx, ok := r.Context().Value("user").(UserContext)
			if !ok || !hasRole(userCtx.Role, requiredRole) {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// hasRole checks if user has required role
func hasRole(userRole, requiredRole UserRole) bool {
	switch requiredRole {
	case RoleAdmin:
		return userRole == RoleAdmin
	case RoleOperator:
		return userRole == RoleAdmin || userRole == RoleOperator
	case RoleViewer:
		return userRole == RoleAdmin || userRole == RoleOperator || userRole == RoleViewer
	default:
		return false
	}
}

// WebhookSecurityMiddleware validates webhook requests
func WebhookSecurityMiddleware(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Validate JWT token for webhook
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "Missing authorization header", http.StatusUnauthorized)
				return
			}

			// In production, validate the JWT token
			// For now, we'll check for a specific webhook token format
			if !strings.HasPrefix(authHeader, "Bearer webhook-") {
				http.Error(w, "Invalid webhook token", http.StatusUnauthorized)
				return
			}

			// Validate application signature if present
			appSignature := r.Header.Get("X-App-Signature")
			if appSignature == "" {
				http.Error(w, "Missing application signature", http.StatusUnauthorized)
				return
			}

			// In production, validate the application signature
			// For now, we'll check for a specific format
			if !strings.HasPrefix(appSignature, "app-sig-") {
				http.Error(w, "Invalid application signature", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// GetUserContext retrieves user context from request
func GetUserContext(r *http.Request) (UserContext, bool) {
	userCtx, ok := r.Context().Value("user").(UserContext)
	return userCtx, ok
}