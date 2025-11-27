package providers

// Minimal manager for demo; use HTTP clients for OpenAI or call model-proxy gRPC.

import (
	"context"
	"errors"
)

type Message struct {
	Role string `json:"role"`
	Content string `json:"content"`
}

type Manager struct {}

func NewManager() *Manager { return &Manager{} }

func (m *Manager) Call(ctx context.Context, model string, msgs []Message, temperature float32, maxTokens int, stream bool) (string, string, int, error) {
	// For demo return echo
	out := ""
	for _, mm := range msgs { out += mm.Content + " " }
	if out=="" { out = "empty" }
	return "local", out, len(out)/4 + 1, nil
}
