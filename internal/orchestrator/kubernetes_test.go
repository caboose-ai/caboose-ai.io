package orchestrator

import (
	"context"
	"strings"
	"testing"

	"github.com/caboose-ai/caboose-ai.io/internal/config"
	"github.com/caboose-ai/caboose-ai.io/internal/runner"
)

func TestKubernetesApplyBuildsCommands(t *testing.T) {
	r := runner.NewMockRunner()
	r.On("kubectl --kubeconfig /k --context c create namespace", []byte("ok"), nil)
	r.On("kubectl --kubeconfig /k --context c apply -f -", []byte("ok"), nil)
	r.On("kubectl --kubeconfig /k --context c apply -f", []byte("ok"), nil)

	cfg := config.DefaultConfig()
	cfg.ComposeDir = t.TempDir()
	cfg.Domain = "example.com"
	cfg.Kubernetes.Kubeconfig = "/k"
	cfg.Kubernetes.Context = "c"
	b := NewKubernetesBackend(r, cfg)

	if err := b.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(r.Calls) != 3 {
		t.Fatalf("calls=%d", len(r.Calls))
	}
	if !strings.Contains(strings.Join(r.Calls[0].Args, " "), "create namespace") {
		t.Fatal("missing namespace create")
	}
}
