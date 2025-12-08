


package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/hashicorp/vault/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	pb "github.com/MaksimVF/ZB/services/secrets-service/pb"
)

func setupTestEnvironment() {
	// Set environment variables for testing
	os.Setenv("VAULT_ADDR", "http://vault:8200")
	os.Setenv("VAULT_TOKEN", "test-token")
	os.Setenv("ADMIN_KEY", "test-admin-key")

	// Initialize the service
	init()
}

func TestGetSecret(t *testing.T) {
	setupTestEnvironment()

	s := &server{}
	tests := []struct {
		name        string
		secretName  string
		mockResponse *api.Secret
		mockError   error
		expectedErrorCode codes.Code
	}{
		{
			name:       "successful secret retrieval",
			secretName: "llm/openai/api_key",
			mockResponse: &api.Secret{
				Data: map[string]interface{}{
					"data": map[string]interface{}{
						"value": "test-secret-value",
					},
				},
			},
			expectedErrorCode: codes.OK,
		},
		{
			name:        "secret not found",
			secretName:  "nonexistent/secret",
			mockResponse: nil,
			expectedErrorCode: codes.NotFound,
		},
		{
			name:        "vault connection error",
			secretName:  "llm/openai/api_key",
			mockError:   fmt.Errorf("connection failed"),
			expectedErrorCode: codes.Internal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock the Vault client behavior
			mockLogical := &MockLogical{}
			if tt.mockError != nil {
				mockLogical.On("Read", "secret/data/"+tt.secretName).Return(nil, tt.mockError)
			} else {
				mockLogical.On("Read", "secret/data/"+tt.secretName).Return(tt.mockResponse, nil)
			}

			// Replace the real Vault client with our mock
			originalClient := vaultClient
			vaultClient = &api.Client{Logical: mockLogical}
			defer func() { vaultClient = originalClient }()

			// Call the method
			req := &pb.GetSecretRequest{Name: tt.secretName}
			resp, err := s.GetSecret(context.Background(), req)

			// Check the result
			if tt.expectedErrorCode == codes.OK {
				require.NoError(t, err)
				assert.NotNil(t, resp)
				assert.Equal(t, "test-secret-value", resp.Value)
			} else {
				require.Error(t, err)
				assert.Nil(t, resp)
				statusErr, ok := status.FromError(err)
				require.True(t, ok)
				assert.Equal(t, tt.expectedErrorCode, statusErr.Code())
			}

			mockLogical.AssertExpectations(t)
		})
	}
}

func TestAdminHandler(t *testing.T) {
	setupTestEnvironment()

	tests := []struct {
		name           string
		method         string
		path           string
		adminKey       string
		body           string
		expectedStatus int
	}{
		{
			name:           "invalid admin key",
			method:         http.MethodGet,
			path:           "/admin/api/secrets",
			adminKey:       "wrong-key",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "valid get request",
			method:         http.MethodGet,
			path:           "/admin/api/secrets",
			adminKey:       "test-admin-key",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "valid post request",
			method:         http.MethodPost,
			path:           "/admin/api/secrets",
			adminKey:       "test-admin-key",
			body:           `{"path": "llm/test/api_key", "value": "test-value"}`,
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			if tt.body != "" {
				req = httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			}
			req.Header.Set("X-Admin-Key", tt.adminKey)

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(adminHandler)
			handler.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)

			if tt.expectedStatus == http.StatusOK && tt.method == http.MethodGet {
				var response map[string]interface{}
				err := json.NewDecoder(rr.Body).Decode(&response)
				require.NoError(t, err)
				assert.NotEmpty(t, response)
			}
		})
	}
}

// MockLogical is a mock implementation of the Vault Logical interface
type MockLogical struct {
	mock.Mock
}

func (m *MockLogical) Read(path string) (*api.Secret, error) {
	args := m.Called(path)
	return args.Get(0).(*api.Secret), args.Error(1)
}

func (m *MockLogical) Write(path string, data map[string]interface{}) (*api.Secret, error) {
	args := m.Called(path, data)
	return args.Get(0).(*api.Secret), args.Error(1)
}

func (m *MockLogical) Delete(path string) (*api.Secret, error) {
	args := m.Called(path)
	return args.Get(0).(*api.Secret), args.Error(1)
}

func (m *MockLogical) List(path string) (*api.Secret, error) {
	args := m.Called(path)
	return args.Get(0).(*api.Secret), args.Error(1)
}

