package install

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/caboose-ai/caboose-ai.io/internal/config"
	"github.com/caboose-ai/caboose-ai.io/internal/runner"
	"github.com/caboose-ai/caboose-ai.io/internal/secrets"
)

type resetBackend struct{}

func (resetBackend) Name() string                   { return "test" }
func (resetBackend) Apply(context.Context) error    { return nil }
func (resetBackend) Teardown(context.Context) error { return nil }

func TestResetPreservesStaticEnvCredentials(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte(strings.Join([]string{
		"AUTHENTIK_BOOTSTRAP_TOKEN=generated",
		"GITHUB_OAUTH_CLIENT_ID=github-id",
		"GITHUB_OAUTH_CLIENT_SECRET=github-secret",
		"GOOGLE_OAUTH_CLIENT_ID=google-id",
		"GOOGLE_OAUTH_CLIENT_SECRET=google-secret",
		"TURNSTILE_SITE_KEY=turnstile-site",
		"TURNSTILE_SECRET_KEY=turnstile-secret",
		"GF_AUTH_GENERIC_OAUTH_CLIENT_ID=grafana-id",
		"HOMARR_OIDC_CLIENT_ID=homarr-id",
		"",
	}, "\n")), 0600); err != nil {
		t.Fatalf("write env: %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.Domain = "example.com"
	cfg.ComposeDir = dir
	inst := New(cfg, secrets.NewEnvFileStore(envPath), runner.NewMockRunner(), runner.NewHTTPClient())
	inst.Backend = resetBackend{}

	if err := inst.Reset(context.Background(), nil); err != nil {
		t.Fatalf("Reset: %v", err)
	}

	got, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("read env: %v", err)
	}
	text := string(got)
	for _, want := range []string{
		"GITHUB_OAUTH_CLIENT_ID=github-id",
		"GITHUB_OAUTH_CLIENT_SECRET=github-secret",
		"GOOGLE_OAUTH_CLIENT_ID=google-id",
		"GOOGLE_OAUTH_CLIENT_SECRET=google-secret",
		"TURNSTILE_SITE_KEY=turnstile-site",
		"TURNSTILE_SECRET_KEY=turnstile-secret",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("preserved env missing %q in:\n%s", want, text)
		}
	}
	for _, removed := range []string{
		"AUTHENTIK_BOOTSTRAP_TOKEN=generated",
		"GF_AUTH_GENERIC_OAUTH_CLIENT_ID=grafana-id",
		"HOMARR_OIDC_CLIENT_ID=homarr-id",
	} {
		if strings.Contains(text, removed) {
			t.Fatalf("reset env still contains %q in:\n%s", removed, text)
		}
	}
}
