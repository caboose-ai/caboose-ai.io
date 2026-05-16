package docker

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type mockRunner struct {
	output  []byte
	command string
	args    []string
}

func (m *mockRunner) Run(_ context.Context, command string, args ...string) ([]byte, error) {
	m.command = command
	m.args = append([]string(nil), args...)
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

func TestDownWithVolumesIncludesComposeProfilesAndOrphans(t *testing.T) {
	dir := t.TempDir()
	composePath := filepath.Join(dir, "docker-compose.yml")
	if err := os.WriteFile(composePath, []byte(`
services:
  base:
    image: example/base
  paperclip:
    image: example/paperclip
    profiles:
      - paperclip
  worker:
    image: example/worker
    profiles:
      - extras
      - paperclip
`), 0600); err != nil {
		t.Fatalf("write compose: %v", err)
	}

	runner := &mockRunner{}
	client := NewComposeClient(runner, dir)
	if _, err := client.DownWithVolumes(context.Background()); err != nil {
		t.Fatalf("DownWithVolumes: %v", err)
	}

	if runner.command != "docker" {
		t.Fatalf("command = %q, want docker", runner.command)
	}
	want := []string{
		"compose",
		"-f", composePath,
		"--profile", "extras",
		"--profile", "paperclip",
		"down",
		"-v",
		"--remove-orphans",
	}
	if !reflect.DeepEqual(runner.args, want) {
		t.Fatalf("args = %#v, want %#v", runner.args, want)
	}
}
