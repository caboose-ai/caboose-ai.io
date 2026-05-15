package docker

import (
	"context"
	"io"
	"testing"
)

type mockRunner struct {
	output []byte
}

func (m *mockRunner) Run(context.Context, string, ...string) ([]byte, error) {
	return m.output, nil
}

func (m *mockRunner) RunWithStdin(context.Context, io.Reader, string, ...string) ([]byte, error) {
	return m.output, nil
}

func TestIsRunningUsesExactComposeServiceMatch(t *testing.T) {
	client := NewComposeClient(&mockRunner{
		output: []byte(`[{"Name":"paperclip-db","Service":"paperclip-db","State":"running"}]`),
	}, "/test")

	running, err := client.IsRunning(context.Background(), "paperclip")
	if err != nil {
		t.Fatalf("IsRunning returned error: %v", err)
	}
	if running {
		t.Fatal("IsRunning(paperclip) = true with only paperclip-db present, want false")
	}
}

func TestIsRunningMatchesServiceOrNameExactly(t *testing.T) {
	client := NewComposeClient(&mockRunner{
		output: []byte(`[
			{"Name":"project-paperclip-1","Service":"paperclip","State":"running"},
			{"Name":"legacy-service","State":"running"},
			{"Name":"stopped-service","State":"exited"}
		]`),
	}, "/test")

	for _, service := range []string{"paperclip", "legacy-service"} {
		running, err := client.IsRunning(context.Background(), service)
		if err != nil {
			t.Fatalf("IsRunning(%s) returned error: %v", service, err)
		}
		if !running {
			t.Fatalf("IsRunning(%s) = false, want true", service)
		}
	}

	running, err := client.IsRunning(context.Background(), "stopped-service")
	if err != nil {
		t.Fatalf("IsRunning(stopped-service) returned error: %v", err)
	}
	if running {
		t.Fatal("IsRunning(stopped-service) = true, want false for non-running state")
	}
}

func TestIsRunningParsesNewlineDelimitedComposeJSON(t *testing.T) {
	client := NewComposeClient(&mockRunner{
		output: []byte("{\"Name\":\"paperclip\",\"State\":\"running\"}\n{\"Name\":\"paperclip-db\",\"State\":\"running\"}\n"),
	}, "/test")

	running, err := client.IsRunning(context.Background(), "paperclip")
	if err != nil {
		t.Fatalf("IsRunning returned error: %v", err)
	}
	if !running {
		t.Fatal("IsRunning(paperclip) = false, want true")
	}
}
