package mcp

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/caboose-ai/caboose-ai.io/internal/runner"
)

func TestHandleAgentInvoke_FallbackToClaude(t *testing.T) {
	r := runner.NewMockRunner()
	r.On("ollama run", nil, errors.New("ollama unavailable"))
	r.On("claude -p", []byte("claude answer"), nil)

	s := &Server{runner: r}
	res, _, err := s.handleAgentInvoke(context.Background(), nil, agentInvokeInput{Prompt: "hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := extractText(res)
	if !strings.Contains(text, "provider: claude") {
		t.Fatalf("expected claude provider, got: %s", text)
	}
}

func TestHandleAgentInvoke_AllFail(t *testing.T) {
	r := runner.NewMockRunner()
	r.On("ollama run", nil, errors.New("nope"))
	r.On("claude -p", nil, errors.New("nope"))
	r.On("copilot chat", nil, errors.New("nope"))

	s := &Server{runner: r}
	_, _, err := s.handleAgentInvoke(context.Background(), nil, agentInvokeInput{Prompt: "hello"})
	if err == nil {
		t.Fatal("expected error")
	}
}
