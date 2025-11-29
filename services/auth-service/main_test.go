



package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestEnvironment() {
	// Set environment variables for testing
	os.Setenv("JWT_SECRET", "test-jwt-secret")
	os.Setenv("REDIS_ADDR", "localhost:6379")
	os.Setenv("DB_HOST", "localhost")
	os.Setenv("DB_USER", "test")
	os.Setenv("DB_PASSWORD", "test")
	os.Setenv("DB_NAME", "test")
	os.Setenv("DB_PORT", "5432")

	// Initialize the service
	init()

	// Use SQLite for testing
	var err error
	db, err = gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	// Migrate the schema
	err = db.AutoMigrate(&User{}, &APIKey{})
	if err != nil {
		panic("failed to migrate database")
	}
}

func TestRegister(t *testing.T) {
	setupTestEnvironment()

	tests := []struct {
		name           string
		email          string
		password       string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "valid registration",
			email:          "test@example.com",
			password:       "StrongPass123",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "invalid email",
			email:          "invalid-email",
			password:       "StrongPass123",
			expectedStatus: http.StatusBadRequest,
			expectedError:  InvalidEmailError,
		},
		{
			name:           "weak password",
			email:          "test@example.com",
			password:       "weak",
			expectedStatus: http.StatusBadRequest,
			expectedError:  WeakPasswordError,
		},
		{
			name:           "duplicate email",
			email:          "test@example.com",
			password:       "StrongPass123",
			expectedStatus: http.StatusConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqBody := map[string]string{
				"email":    tt.email,
				"password": tt.password,
			}
			reqBytes, _ := json.Marshal(reqBody)

			req := httptest.NewRequest("POST", "/register", strings.NewReader(string(reqBytes)))
			rr := httptest.NewRecorder()

			Register(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)

			if tt.expectedStatus != http.StatusOK {
				assert.Contains(t, rr.Body.String(), tt.expectedError)
			} else {
				var response map[string]string
				err := json.NewDecoder(rr.Body).Decode(&response)
				require.NoError(t, err)
				assert.Equal(t, "ok", response["status"])
				assert.NotEmpty(t, response["user_id"])
			}
		})
	}
}

func TestLogin(t *testing.T) {
	setupTestEnvironment()

	// First register a user
	reqBody := map[string]string{
		"email":    "login-test@example.com",
		"password": "StrongPass123",
	}
	reqBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/register", strings.NewReader(string(reqBytes)))
	rr := httptest.NewRecorder()
	Register(rr, req)

	tests := []struct {
		name           string
		email          string
		password       string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "valid login",
			email:          "login-test@example.com",
			password:       "StrongPass123",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "invalid credentials",
			email:          "login-test@example.com",
			password:       "wrongpassword",
			expectedStatus: http.StatusUnauthorized,
			expectedError:  InvalidCredentialsError,
		},
		{
			name:           "user not found",
			email:          "nonexistent@example.com",
			password:       "StrongPass123",
			expectedStatus: http.StatusUnauthorized,
			expectedError:  InvalidCredentialsError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqBody := map[string]string{
				"email":    tt.email,
				"password": tt.password,
			}
			reqBytes, _ := json.Marshal(reqBody)

			req := httptest.NewRequest("POST", "/login", strings.NewReader(string(reqBytes)))
			rr := httptest.NewRecorder()

			Login(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)

			if tt.expectedStatus != http.StatusOK {
				assert.Contains(t, rr.Body.String(), tt.expectedError)
			} else {
				var response map[string]interface{}
				err := json.NewDecoder(rr.Body).Decode(&response)
				require.NoError(t, err)
				assert.NotEmpty(t, response["token"])

				// Verify JWT token
				tokenStr := response["token"].(string)
				token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
					return []byte("test-jwt-secret"), nil
				})
				require.NoError(t, err)
				assert.True(t, token.Valid)
			}
		})
	}
}

func TestAuthMiddleware(t *testing.T) {
	setupTestEnvironment()

	// Create a valid JWT token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": "test-user-id",
		"email":   "test@example.com",
		"role":    "user",
		"exp":     time.Now().Add(1 * time.Hour).Unix(),
	})
	tokenString, _ := token.SignedString([]byte("test-jwt-secret"))

	tests := []struct {
		name           string
		authHeader     string
		expectedStatus int
	}{
		{
			name:           "valid token",
			authHeader:     "Bearer " + tokenString,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "invalid token",
			authHeader:     "Bearer invalid.token.string",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "no token",
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/me", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			rr := httptest.NewRecorder()

			// Create a test user
			user := User{
				ID:    "test-user-id",
				Email: "test@example.com",
				Role:  "user",
			}
			db.Create(&user)

			handler := AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})
			handler.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)
		})
	}
}



