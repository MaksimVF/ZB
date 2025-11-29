package server

import (
"context"
"io"
"log"
"net"
"time"

gen "github.com/yourorg/head/gen" // сюда генерируются chat.proto
model "github.com/yourorg/head/gen_model" // сюда генерируются model.proto
"github.com/yourorg/head/internal/config"
modelclient "github.com/yourorg/head/internal/providers"

"google.golang.org/grpc"
"google.golang.org/grpc/codes"
"google.golang.org/grpc/status"

"github.com/prometheus/client_golang/prometheus"
"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
requestsTotal = promauto.NewCounterVec(
prometheus.CounterOpts{Name: "head_requests_total", Help: "Total requests"},
[]string{"model", "status"},
)
requestLatency = promauto.NewHistogramVec(
prometheus.HistogramOpts{Name: "head_request_latency_seconds", Help: "Request latency"},
[]string{"model"},
)
)

type HeadServer struct {
gen.UnimplementedChatServiceServer // встраиваем, чтобы не писать заглушки
cfg   *config.Config
model *modelclient.ModelClient
}

func New(cfg *config.Config) *HeadServer {
return &HeadServer{
cfg:   cfg,
model: modelclient.NewModelClient(cfg.ModelProxyAddr),
}
}

func (s *HeadServer) Run() error {
ctx := context.Background()
if err := s.model.Init(ctx); err != nil {
return err
}

srv := grpc.NewServer() // mTLS уже настроен на стороне клиента и сервера через credentials в Dial
gen.RegisterChatServiceServer(srv, s)

lis, err := net.Listen("tcp", s.cfg.GRPCAddr)
if err != nil {
return err
}

log.Printf("head gRPC+mTLS server listening on %s", s.cfg.GRPCAddr)
return srv.Serve(lis)
}

func (s *HeadServer) Shutdown(ctx context.Context) error {
done := make(chan struct{})
go func() {
s.model.Close()
close(done)
}()

select {
case <-ctx.Done():
return ctx.Err()
case <-done:
return nil
}
}

// Обычный (не стриминговый) запрос — возвращает полный текст сразу
func (s *HeadServer) ChatCompletion(ctx context.Context, req *gen.ChatRequest) (*gen.ChatResponse, error) {
start := time.Now()
modelName := req.Model
if modelName == "" {
modelName = "gpt-4o"
}

messages := make([]string, 0, len(req.Messages))
for _, m := range req.Messages {
messages = append(messages, m.Content)
}

text, tokens, err := s.model.Generate(ctx, modelName, messages, float32(req.Temperature), req.MaxTokens)
if err != nil {
requestsTotal.WithLabelValues(modelName, "error").Inc()
return nil, status.Errorf(codes.Internal, "model error: %v", err)
}

requestsTotal.WithLabelValues(modelName, "ok").Inc()
requestLatency.WithLabelValues(modelName).Observe(time.Since(start).Seconds())

	return &gen.ChatResponse{
	RequestId:  req.RequestId,
	FullText:   text,
	Model:      modelName,
	Provider:  "litellm",
	TokensUsed: int32(tokens),
	}, nil
}


// Стриминговый запрос — настоящий SSE-совместимый стриминг
func (s *HeadServer) ChatCompletionStream(req *gen.ChatRequest, stream gen.ChatService_ChatCompletionStreamServer) error {
ctx := stream.Context()
modelName := req.Model
if modelName == "" {
modelName = "gpt-4o"
}

messages := make([]string, 0, len(req.Messages))
for _, m := range req.Messages {
messages = append(messages, m.Content)
}

// Открываем стриминговый gRPC-клиент к model-proxy
grpcStream, err := s.model.stub.GenerateStream(ctx, &model.GenRequest{
RequestId:   req.RequestId,
Model:       modelName,
Messages:    messages,
Temperature: req.Temperature,
MaxTokens:   req.MaxTokens,
Stream:      true,
})
if err != nil {
requestsTotal.WithLabelValues(modelName, "error").Inc()
return status.Errorf(codes.Internal, "failed to start stream: %v", err)
}

var fullText string
var totalTokens int32

for {
chunk, err := grpcStream.Recv()
if err == io.EOF {
// Стриминг завершён
break
}
if err != nil {
requestsTotal.WithLabelValues(modelName, "error").Inc()
return status.Errorf(codes.Internal, "stream recv error: %v", err)
}

fullText += chunk.Text
if chunk.TokensUsed > totalTokens {
totalTokens = chunk.TokensUsed
}

// Отправляем клиенту (tail → пользователь) каждый кусок
if err := stream.Send(&gen.ChatResponseChunk{
RequestId:  req.RequestId,
Chunk:      chunk.Text,
IsFinal:    false,
Provider:   "litellm",
TokensUsed: chunk.TokensUsed,
}); err != nil {
return err
}
}

// Финальный чанк — совместимо с OpenAI
_ = stream.Send(&gen.ChatResponseChunk{
RequestId:  req.RequestId,
Chunk:      "",
IsFinal:    true,
Provider:   "litellm",
TokensUsed: totalTokens,
})

requestsTotal.WithLabelValues(modelName, "ok").Inc()
return nil
}
