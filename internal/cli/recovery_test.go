package cli

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/caboose-ai/caboose-ai.io/internal/docker"
	"github.com/caboose-ai/caboose-ai.io/internal/runner"
)

func TestParseRecoveryToken(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   string
	}{
		{
			name:   "standard output",
			output: "/recovery/use-token/SzDx5q3Ga8AY3BEHnyFv9qrZZKp1cvyJvDVf3QyElltTZHnkHifKgLaXdO4v/",
			want:   "SzDx5q3Ga8AY3BEHnyFv9qrZZKp1cvyJvDVf3QyElltTZHnkHifKgLaXdO4v",
		},
		{
			name:   "with prefix text",
			output: "Successfully created recovery link /recovery/use-token/abc123/",
			want:   "abc123",
		},
		{
			name:   "no trailing slash",
			output: "/recovery/use-token/tokenvalue",
			want:   "tokenvalue",
		},
		{
			name:   "empty output",
			output: "",
			want:   "",
		},
		{
			name:   "unrelated output",
			output: "some error happened",
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseRecoveryToken(tt.output)
			if got != tt.want {
				t.Errorf("parseRecoveryToken(%q) = %q, want %q", tt.output, got, tt.want)
			}
		})
	}
}

func TestRunRecoveryRequiresRunningAuthentikContainer(t *testing.T) {
	r := runner.NewMockRunner()
	r.On("docker inspect --format {{.State.Running}} authentik-server", []byte("false\n"), nil)

	_, err := RunRecovery(context.Background(), docker.NewExecClient(r), RecoveryOptions{
		Domain:     "example.com",
		Username:   "auth-admin",
		Minutes:    10,
		ComposeDir: "/opt/homelab",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "authentik container \"authentik-server\" is not running") {
		t.Fatalf("error = %q, want container-not-running message", err.Error())
	}
	if !strings.Contains(err.Error(), "docker compose -f /opt/homelab/docker-compose.yml up -d authentik-server") {
		t.Fatalf("error = %q, want compose start hint", err.Error())
	}
	for _, call := range r.Calls {
		if call.Name == "docker" && strings.Join(call.Args, " ") == "exec authentik-server ak create_recovery_key 10 auth-admin" {
			t.Fatalf("RunRecovery should not exec when authentik-server is stopped")
		}
	}
}

func TestRunRecoveryReportsMissingAuthentikContainerAsStopped(t *testing.T) {
	r := runner.NewMockRunner()
	r.On("docker inspect --format {{.State.Running}} authentik-server", nil, fmt.Errorf("docker: exit status 1\nError: No such object: authentik-server"))

	_, err := RunRecovery(context.Background(), docker.NewExecClient(r), RecoveryOptions{
		Domain:   "example.com",
		Username: "auth-admin",
		Minutes:  10,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "authentik container \"authentik-server\" is not running") {
		t.Fatalf("error = %q, want missing container to be reported as not running", err.Error())
	}
}
