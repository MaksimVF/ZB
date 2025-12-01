




package providers

import (
    "context"
    "crypto/tls"
    "crypto/x509"
    "io"
    "io/ioutil"
    "time"
    "sync"
    "sync/atomic"

    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials"
    "google.golang.org/grpc/keepalive"

    model "github.com/yourorg/head/gen_model" // сюда попадают model.proto
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/trace"
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
)

// ModelClient — обёртка над gRPC-клиентом к model-proxy
type ModelClient struct {
    addr string
    conn *grpc.ClientConn
    stub model.ModelServiceClient
    activeRequests int32
    connectionPool *sync.Pool
}

// NewModelClient создаёт клиент, но ещё не подключается
func NewModelClient(addr string) *ModelClient {
    return &ModelClient{
        addr: addr,
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
    return nil
}

// Close закрывает соединение
func (m *ModelClient) Close() {
    if m.conn != nil {
        m.conn.Close()
    }
}

// Generate — обычный (не стриминговый) вызов к модели
func (m *ModelClient) Generate(
    ctx context.Context,
    modelName string,
    messages []string,
    temperature float32,
    maxTokens int32,
) (text string, tokens int, err error) {

    // Start tracing span
    tracer := otel.Tracer("model-client")
    ctx, span := tracer.Start(ctx, "Generate",
        trace.WithAttributes(
            attribute.String("model", modelName),
            attribute.Int("messages", len(messages)),
        ),
    )
    defer span.End()

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

    resp, err := m.stub.Generate(ctx, req)
    if err != nil {
        modelRequestErrors.WithLabelValues(modelName, "generate_error").Inc()
        return "", 0, err
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

        clientStream, err := m.stub.GenerateStream(ctx, req)
        if err != nil {
            modelRequestErrors.WithLabelValues(modelName, "stream_error").Inc()
            errCh <- err
            return
        }

        for {
            chunk, err := clientStream.Recv()
            if err == io.EOF {
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


