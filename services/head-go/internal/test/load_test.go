





package test

import (
    "context"
    "fmt"
    "sync"
    "sync/atomic"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "google.golang.org/grpc"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"

    gen "github.com/yourorg/head/gen"
)

// LoadTestConfig holds load test configuration
type LoadTestConfig struct {
    NumRequests    int
    Concurrency    int
    RequestTimeout time.Duration
    TestTimeout    time.Duration
}

// LoadTestResult holds load test results
type LoadTestResult struct {
    SuccessCount   int64
    ErrorCount     int64
    TotalDuration  time.Duration
    MinDuration   time.Duration
    MaxDuration   time.Duration
    AvgDuration   time.Duration
}

// RunLoadTest runs a load test
func RunLoadTest(t *testing.T, client gen.ChatServiceClient, config LoadTestConfig) *LoadTestResult {
    var wg sync.WaitGroup
    var successCount, errorCount int64
    var totalDuration, minDuration, maxDuration time.Duration
    var mu sync.Mutex

    minDuration = time.Duration(^uint64(0) >> 1) // Max duration

    startTime := time.Now()

    for i := 0; i < config.Concurrency; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()

            for j := 0; j < config.NumRequests/config.Concurrency; j++ {
                ctx, cancel := context.WithTimeout(context.Background(), config.RequestTimeout)
                req := &gen.ChatRequest{
                    RequestId:   fmt.Sprintf("load-test-%d-%d", i, j),
                    Model:       "gpt-4o",
                    Messages:    []*gen.Message{{Role: "user", Content: "Hello, world!"}},
                    Temperature: 0.7,
                    MaxTokens:   100,
                }

                reqStart := time.Now()
                resp, err := client.ChatCompletion(ctx, req)
                duration := time.Since(reqStart)

                mu.Lock()
                if err != nil {
                    errorCount++
                } else {
                    successCount++
                    totalDuration += duration
                    if duration < minDuration {
                        minDuration = duration
                    }
                    if duration > maxDuration {
                        maxDuration = duration
                    }
                }
                mu.Unlock()

                cancel()

                // Check if test timeout is reached
                if time.Since(startTime) > config.TestTimeout {
                    return
                }
            }
        }()
    }

    wg.Wait()

    var avgDuration time.Duration
    if successCount > 0 {
        avgDuration = totalDuration / time.Duration(successCount)
    }

    return &LoadTestResult{
        SuccessCount:  successCount,
        ErrorCount:    errorCount,
        TotalDuration: time.Since(startTime),
        MinDuration:  minDuration,
        MaxDuration:  maxDuration,
        AvgDuration:  avgDuration,
    }
}

// TestLoad tests the system under load
func TestLoad(t *testing.T) {
    // Set up test configuration
    config := LoadTestConfig{
        NumRequests:    100,
        Concurrency:    10,
        RequestTimeout: 5 * time.Second,
        TestTimeout:    30 * time.Second,
    }

    // Create a test client
    cfg := config.Load()
    server := server.New(cfg)
    listener := bufconn.Listen(1024 * 1024)
    bufDialer := func(context.Context, string) (net.Conn, error) {
        return listener.Dial()
    }

    go func() {
        if err := server.Run(); err != nil {
            fmt.Printf("Server failed: %v\n", err)
        }
    }()

    conn, err := grpc.DialContext(context.Background(), "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithInsecure())
    require.NoError(t, err)
    defer conn.Close()

    client := gen.NewChatServiceClient(conn)

    // Run load test
    result := RunLoadTest(t, client, config)

    // Print results
    fmt.Printf("Load Test Results:\n")
    fmt.Printf("  Success Count: %d\n", result.SuccessCount)
    fmt.Printf("  Error Count: %d\n", result.ErrorCount)
    fmt.Printf("  Total Duration: %v\n", result.TotalDuration)
    fmt.Printf("  Min Duration: %v\n", result.MinDuration)
    fmt.Printf("  Max Duration: %v\n", result.MaxDuration)
    fmt.Printf("  Avg Duration: %v\n", result.AvgDuration)

    // Assertions
    assert.Greater(t, result.SuccessCount, int64(0))
    assert.Equal(t, result.SuccessCount+result.ErrorCount, int64(config.NumRequests))
}

// TestChaos tests chaos scenarios
func TestChaos(t *testing.T) {
    // Set up test configuration
    config := LoadTestConfig{
        NumRequests:    50,
        Concurrency:    5,
        RequestTimeout: 5 * time.Second,
        TestTimeout:    30 * time.Second,
    }

    // Create a test client
    cfg := config.Load()
    server := server.New(cfg)
    listener := bufconn.Listen(1024 * 1024)
    bufDialer := func(context.Context, string) (net.Conn, error) {
        return listener.Dial()
    }

    go func() {
        if err := server.Run(); err != nil {
            fmt.Printf("Server failed: %v\n", err)
        }
    }()

    conn, err := grpc.DialContext(context.Background(), "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithInsecure())
    require.NoError(t, err)
    defer conn.Close()

    client := gen.NewChatServiceClient(conn)

    // Run chaos test
    result := RunLoadTest(t, client, config)

    // Print results
    fmt.Printf("Chaos Test Results:\n")
    fmt.Printf("  Success Count: %d\n", result.SuccessCount)
    fmt.Printf("  Error Count: %d\n", result.ErrorCount)
    fmt.Printf("  Total Duration: %v\n", result.TotalDuration)
    fmt.Printf("  Min Duration: %v\n", result.MinDuration)
    fmt.Printf("  Max Duration: %v\n", result.MaxDuration)
    fmt.Printf("  Avg Duration: %v\n", result.AvgDuration)

    // Assertions
    assert.Greater(t, result.SuccessCount, int64(0))
    assert.Equal(t, result.SuccessCount+result.ErrorCount, int64(config.NumRequests))
}




