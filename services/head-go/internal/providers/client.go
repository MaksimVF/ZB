package providers

import (
"context"
"crypto/tls"
"crypto/x509"
"io"
"io/ioutil"
"time"

"google.golang.org/grpc"
"google.golang.org/grpc/credentials"

model "github.com/yourorg/head/gen_model" // сюда попадают model.proto
)

// ModelClient — обёртка над gRPC-клиентом к model-proxy
type ModelClient struct {
addr string
conn *grpc.ClientConn
stub model.ModelServiceClient
}

// NewModelClient создаёт клиент, но ещё не подключается
func NewModelClient(addr string) *ModelClient {
return &ModelClient{addr: addr}
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
tlsCreds, err := loadTLSCredentials()
if err != nil {
return err
}

conn, err := grpc.DialContext(ctx, m.addr,
grpc.WithTransportCredentials(tlsCreds),
grpc.WithBlock(),
grpc.WithTimeout(10*time.Second),
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
errCh <- err
return
}

for {
chunk, err := clientStream.Recv()
if err == io.EOF {
return
}
if err != nil {
errCh <- err
return
}
streamCh <- chunk
}
}()

return streamCh, errCh
}
