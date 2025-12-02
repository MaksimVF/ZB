package providers

import (
    "context"
    "crypto/tls"
    "crypto/x509"
    "fmt"
    "io"
    "io/ioutil"
    "log"
    "sync"
    "sync/atomic"
    "time"

    "github.com/afex/hystrix-go/hystrix"
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/codes"
    "go.opentelemetry.io/otel/trace"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials"
    "google.golang.org/grpc/keepalive"

    model "github.com/yourorg/head/gen_model" // сюда попадают model.proto
)

var (
    modelRequestLatency = promauto.NewHistogramVec(
        prometheus.HistogramOpts{Name: "model_request_latency_seconds", Help: "Model request latency"},
        []string{"model"},
    )
    modelRequestErrors = promauto.NewCounterVec(
        prometheus.CounterOpts{Name: "model_request_errors_total", Help: "Total model request errors"},
        []string{"model", "error_type"},
    )
    activeModelConnections = promauto.NewGauge(
        prometheus.GaugeOpts{Name: "model_active_connections", Help: "Currently active model connections"},
    )
    circuitBreakerErrors = promauto.NewCounterVec(
        prometheus.CounterOpts{Name: "circuit_breaker_errors_total", Help: "Total circuit breaker errors"},
        []string{"model", "error_type"},
    )
)

// ModelClient — обёртка над gRPC-клиентом к model-proxy
type ModelClient struct {
    addr string
    conn *grpc.ClientConn
    stub model.ModelServiceClient
    activeRequests int32
    connectionPool *sync.Pool
    maxConnections int
    connectionCount int32
}

// NewModelClient создаёт клиент, но ещё не подключается
func NewModelClient(addr string) *ModelClient {
    return &ModelClient{
        addr: addr,
        maxConnections: 100, // Default max connections
        connectionPool: &sync.Pool{
            New: func() interface{} {
                return &grpc.ClientConn{}
            },
        },
    }
}

// loadTLSCredentials загружает сертификаты для mTLS
func loadTLSCredentials() (credentials.TransportCredentials, error) {
    // Сертификат и ключ клиента (head)
    cert, err := tls.LoadX509KeyPair("/certs/head.pem", "/certs/head-key.pem")
    if err != nil {
        return nil, err
    }

    // CA для проверки сервера model-proxy
    caCert, err := ioutil.ReadFile("/certs/ca.pem")
    if err != nil {
        return nil, err
    }
    caCertPool := x509.NewCertPool()
    if !caCertPool.AppendCertsFromPEM(caCert) {
        return nil, err
    }

    // Настраиваем TLS с взаимной аутентификацией
    config := &tls.Config{
        ServerName:   "model-proxy", // Должно совпадать с CN в сертификате model-proxy
        Certificates: []tls.Certificate{cert},
        RootCAs:      caCertPool,
    }

    return credentials.NewTLS(config), nil
}

// Init(ctx context.Context) error {
func (m *ModelClient) Init(ctx context.Context) error {
    tlsCreds, err := loadTLSCredentials()
    if err != nil {
        return err
    }

    // Configure keepalive for better connection management
    keepaliveParams := keepalive.ClientParameters{
        Time:                10 * time.Second, // ping every 10 seconds
        Timeout:            2 * time.Second,  // wait 2 seconds for pong
        PermitWithoutStream: true,
    }

    conn, err := grpc.DialContext(ctx, m.addr,
        grpc.WithTransportCredentials(tlsCreds),
        grpc.WithBlock(),
        grpc.WithTimeout(10*time.Second),
        grpc.WithKeepaliveParams(keepaliveParams),
    )
    if err != nil {
        return err
    }

    m.conn = conn
    m.stub = model.NewModelServiceClient(conn)

    // Initialize circuit breakers for different models
    hystrix.ConfigureCommand("model_generate", hystrix.CommandConfig{
        Timeout:                5000, // 5 seconds
        MaxConcurrentRequests:  50,
        ErrorPercentThreshold:  30,
        SleepWindow:            10000, // 10 seconds
        RequestVolumeThreshold: 10,
    })

    hystrix.ConfigureCommand("model_generate_stream", hystrix.CommandConfig{
        Timeout:                10000, // 10 seconds
        MaxConcurrentRequests:  30,
        ErrorPercentThreshold:  25,
        SleepWindow:            15000, // 15 seconds
        RequestVolumeThreshold: 5,
    })

    // Initialize connection pool
    for i := 0; i < m.maxConnections; i++ {
        conn, err := grpc.DialContext(ctx, m.addr,
            grpc.WithTransportCredentials(tlsCreds),
            grpc.WithBlock(),
            grpc.WithTimeout(10*time.Second),
            grpc.WithKeepaliveParams(keepaliveParams),
        )
        if err != nil {
            log.Printf("Failed to create connection for pool: %v", err)
            continue
        }
        m.connectionPool.Put(conn)
        atomic.AddInt32(&m.connectionCount, 1)
    }

    return nil
}

// ReturnConnectionToPool возвращает соединение обратно в пул
func (m *ModelClient) ReturnConnectionToPool(conn *grpc.ClientConn) {
    if m.connectionPool != nil && conn != nil {
        m.connectionPool.Put(conn)
    }
}

// BatchGenerate — пакетная обработка запросов к модели
func (m *ModelClient) BatchGenerate(
    ctx context.Context,
    requests []*model.GenRequest,
) (*model.BatchGenResponse, error) {
    // Start a span for the BatchGenerate operation
    tracer := otel.GetTracerProvider().Tracer("head-go")
    ctx, span := tracer.Start(ctx, "ModelClient.BatchGenerate")
    defer span.End()

    span.SetAttributes(
        trace.StringAttribute("requests_count", fmt.Sprintf("%d", len(requests))),
    )

    // Increment active request count
    atomic.AddInt32(&m.activeRequests, 1)
    defer atomic.AddInt32(&m.activeRequests, -1)

    // Update active connections metric
    activeModelConnections.Set(float64(atomic.LoadInt32(&m.activeRequests)))

    start := time.Now()
    defer func() {
        modelRequestLatency.WithLabelValues("batch").Observe(time.Since(start).Seconds())
    }()

    // Create BatchGenRequest
    batchReq := &model.BatchGenRequest{
        Requests: requests,
    }

    // Get connection from pool or create new one
    var conn *grpc.ClientConn
    if m.connectionPool != nil {
        connInterface := m.connectionPool.Get()
        if connInterface != nil {
            conn = connInterface.(*grpc.ClientConn)
            if conn.GetState() != grpc.ConnectivityReady {
                // Connection not ready, create new one
                conn.Close()
                conn = nil
            }
        }
    }

    if conn == nil {
        // Fallback to default connection
        conn = m.conn
    }

    // Execute with circuit breaker
    var resp *model.BatchGenResponse
    err := hystrix.Do("model_generate", func() error {
        var innerErr error
        client := model.NewModelServiceClient(conn)
        resp, innerErr = client.BatchGenerate(ctx, batchReq)
        return innerErr
    }, nil)

    if err != nil {
        modelRequestErrors.WithLabelValues("batch", "batch_generate_error").Inc()
        circuitBreakerErrors.WithLabelValues("batch", "batch_circuit_breaker").Inc()
        return nil, err
    }

    // Return connection to pool if it's from the pool
    if conn != m.conn {
        m.ReturnConnectionToPool(conn)
    }

    return resp, nil
}

// Close закрывает все соединения
func (m *ModelClient) Close() {
    if m.conn != nil {
        m.conn.Close()
    }

    // Close all connections in the pool
    for {
        conn := m.connectionPool.Get()
        if conn == nil {
            break
        }
        if grpcConn, ok := conn.(*grpc.ClientConn); ok {
            grpcConn.Close()
        }
    }
}

// Generate — обычный (не стриминговый) вызов к модели с ретраями и circuit breaker
func (m *ModelClient) Generate(
    ctx context.Context,
    modelName string,
    messages []string,
    temperature float32,
    maxTokens int32,
) (text string, tokens int, err error) {
    // Start a span for the Generate operation
    tracer := otel.GetTracerProvider().Tracer("head-go")
    ctx, span := tracer.Start(ctx, "ModelClient.Generate")
    defer span.End()

    span.SetAttributes(
        trace.StringAttribute("model", modelName),
        trace.IntAttribute("max_tokens", int(maxTokens)),
        trace.Float64Attribute("temperature", float64(temperature)),
    )

    // Increment active request count
    atomic.AddInt32(&m.activeRequests, 1)
    defer atomic.AddInt32(&m.activeRequests, -1)

    // Update active connections metric
    activeModelConnections.Set(float64(atomic.LoadInt32(&m.activeRequests)))

    start := time.Now()
    defer func() {
        modelRequestLatency.WithLabelValues(modelName).Observe(time.Since(start).Seconds())
    }()

    req := &model.GenRequest{
        RequestId:   "",
        Model:       modelName,
        Messages:    messages,
        Temperature: temperature,
        MaxTokens:   maxTokens,
        Stream:      false,
    }

    // Get connection from pool or create new one
    var conn *grpc.ClientConn
    if m.connectionPool != nil {
        connInterface := m.connectionPool.Get()
        if connInterface != nil {
            conn = connInterface.(*grpc.ClientConn)
            if conn.GetState() != grpc.ConnectivityReady {
                // Connection not ready, create new one
                conn.Close()
                conn = nil
            }
        }
    }

    if conn == nil {
        // Fallback to default connection
        conn = m.conn
    }

    // Execute with circuit breaker
    var resp *model.GenResponse
    err = hystrix.Do("model_generate", func() error {
        var innerErr error
        client := model.NewModelServiceClient(conn)
        resp, innerErr = client.Generate(ctx, req)
        return innerErr
    }, nil)

    if err != nil {
        modelRequestErrors.WithLabelValues(modelName, "generate_error").Inc()
        circuitBreakerErrors.WithLabelValues(modelName, "generate_circuit_breaker").Inc()
        return "", 0, err
    }

    // Return connection to pool if it's from the pool
    if conn != m.conn {
        m.ReturnConnectionToPool(conn)
    }

    return resp.Text, int(resp.TokensUsed), nil
}

// GenerateStream — настоящий стриминговый вызов Возвращает канал, по которому приходят чанки
func (m *ModelClient) GenerateStream(
    ctx context.Context,
    modelName string,
    messages []string,
    temperature float32,
    maxTokens int32,
) (<-chan *model.GenResponse, <-chan error) {
    // Start a span for the GenerateStream operation
    tracer := otel.GetTracerProvider().Tracer("head-go")
    ctx, span := tracer.Start(ctx, "ModelClient.GenerateStream")
    defer span.End()

    span.SetAttributes(
        trace.StringAttribute("model", modelName),
        trace.IntAttribute("max_tokens", int(maxTokens)),
        trace.Float64Attribute("temperature", float64(temperature)),
    )

    streamCh := make(chan *model.GenResponse, 10)
    errCh := make(chan error, 1)

    go func() {
        defer close(streamCh)
        defer close(errCh)

        // Start tracing span
        tracer := otel.Tracer("model-client")
        ctx, span := tracer.Start(ctx, "GenerateStream",
            trace.WithAttributes(
                attribute.String("model", modelName),
                attribute.Int("messages", len(messages)),
            ),
        )
        defer span.End()

        req := &model.GenRequest{
            RequestId:   "",
            Model:       modelName,
            Messages:    messages,
            Temperature: temperature,
            MaxTokens:   maxTokens,
            Stream:      true,
        }

        // Get connection from pool or create new one
        var conn *grpc.ClientConn
        if m.connectionPool != nil {
            connInterface := m.connectionPool.Get()
            if connInterface != nil {
                conn = connInterface.(*grpc.ClientConn)
                if conn.GetState() != grpc.ConnectivityReady {
                    // Connection not ready, create new one
                    conn.Close()
                    conn = nil
                }
            }
        }

        if conn == nil {
            // Fallback to default connection
            conn = m.conn
        }

        // Execute with circuit breaker
        var clientStream model.ModelService_GenerateStreamClient
        err := hystrix.Do("model_generate_stream", func() error {
            var innerErr error
            client := model.NewModelServiceClient(conn)
            clientStream, innerErr = client.GenerateStream(ctx, req)
            return innerErr
        }, nil)

        if err != nil {
            modelRequestErrors.WithLabelValues(modelName, "stream_error").Inc()
            circuitBreakerErrors.WithLabelValues(modelName, "stream_circuit_breaker").Inc()
            errCh <- err
            return
        }

        for {
            chunk, err := clientStream.Recv()
            if err == io.EOF {
                // Return connection to pool if it's from the pool
                if conn != m.conn {
                    m.ReturnConnectionToPool(conn)
                }
                return
            }
            if err != nil {
                modelRequestErrors.WithLabelValues(modelName, "stream_recv_error").Inc()
                errCh <- err
                return
            }
            streamCh <- chunk
        }
    }()

    return streamCh, errCh
}
