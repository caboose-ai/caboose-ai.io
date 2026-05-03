package mcp

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/caboose-ai/caboose-ai.io/internal/docker"
)

type mockRunner struct {
	output   []byte
	err      error
	lastArgs []string
}

func (m *mockRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	m.lastArgs = append([]string{name}, args...)
	return m.output, m.err
}

func (m *mockRunner) RunWithStdin(_ context.Context, _ io.Reader, name string, args ...string) ([]byte, error) {
	return m.Run(context.Background(), name, args...)
}

func newTestServer(r *mockRunner) *Server {
	return &Server{
		compose: docker.NewComposeClient(r, "/test/homelab"),
		runner:  r,
	}
}

func TestHandleDockerPS(t *testing.T) {
	r := &mockRunner{output: []byte(`[{"Name":"forgejo","State":"running"}]`)}
	s := newTestServer(r)

	result, _, err := s.handleDockerPS(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(extractText(result), "forgejo") {
		t.Errorf("expected output to contain 'forgejo', got %q", extractText(result))
	}
}

func TestHandleDockerPS_Error(t *testing.T) {
	r := &mockRunner{err: fmt.Errorf("docker not running")}
	s := newTestServer(r)

	_, _, err := s.handleDockerPS(context.Background(), nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "docker_ps") {
		t.Errorf("error should be wrapped with tool name, got %q", err.Error())
	}
}

func TestHandleDockerUp(t *testing.T) {
	r := &mockRunner{output: []byte("started")}
	s := newTestServer(r)

	result, _, err := s.handleDockerUp(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(extractText(result), "started successfully") {
		t.Errorf("unexpected result: %q", extractText(result))
	}
}

func TestHandleDockerDown(t *testing.T) {
	r := &mockRunner{output: []byte("stopped")}
	s := newTestServer(r)

	result, _, err := s.handleDockerDown(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(extractText(result), "stopped successfully") {
		t.Errorf("unexpected result: %q", extractText(result))
	}
}

func TestHandleDockerRestart(t *testing.T) {
	r := &mockRunner{output: []byte("restarted")}
	s := newTestServer(r)

	input := dockerRestartInput{Services: []string{"forgejo", "grafana"}}
	result, _, err := s.handleDockerRestart(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(extractText(result), "Restarted") {
		t.Errorf("unexpected result: %q", extractText(result))
	}
}

func TestHandleDockerRestart_NoServices(t *testing.T) {
	r := &mockRunner{}
	s := newTestServer(r)

	_, _, err := s.handleDockerRestart(context.Background(), nil, dockerRestartInput{})
	if err == nil {
		t.Fatal("expected error for empty services")
	}
}

func TestHandleDockerLogs(t *testing.T) {
	r := &mockRunner{output: []byte("log line 1\nlog line 2")}
	s := newTestServer(r)

	input := dockerLogsInput{Service: "forgejo", Tail: 50}
	result, _, err := s.handleDockerLogs(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(extractText(result), "log line 1") {
		t.Errorf("unexpected result: %q", extractText(result))
	}
}

func TestHandleDockerLogs_DefaultTail(t *testing.T) {
	r := &mockRunner{output: []byte("logs")}
	s := newTestServer(r)

	input := dockerLogsInput{Service: "forgejo"}
	_, _, err := s.handleDockerLogs(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	args := strings.Join(r.lastArgs, " ")
	if !strings.Contains(args, "100") {
		t.Errorf("expected default tail of 100, got args: %q", args)
	}
}

func TestHandleDockerLogs_NoService(t *testing.T) {
	r := &mockRunner{}
	s := newTestServer(r)

	_, _, err := s.handleDockerLogs(context.Background(), nil, dockerLogsInput{})
	if err == nil {
		t.Fatal("expected error for empty service")
	}
}
