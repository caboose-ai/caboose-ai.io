package cli

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/caboose-ai/caboose-ai.io/internal/config"
	"github.com/caboose-ai/caboose-ai.io/internal/docker"
	"github.com/caboose-ai/caboose-ai.io/internal/runner"
	"github.com/caboose-ai/caboose-ai.io/internal/service"
)

func TestRunServiceStatusUsesHealthURLForExternalRuntimeWithoutCompose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" {
			t.Fatalf("path = %q, want /healthz", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	mockRunner := runner.NewMockRunner()
	manifest := service.Manifest{
		Slug:        "external-app",
		DisplayName: "External App",
		Runtime:     service.RuntimeExternal,
		URLKey:      "openclaw",
		Health: service.HealthSpec{
			URLKey: "openclaw",
			Path:   "/healthz",
			Kind:   "http",
		},
	}

	exitCode, stdout, stderr := captureServiceCommandOutput(t, func() int {
		return runServiceStatus(context.Background(), docker.NewComposeClient(mockRunner, "dev/homelab"), http.DefaultClient, config.URLs{OpenClaw: server.URL}, manifest)
	})

	if exitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", exitCode, stderr)
	}
	if len(mockRunner.Calls) != 0 {
		t.Fatalf("compose runner calls = %v, want none", mockRunner.Calls)
	}
	for _, want := range []string{`"runtime": "external"`, `"health_url": "` + server.URL + `/healthz"`, `"healthy": true`} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("stdout = %q, missing %q", stdout, want)
		}
	}
}

func TestRunServiceLogsReturnsClearMessageForExternalRuntimeWithoutCompose(t *testing.T) {
	mockRunner := runner.NewMockRunner()
	manifest := service.Manifest{
		Slug:        "external-app",
		DisplayName: "External App",
		Runtime:     service.RuntimeExternal,
	}

	exitCode, stdout, stderr := captureServiceCommandOutput(t, func() int {
		return runServiceLogs(context.Background(), docker.NewComposeClient(mockRunner, "dev/homelab"), manifest)
	})

	if exitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", exitCode, stderr)
	}
	if stdout == "" || !strings.Contains(stdout, `service "external-app" uses external runtime; no compose logs are available`) {
		t.Fatalf("stdout = %q, want external no compose logs message", stdout)
	}
	if len(mockRunner.Calls) != 0 {
		t.Fatalf("compose runner calls = %v, want none", mockRunner.Calls)
	}
}

func TestRunServiceSmokeUsesManifestSmokeFlow(t *testing.T) {
	r := &recordingRunner{}
	code := runServiceSmoke(context.Background(), r, service.Manifest{
		Slug:      "paperclip",
		SmokeFlow: "paperclip",
	})
	if code != 0 {
		t.Fatalf("runServiceSmoke exit = %d, want 0", code)
	}

	want := []string{
		"test",
		"-tags", "integration",
		"./internal/smoketest/",
		"-run", "TestSSO_ServiceSmoke",
		"-v",
		"-args",
		"-smoke-flow", "paperclip",
	}
	if r.name != "go" || !reflect.DeepEqual(r.args, want) {
		t.Fatalf("runner command = %s %v, want go %v", r.name, r.args, want)
	}
}

func TestServiceURLIncludesPaperclip(t *testing.T) {
	urls := config.DeriveURLs("example.com")
	if got := serviceURL(urls, "paperclip"); got != "https://paperclip.example.com" {
		t.Fatalf("serviceURL(paperclip) = %q", got)
	}
}

type recordingRunner struct {
	name string
	args []string
}

func (r *recordingRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	r.name = name
	r.args = append([]string{}, args...)
	return []byte("ok\n"), nil
}

func (r *recordingRunner) RunWithStdin(ctx context.Context, _ io.Reader, name string, args ...string) ([]byte, error) {
	return r.Run(ctx, name, args...)
}

func captureServiceCommandOutput(t *testing.T, fn func() int) (int, string, string) {
	t.Helper()

	oldStdout := os.Stdout
	oldStderr := os.Stderr
	stdoutRead, stdoutWrite, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating stdout pipe: %v", err)
	}
	stderrRead, stderrWrite, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating stderr pipe: %v", err)
	}
	os.Stdout = stdoutWrite
	os.Stderr = stderrWrite
	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	}()

	exitCode := fn()
	stdoutWrite.Close()
	stderrWrite.Close()

	var stdout, stderr bytes.Buffer
	_, _ = io.Copy(&stdout, stdoutRead)
	_, _ = io.Copy(&stderr, stderrRead)
	return exitCode, stdout.String(), stderr.String()
}
