package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/yourorg/head/gen"
	"github.com/yourorg/head/internal/config"
	"github.com/yourorg/head/internal/metrics"
	"github.com/yourorg/head/internal/providers"

	"google.golang.org/grpc"
)

type Server struct {
	cfg *config.Config
	pm *providers.Manager
	grpcSrv *grpc.Server
}

func New(cfg *config.Config) *Server {
	return &Server{cfg:cfg, pm: providers.NewManager()}
}

func (s *Server) Run() error {
	s.grpcSrv = grpc.NewServer()
	gen.RegisterChatServiceServer(s.grpcSrv, s)
	lis, err := net.Listen("tcp", s.cfg.GRPCAddr)
	if err != nil { return err }
	log.Printf("gRPC head listening %s", s.cfg.GRPCAddr)
	return s.grpcSrv.Serve(lis)
}

func (s *Server) Shutdown(ctx context.Context) error {
	done := make(chan struct{})
	go func(){ s.grpcSrv.GracefulStop(); close(done) }()
	select {
	case <-ctx.Done():
		s.grpcSrv.Stop()
		return ctx.Err()
	case <-done:
		return nil
	}
}

// ChatCompletion simple impl
func (s *Server) ChatCompletion(ctx context.Context, req *gen.ChatRequest) (*gen.ChatResponse, error) {
	model := req.Model
	start := time.Now()
	metrics.Requests.WithLabelValues(model,"received").Inc()

	// basic cache key
	b, _ := json.Marshal(req)
	sum := sha256.Sum256(b)
	key := "head:cache:" + hex.EncodeToString(sum[:])

	// call provider
	msgs := make([]providers.Message,0,len(req.Messages))
	for _,m := range req.Messages { msgs = append(msgs, providers.Message{Role:m.Role, Content:m.Content}) }
	prov, text, tokens, err := s.pm.Call(ctx, model, msgs, float32(req.Temperature), int(req.MaxTokens), false)
	if err!=nil { metrics.Requests.WithLabelValues(model,"error").Inc(); return nil, err }

	metrics.Requests.WithLabelValues(model,"ok").Inc()
	metrics.Latency.WithLabelValues(model).Observe(time.Since(start).Seconds())

	return &gen.ChatResponse{ RequestId: req.RequestId, FullText: text, Model: model, Provider: prov, TokensUsed: int32(tokens)}, nil
}

func (s *Server) ChatCompletionStream(req *gen.ChatRequest, stream gen.ChatService_ChatCompletionStreamServer) error {
	prov, text, tokens, err := s.pm.Call(stream.Context(), req.Model, []providers.Message{}, float32(req.Temperature), int(req.MaxTokens), true)
	if err!=nil { return err }
	_ = prov; _ = tokens
	// single chunk
	return stream.Send(&gen.ChatResponseChunk{ RequestId: req.RequestId, Chunk: text, IsFinal:true, Provider:prov, TokensUsed:int32(tokens)})
}
