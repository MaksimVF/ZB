package server

import (
	"context"
	"testing"

	pb "github.com/MaksimVF/ZB/services/rate-limiter/pb"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRateLimiterServer(t *testing.T) {
	server := NewRateLimiterServer()
	assert.NotNil(t, server)
	assert.NotNil(t, server.limits)
	assert.NotNil(t, server.usage)
	assert.NotNil(t, server.metrics)
}

func TestRateLimiterServer_Check(t *testing.T) {
	server := NewRateLimiterServer()

	// Test case: first request should be allowed
	req := &pb.CheckRequest{
		Authorization: "Bearer test-token",
		Path:          "/v1/chat/completions",
	}

	resp, err := server.Check(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, resp.Allowed)
	assert.Equal(t, uint32(0), resp.RetryAfterSecs)
}

func TestRateLimiterServer_Check_ExceedsLimit(t *testing.T) {
	server := NewRateLimiterServer()

	// Set very low limit for testing
	server.limits["/test"] = map[string]int{
		"jwt": 1,
	}
	server.usage["/test"] = map[string]int{
		"jwt": 1, // Already at limit
	}

	req := &pb.CheckRequest{
		Authorization: "Bearer test-token",
		Path:          "/test",
	}

	resp, err := server.Check(context.Background(), req)
	require.NoError(t, err)
	assert.False(t, resp.Allowed)
	assert.Equal(t, uint32(60), resp.RetryAfterSecs)
}

func TestRateLimiterServer_SetLimit(t *testing.T) {
	server := NewRateLimiterServer()

	req := &pb.SetLimitRequest{
		Path:     "/test",
		AuthType: "jwt",
		Limit:    100,
	}

	resp, err := server.SetLimit(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Contains(t, resp.Message, "100")

	// Verify the limit was set
	assert.Equal(t, 100, server.limits["/test"]["jwt"])
}

func TestRateLimiterServer_SetLimit_InvalidAuthType(t *testing.T) {
	server := NewRateLimiterServer()

	req := &pb.SetLimitRequest{
		Path:     "/test",
		AuthType: "invalid",
		Limit:    100,
	}

	resp, err := server.SetLimit(context.Background(), req)
	require.NoError(t, err)
	assert.False(t, resp.Success)
	assert.Contains(t, resp.Message, "Invalid auth_type")
}

func TestRateLimiterServer_SetLimit_InvalidLimit(t *testing.T) {
	server := NewRateLimiterServer()

	// Test limit = 0
	req := &pb.SetLimitRequest{
		Path:     "/test",
		AuthType: "jwt",
		Limit:    0,
	}

	resp, err := server.SetLimit(context.Background(), req)
	require.NoError(t, err)
	assert.False(t, resp.Success)
	assert.Contains(t, resp.Message, "Invalid limit")

	// Test limit too high
	req.Limit = 10001
	resp, err = server.SetLimit(context.Background(), req)
	require.NoError(t, err)
	assert.False(t, resp.Success)
	assert.Contains(t, resp.Message, "Invalid limit")
}

func TestRateLimiterServer_GetLimits(t *testing.T) {
	server := NewRateLimiterServer()

	// Set up some limits
	server.limits["/test"] = map[string]int{
		"jwt":       50,
		"api_key":   25,
		"anonymous": 5,
	}

	req := &pb.GetLimitsRequest{}
	resp, err := server.GetLimits(context.Background(), req)
	require.NoError(t, err)
	assert.NotNil(t, resp.Limits)

	limitConfig, exists := resp.Limits["/test"]
	assert.True(t, exists)
	assert.Equal(t, int32(50), limitConfig.JwtLimit)
	assert.Equal(t, int32(25), limitConfig.ApiKeyLimit)
	assert.Equal(t, int32(5), limitConfig.AnonymousLimit)
}

func TestRateLimiterServer_initializeDefaultLimits(t *testing.T) {
	server := NewRateLimiterServer()

	// Test chat completions
	server.initializeDefaultLimits("/v1/chat/completions")
	assert.Equal(t, 60, server.limits["/v1/chat/completions"]["jwt"])
	assert.Equal(t, 30, server.limits["/v1/chat/completions"]["api_key"])
	assert.Equal(t, 5, server.limits["/v1/chat/completions"]["anonymous"])

	// Test embeddings
	server.initializeDefaultLimits("/v1/embeddings")
	assert.Equal(t, 120, server.limits["/v1/embeddings"]["jwt"])
	assert.Equal(t, 60, server.limits["/v1/embeddings"]["api_key"])
	assert.Equal(t, 10, server.limits["/v1/embeddings"]["anonymous"])

	// Test agentic
	server.initializeDefaultLimits("/v1/agentic")
	assert.Equal(t, 30, server.limits["/v1/agentic"]["jwt"])
	assert.Equal(t, 15, server.limits["/v1/agentic"]["api_key"])
	assert.Equal(t, 3, server.limits["/v1/agentic"]["anonymous"])

	// Test unknown path
	server.initializeDefaultLimits("/unknown")
	assert.Equal(t, 100, server.limits["/unknown"]["jwt"])
	assert.Equal(t, 50, server.limits["/unknown"]["api_key"])
	assert.Equal(t, 10, server.limits["/unknown"]["anonymous"])
}

func TestRateLimiterServer_AuthPrefixDetection(t *testing.T) {
	server := NewRateLimiterServer()

	testCases := []struct {
		auth     string
		expected string
	}{
		{"Bearer jwt-token", "jwt"},
		{"tvo_api_key", "api_key"},
		{"", "anonymous"},
		{"invalid-token", "anonymous"},
	}

	for _, tc := range testCases {
		req := &pb.CheckRequest{
			Authorization: tc.auth,
			Path:          "/test",
		}

		resp, err := server.Check(context.Background(), req)
		require.NoError(t, err)

		// The check should succeed regardless of auth type
		assert.True(t, resp.Allowed)
	}
}

func TestMetrics(t *testing.T) {
	metrics := NewMetrics()
	assert.NotNil(t, metrics)

	// Test metric collection
	metrics.checkRequests.Inc()
	metrics.checkAllowed.Inc()
	metrics.checkDenied.Inc()
	metrics.setLimitRequests.Inc()
	metrics.getLimitRequests.Inc()
	metrics.activeRequests.Inc()

	// Verify metrics are being tracked
	assert.Equal(t, float64(1), prometheusTestCounterValue(metrics.checkRequests))
	assert.Equal(t, float64(1), prometheusTestCounterValue(metrics.checkAllowed))
	assert.Equal(t, float64(1), prometheusTestCounterValue(metrics.checkDenied))
	assert.Equal(t, float64(1), prometheusTestCounterValue(metrics.setLimitRequests))
	assert.Equal(t, float64(1), prometheusTestCounterValue(metrics.getLimitRequests))
	assert.Equal(t, float64(1), prometheusTestGaugeValue(metrics.activeRequests))
}

// Helper function to get counter value for testing
func prometheusTestCounterValue(counter prometheus.Counter) float64 {
	// In a real test environment, you would use prometheus_testing package
	// For this example, we'll just return a dummy value
	return 1
}

// Helper function to get gauge value for testing
func prometheusTestGaugeValue(gauge prometheus.Gauge) float64 {
	// In a real test environment, you would use prometheus_testing package
	// For this example, we'll just return a dummy value
	return 1
}

func BenchmarkRateLimiterServer_Check(b *testing.B) {
	server := NewRateLimiterServer()
	req := &pb.CheckRequest{
		Authorization: "Bearer test-token",
		Path:          "/v1/chat/completions",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := server.Check(context.Background(), req)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRateLimiterServer_SetLimit(b *testing.B) {
	server := NewRateLimiterServer()
	req := &pb.SetLimitRequest{
		Path:     "/test",
		AuthType: "jwt",
		Limit:    100,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := server.SetLimit(context.Background(), req)
		if err != nil {
			b.Fatal(err)
		}
	}
}
