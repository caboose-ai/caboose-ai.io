package mcp

import (
	"context"
	"errors"
	"io"
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

func TestInvokeProvider_Emberfall(t *testing.T) {
	r := &captureRunner{}

	s := &Server{runner: r}
	prompt := "tests:\n- url: https://example.com\n  method: GET\n  expect:\n    status: 200\n"
	out, err := s.invokeProvider(context.Background(), "emberfall", "", prompt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(out) != "ok" {
		t.Fatalf("unexpected output: %s", string(out))
	}
	if r.name != "emberfall" {
		t.Fatalf("unexpected command: %s", r.name)
	}
	if len(r.args) != 2 || r.args[0] != "--tests" || r.args[1] != "-" {
		t.Fatalf("unexpected args: %#v", r.args)
	}
	if r.stdin != prompt {
		t.Fatalf("unexpected stdin content: %q", r.stdin)
	}
}

type captureRunner struct {
	name  string
	args  []string
	stdin string
}

func (r *captureRunner) Run(context.Context, string, ...string) ([]byte, error) {
	return nil, errors.New("Run should not be called")
}

func (r *captureRunner) RunWithStdin(_ context.Context, stdin io.Reader, name string, args ...string) ([]byte, error) {
	body, err := io.ReadAll(stdin)
	if err != nil {
		return nil, err
	}
	r.name = name
	r.args = args
	r.stdin = string(body)
	return []byte("ok"), nil
}
