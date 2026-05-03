package orchestrator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/caboose-ai/caboose-ai.io/internal/config"
	"github.com/caboose-ai/caboose-ai.io/internal/runner"
)

type KubernetesBackend struct {
	runner      runner.CommandRunner
	cfg         *config.Config
	manifestDir string
}

func NewKubernetesBackend(r runner.CommandRunner, cfg *config.Config) *KubernetesBackend {
	return &KubernetesBackend{runner: r, cfg: cfg, manifestDir: filepath.Join(cfg.ComposeDir, "rendered", "k8s")}
}
func (b *KubernetesBackend) Name() string { return "kubernetes" }

func (b *KubernetesBackend) kubectlArgs(args ...string) []string {
	prefix := []string{}
	if b.cfg.Kubernetes.Kubeconfig != "" { prefix = append(prefix, "--kubeconfig", b.cfg.Kubernetes.Kubeconfig) }
	if b.cfg.Kubernetes.Context != "" { prefix = append(prefix, "--context", b.cfg.Kubernetes.Context) }
	return append(prefix, args...)
}

func (b *KubernetesBackend) renderManifest() error {
	if err := os.MkdirAll(b.manifestDir, 0755); err != nil { return err }
	urls := b.cfg.URLs()
	manifest := fmt.Sprintf("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: homelab-config\n  namespace: %s\n  labels:\n    app.kubernetes.io/part-of: homelab\n    app.kubernetes.io/managed-by: caboose-homelab\ndata:\n  domain: %s\n  dashboard: %s\n", b.cfg.Kubernetes.Namespace, b.cfg.Domain, urls.Dashboard)
	return os.WriteFile(filepath.Join(b.manifestDir, "homelab-config.yaml"), []byte(manifest), 0644)
}

func (b *KubernetesBackend) Apply(ctx context.Context) error {
	if err := b.renderManifest(); err != nil { return err }
	if _, err := b.runner.Run(ctx, "kubectl", b.kubectlArgs("create", "namespace", b.cfg.Kubernetes.Namespace, "--dry-run=client", "-o", "yaml")...); err != nil { return err }
	if _, err := b.runner.Run(ctx, "kubectl", b.kubectlArgs("apply", "-f", "-")...); err != nil { return err }
	if _, err := b.runner.Run(ctx, "kubectl", b.kubectlArgs("apply", "-f", b.manifestDir)...); err != nil { return err }
	return nil
}

func (b *KubernetesBackend) Teardown(ctx context.Context) error {
	_, err := b.runner.Run(ctx, "kubectl", b.kubectlArgs("delete", "all,cm,secret,ingress", "-n", b.cfg.Kubernetes.Namespace, "-l", "app.kubernetes.io/managed-by=caboose-homelab", "--ignore-not-found")...)
	return err
}
