package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/caboose-ai/caboose-ai.io/internal/config"
	"github.com/caboose-ai/caboose-ai.io/internal/install"
	"github.com/caboose-ai/caboose-ai.io/internal/runner"
	"github.com/caboose-ai/caboose-ai.io/internal/secrets"
)

type installTestSecretStore struct{}

func (installTestSecretStore) Get(context.Context, string) (string, error) { return "", nil }
func (installTestSecretStore) Put(context.Context, string, string) error   { return nil }
func (installTestSecretStore) Generate(context.Context, string, int, secrets.GenerateOpts) (string, error) {
	return "generated", nil
}
func (installTestSecretStore) Delete(context.Context, string) error { return nil }
func (installTestSecretStore) EnsureVault(context.Context) error    { return nil }

func TestRunInstallDryRunLeavesPaperclipExampleManual(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Domain = "example.test"
	cfg.DryRun = true
	cfg.SecretsEnvOnly = true

	mockRunner := runner.NewMockRunner()
	mockRunner.On("docker --version", []byte("Docker version 1\n"), nil)
	mockRunner.On("docker compose version", []byte("Docker Compose version 2\n"), nil)
	mockRunner.On("op --version", []byte("2.0.0\n"), nil)

	inst := install.New(cfg, installTestSecretStore{}, mockRunner, nil)
	exitCode, _, stderr := captureServiceCommandOutput(t, func() int {
		return RunInstall(context.Background(), inst)
	})

	if exitCode != 0 {
		t.Fatalf("RunInstall exit code = %d, want 0\n%s", exitCode, stderr)
	}
	if strings.Contains(stderr, "would start and seed Paperclip") {
		t.Fatalf("dry-run output still auto-starts Paperclip: %s", stderr)
	}
	if !strings.Contains(stderr, "would leave Paperclip example stopped until requested with `mise run paperclip:example`") {
		t.Fatalf("dry-run output missing manual Paperclip example hint: %s", stderr)
	}
}
