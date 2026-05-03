package install

import (
	"testing"

	"github.com/caboose-ai/caboose-ai.io/internal/config"
	"github.com/caboose-ai/caboose-ai.io/internal/runner"
	"github.com/caboose-ai/caboose-ai.io/internal/secrets"
)

func TestBackendSelection(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Domain = "example.com"
	inst := New(cfg, secrets.NewEnvFileStore("/tmp/.env"), runner.NewMockRunner(), runner.NewHTTPClient())
	if inst.Backend.Name() != "compose" { t.Fatalf("got %s", inst.Backend.Name()) }

	cfg2 := config.DefaultConfig(); cfg2.Domain = "example.com"; cfg2.Orchestrator = "kubernetes"
	inst2 := New(cfg2, secrets.NewEnvFileStore("/tmp/.env2"), runner.NewMockRunner(), runner.NewHTTPClient())
	if inst2.Backend.Name() != "kubernetes" { t.Fatalf("got %s", inst2.Backend.Name()) }
}
