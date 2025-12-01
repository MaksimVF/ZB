





package test

import (
    "context"
    "fmt"
    "io"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/stretchr/testify/suite"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/trace"
    "google.golang.org/grpc"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
    "google.golang.org/grpc/test/bufconn"

    gen "github.com/yourorg/head/gen"
    "github.com/yourorg/head/internal/config"
    "github.com/yourorg/head/internal/server"
)

// TestSuite is the base test suite
type TestSuite struct {
    suite.Suite
    cfg        *config.Config
    server      *server.HeadServer
    bufDialer   func(context.Context, string) (net.Conn, error)
    conn        *grpc.ClientConn
    client      gen.ChatServiceClient
    testTimeout time.Duration
}

// SetupSuite sets up the test suite
func (s *TestSuite) SetupSuite() {
    // Initialize configuration
    s.cfg = config.Load()
    s.cfg.GRPCAddr = "bufnet"

    // Initialize server
    s.server = server.New(s.cfg)

    // Set up bufconn listener
    listener := bufconn.Listen(1024 * 1024)
    s.bufDialer = func(context.Context, string) (net.Conn, error) {
        return listener.Dial()
    }

    // Start server in a goroutine
    go func() {
        if err := s.server.Run(); err != nil {
            fmt.Printf("Server failed: %v\n", err)
        }
    }()

    // Set up client connection
    var err error
    s.conn, err = grpc.DialContext(context.Background(), "bufnet", grpc.WithContextDialer(s.bufDialer), grpc.WithInsecure())
    require.NoError(s.T(), err)

    s.client = gen.NewChatServiceClient(s.conn)
    s.testTimeout = 5 * time.Second
}

// TearDownSuite tears down the test suite
func (s *TestSuite) TearDownSuite() {
    if s.conn != nil {
        s.conn.Close()
    }
}

// TestChatCompletion tests the ChatCompletion endpoint
func (s *TestSuite) TestChatCompletion() {
    ctx, cancel := context.WithTimeout(context.Background(), s.testTimeout)
    defer cancel()

    req := &gen.ChatRequest{
        RequestId:   "test-request",
        Model:       "gpt-4o",
        Messages:    []*gen.Message{{Role: "user", Content: "Hello, world!"}},
        Temperature: 0.7,
        MaxTokens:   100,
    }

    resp, err := s.client.ChatCompletion(ctx, req)
    require.NoError(s.T(), err)
    assert.NotNil(s.T(), resp)
    assert.NotEmpty(s.T(), resp.FullText)
    assert.Equal(s.T(), "gpt-4o", resp.Model)
    assert.Equal(s.T(), "test-request", resp.RequestId)
    assert.Greater(s.T(), resp.TokensUsed, int32(0))
}

// TestChatCompletionStream tests the ChatCompletionStream endpoint
func (s *TestSuite) TestChatCompletionStream() {
    ctx, cancel := context.WithTimeout(context.Background(), s.testTimeout)
    defer cancel()

    req := &gen.ChatRequest{
        RequestId:   "test-stream-request",
        Model:       "gpt-4o",
        Messages:    []*gen.Message{{Role: "user", Content: "Hello, world!"}},
        Temperature: 0.7,
        MaxTokens:   100,
    }

    stream, err := s.client.ChatCompletionStream(ctx, req)
    require.NoError(s.T(), err)
    require.NotNil(s.T(), stream)

    var fullText string
    var tokensUsed int32

    for {
        resp, err := stream.Recv()
        if err != nil {
            if err == io.EOF {
                break
            }
            require.NoError(s.T(), err)
        }

        require.NotNil(s.T(), resp)
        fullText += resp.Chunk
        if resp.TokensUsed > tokensUsed {
            tokensUsed = resp.TokensUsed
        }
    }

    assert.NotEmpty(s.T(), fullText)
    assert.Greater(s.T(), tokensUsed, int32(0))
}

// TestAuthentication tests authentication
func (s *TestSuite) TestAuthentication() {
    // Enable authentication
    s.cfg.FeaturesConfig.SetEnabled("authentication", true)

    // Test without token
    ctx, cancel := context.WithTimeout(context.Background(), s.testTimeout)
    defer cancel()

    req := &gen.ChatRequest{
        RequestId:   "test-auth-request",
        Model:       "gpt-4o",
        Messages:    []*gen.Message{{Role: "user", Content: "Hello, world!"}},
        Temperature: 0.7,
        MaxTokens:   100,
    }

    _, err := s.client.ChatCompletion(ctx, req)
    require.Error(s.T(), err)
    st, ok := status.FromError(err)
    require.True(s.T(), ok)
    assert.Equal(s.T(), codes.Unauthenticated, st.Code())
}

// TestCircuitBreaker tests circuit breaker functionality
func (s *TestSuite) TestCircuitBreaker() {
    // Enable circuit breaker
    s.cfg.FeaturesConfig.SetEnabled("circuit_breaker", true)

    // TODO: Implement circuit breaker test
}

// TestWebhook tests webhook functionality
func (s *TestSuite) TestWebhook() {
    // Enable webhook
    s.cfg.FeaturesConfig.SetEnabled("webhook", true)

    // TODO: Implement webhook test
}

// TestModelRegistry tests model registry functionality
func (s *TestSuite) TestModelRegistry() {
    // Enable model registry
    s.cfg.FeaturesConfig.SetEnabled("model_registry", true)

    // Test with invalid model
    ctx, cancel := context.WithTimeout(context.Background(), s.testTimeout)
    defer cancel()

    req := &gen.ChatRequest{
        RequestId:   "test-model-request",
        Model:       "invalid-model",
        Messages:    []*gen.Message{{Role: "user", Content: "Hello, world!"}},
        Temperature: 0.7,
        MaxTokens:   100,
    }

    _, err := s.client.ChatCompletion(ctx, req)
    require.Error(s.T(), err)
    st, ok := status.FromError(err)
    require.True(s.T(), ok)
    assert.Equal(s.T(), codes.InvalidArgument, st.Code())
}

// RunTests runs all tests
func RunTests() {
    suite.Run(&TestSuite{})
}

// TestTracing tests OpenTelemetry tracing
func (s *TestSuite) TestTracing() {
    // Initialize tracing
    ctx := context.Background()
    tracer := otel.GetTracerProvider().Tracer("head-go")
    ctx, span := tracer.Start(ctx, "TestTracing")
    defer span.End()

    span.SetAttributes(
        attribute.String("test", "tracing"),
        attribute.Int("value", 42),
    )

    // Verify span is created
    assert.NotNil(s.T(), span)
}

// TestLoadTesting tests load handling
func (s *TestSuite) TestLoadTesting() {
    // TODO: Implement load testing
}

// TestChaosTesting tests chaos scenarios
func (s *TestSuite) TestChaosTesting() {
    // TODO: Implement chaos testing
}





