






package test

import (
    "context"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "google.golang.org/grpc"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
    "google.golang.org/grpc/test/bufconn"

    gen "github.com/yourorg/head/gen"
    "github.com/yourorg/head/internal/config"
    "github.com/yourorg/head/internal/server"
)

// ContractTestSuite tests API contracts
type ContractTestSuite struct {
    suite.Suite
    cfg        *config.Config
    server      *server.HeadServer
    bufDialer   func(context.Context, string) (net.Conn, error)
    conn        *grpc.ClientConn
    client      gen.ChatServiceClient
    testTimeout time.Duration
}

// SetupSuite sets up the contract test suite
func (s *ContractTestSuite) SetupSuite() {
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

// TearDownSuite tears down the contract test suite
func (s *ContractTestSuite) TearDownSuite() {
    if s.conn != nil {
        s.conn.Close()
    }
}

// TestChatCompletionContract tests the ChatCompletion contract
func (s *ContractTestSuite) TestChatCompletionContract() {
    ctx, cancel := context.WithTimeout(context.Background(), s.testTimeout)
    defer cancel()

    req := &gen.ChatRequest{
        RequestId:   "contract-test-request",
        Model:       "gpt-4o",
        Messages:    []*gen.Message{{Role: "user", Content: "Hello, world!"}},
        Temperature: 0.7,
        MaxTokens:   100,
    }

    resp, err := s.client.ChatCompletion(ctx, req)
    require.NoError(s.T(), err)
    require.NotNil(s.T(), resp)

    // Validate contract
    assert.Equal(s.T(), req.RequestId, resp.RequestId)
    assert.NotEmpty(s.T(), resp.FullText)
    assert.NotEmpty(s.T(), resp.Model)
    assert.NotEmpty(s.T(), resp.Provider)
    assert.Greater(s.T(), resp.TokensUsed, int32(0))
}

// TestChatCompletionStreamContract tests the ChatCompletionStream contract
func (s *ContractTestSuite) TestChatCompletionStreamContract() {
    ctx, cancel := context.WithTimeout(context.Background(), s.testTimeout)
    defer cancel()

    req := &gen.ChatRequest{
        RequestId:   "contract-stream-test-request",
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
    var finalChunkReceived bool

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
        if resp.IsFinal {
            finalChunkReceived = true
        }

        // Validate contract for each chunk
        assert.Equal(s.T(), req.RequestId, resp.RequestId)
        assert.NotEmpty(s.T(), resp.Provider)
        assert.GreaterOrEqual(s.T(), resp.TokensUsed, int32(0))
    }

    // Validate final state
    assert.True(s.T(), finalChunkReceived)
    assert.NotEmpty(s.T(), fullText)
    assert.Greater(s.T(), tokensUsed, int32(0))
}

// TestErrorContracts tests error handling contracts
func (s *ContractTestSuite) TestErrorContracts() {
    // Test invalid model
    ctx, cancel := context.WithTimeout(context.Background(), s.testTimeout)
    defer cancel()

    req := &gen.ChatRequest{
        RequestId:   "error-test-request",
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

// TestAuthenticationContract tests authentication contract
func (s *ContractTestSuite) TestAuthenticationContract() {
    // Enable authentication
    s.cfg.FeaturesConfig.SetEnabled("authentication", true)

    ctx, cancel := context.WithTimeout(context.Background(), s.testTimeout)
    defer cancel()

    req := &gen.ChatRequest{
        RequestId:   "auth-test-request",
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

// RunContractTests runs all contract tests
func RunContractTests() {
    suite.Run(&ContractTestSuite{})
}





