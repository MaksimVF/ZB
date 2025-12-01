package providers

import (
"context"
"crypto/tls"
"crypto/x509"
"io"
"io/ioutil"
"log"
"sync"
"time"

"github.com/sony/gobreaker"
"go.opentelemetry.io/otel"
"go.opentelemetry.io/otel/codes"
"go.opentelemetry.io/otel/trace"
"google.golang.org/grpc"
"google.golang.org/grpc/credentials"
"google.golang.org/grpc/keepalive"

model "github.com/yourorg/head/gen_model" // сюда попадают model.proto
)

// ModelClient — обёртка над gRPC-клиентом к model-proxy
type ModelClient struct {
addr            string
connPool        []*grpc.ClientConn
currentConn     int
stub            model.ModelServiceClient
circuitBreaker  *gobreaker.CircuitBreaker
retryConfig     RetryConfig
mu              sync.Mutex
}

// RetryConfig holds retry configuration
type RetryConfig struct {
MaxRetries    int
InitialBackoff time.Duration
MaxBackoff    time.Duration
}

// NewModelClient создаёт клиент, но ещё не подключается
func NewModelClient(addr string) *ModelClient {
return &ModelClient{
addr: addr,
retryConfig: RetryConfig{
MaxRetries:    3,
InitialBackoff: 100 * time.Millisecond,
MaxBackoff:    2 * time.Second,
},
circuitBreaker: gobreaker.NewCircuitBreaker(gobreaker.Settings{
Name:        "model-proxy",
MaxRequests:  1,
Interval:    10 * time.Second,
Timeout:     30 * time.Second,
ReadyToTrip: func(counts gobreaker.Counts) bool {
failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
return counts.Requests >= 3 && failureRatio >= 0.6
},
}),
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

// Init initializes the model client with connection pooling and circuit breakers
func (m *ModelClient) Init(ctx context.Context) error {
tlsCreds, err := loadTLSCredentials()
if err != nil {
return err
}

// Create connection pool (2 connections for redundancy)
connPoolSize := 2
m.connPool = make([]*grpc.ClientConn, connPoolSize)

for i := 0; i < connPoolSize; i++ {
conn, err := grpc.DialContext(ctx, m.addr,
grpc.WithTransportCredentials(tlsCreds),
grpc.WithBlock(),
grpc.WithTimeout(10*time.Second),
grpc.WithKeepaliveParams(keepalive.ClientParameters{
Time:                10 * time.Second, // send pings every 10 seconds if there is no activity
Timeout:             2 * time.Second, // wait 2 seconds for pong response
PermitWithoutStream: true,
}),
)
if err != nil {
return err
}
m.connPool[i] = conn
}

// Use the first connection for the stub
m.stub = model.NewModelServiceClient(m.connPool[0])
return nil
}

// getNextConnection returns the next connection in the pool with round-robin
func (m *ModelClient) getNextConnection() *grpc.ClientConn {
m.mu.Lock()
defer m.mu.Unlock()

conn := m.connPool[m.currentConn]
m.currentConn = (m.currentConn + 1) % len(m.connPool)
return conn
}

// Close закрывает все соединения
func (m *ModelClient) Close() {
m.mu.Lock()
defer m.mu.Unlock()

for _, conn := range m.connPool {
if conn != nil {
conn.Close()
}
}
}

// withRetry executes a function with retry logic and exponential backoff
func (m *ModelClient) withRetry(ctx context.Context, operation string, fn func() error) error {
var lastErr error
backoff := m.retryConfig.InitialBackoff

for attempt := 0; attempt <= m.retryConfig.MaxRetries; attempt++ {
// Check if context is cancelled
select {
case <-ctx.Done():
return ctx.Err()
default:
}

// Execute the operation
err := fn()
if err == nil {
return nil
}

lastErr = err
log.Printf("Attempt %d failed for %s: %v", attempt+1, operation, err)

// If this is the last attempt, break
if attempt == m.retryConfig.MaxRetries {
break
}

// Exponential backoff with jitter
time.Sleep(backoff)
backoff = backoff * 2
if backoff > m.retryConfig.MaxBackoff {
backoff = m.retryConfig.MaxBackoff
}
}

return lastErr
}

// withCircuitBreaker executes a function with circuit breaker protection
func (m *ModelClient) withCircuitBreaker(ctx context.Context, operation string, fn func() (interface{}, error)) (interface{}, error) {
result, err := m.circuitBreaker.Execute(func() (interface{}, error) {
// Start a span for the circuit breaker operation
tracer := otel.GetTracerProvider().Tracer("head-go")
ctx, span := tracer.Start(ctx, "CircuitBreaker_"+operation)
defer span.End()

// Execute the function
res, err := fn()
if err != nil {
span.SetStatus(codes.Error, err.Error())
span.RecordError(err)
return nil, err
}

return res, nil
})

if err != nil {
log.Printf("Circuit breaker error for %s: %v", operation, err)
return nil, err
}

return result, nil
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

req := &model.GenRequest{
RequestId:   "",
Model:       modelName,
Messages:    messages,
Temperature: temperature,
MaxTokens:   maxTokens,
Stream:      false,
}

// Execute with circuit breaker and retry logic
result, err := m.withCircuitBreaker(ctx, "Generate", func() (interface{}, error) {
var resp *model.GenResponse

err := m.withRetry(ctx, "Generate", func() error {
// Get next connection from pool
conn := m.getNextConnection()
client := model.NewModelServiceClient(conn)

var err error
resp, err = client.Generate(ctx, req)
return err
})

if err != nil {
return nil, err
}

return resp, nil
})

if err != nil {
span.SetStatus(codes.Error, err.Error())
span.RecordError(err)
return "", 0, err
}

resp := result.(*model.GenResponse)
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

req := &model.GenRequest{
RequestId:   "",
Model:       modelName,
Messages:    messages,
Temperature: temperature,
MaxTokens:   maxTokens,
Stream:      true,
}

// Execute with circuit breaker and retry logic
_, err := m.withCircuitBreaker(ctx, "GenerateStream", func() (interface{}, error) {
var clientStream model.ModelService_GenerateStreamClient

err := m.withRetry(ctx, "GenerateStream", func() error {
// Get next connection from pool
conn := m.getNextConnection()
client := model.NewModelServiceClient(conn)

var err error
clientStream, err = client.GenerateStream(ctx, req)
return err
})

if err != nil {
return nil, err
}

// Process the stream
for {
chunk, err := clientStream.Recv()
if err == io.EOF {
return nil, nil
}
if err != nil {
return nil, err
}
streamCh <- chunk
}
})

if err != nil {
span.SetStatus(codes.Error, err.Error())
span.RecordError(err)
errCh <- err
}
}()

return streamCh, errCh
}
