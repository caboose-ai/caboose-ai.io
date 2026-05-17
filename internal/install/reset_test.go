package install

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/caboose-ai/caboose-ai.io/internal/config"
	"github.com/caboose-ai/caboose-ai.io/internal/runner"
	"github.com/caboose-ai/caboose-ai.io/internal/secrets"
)

type resetBackend struct {
	teardownCalled bool
}

func (resetBackend) Name() string                { return "test" }
func (resetBackend) Apply(context.Context) error { return nil }
func (b *resetBackend) Teardown(context.Context) error {
	b.teardownCalled = true
	return nil
}

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
	inst.Backend = &resetBackend{}

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

func TestResetStoresStaticEnvBeforeRemovingEnv(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte(strings.Join([]string{
		"AUTHENTIK_BOOTSTRAP_TOKEN=generated",
		"GITHUB_OAUTH_CLIENT_ID=github-id",
		"GITHUB_OAUTH_CLIENT_SECRET=github-secret",
		"CLOUDFLARE_ZONE_ID=zone-id",
		"",
	}, "\n")), 0600); err != nil {
		t.Fatalf("write env: %v", err)
	}

	store := &recordingSecretStore{values: map[string]string{}}
	cfg := config.DefaultConfig()
	cfg.Domain = "example.com"
	cfg.ComposeDir = dir
	inst := New(cfg, store, runner.NewMockRunner(), runner.NewHTTPClient())
	inst.Backend = &resetBackend{}
	inst.State.StoreStaticEnv = true

	if err := inst.Reset(context.Background(), nil); err != nil {
		t.Fatalf("Reset: %v", err)
	}

	if _, err := os.Stat(envPath); !os.IsNotExist(err) {
		t.Fatalf("expected env file to be removed, stat err=%v", err)
	}
	for key, want := range map[string]string{
		"GITHUB_OAUTH_CLIENT_ID":     "github-id",
		"GITHUB_OAUTH_CLIENT_SECRET": "github-secret",
		"CLOUDFLARE_ZONE_ID":         "zone-id",
	} {
		if got := store.values[key]; got != want {
			t.Fatalf("stored %s = %q, want %q", key, got, want)
		}
	}
	if _, ok := store.values["AUTHENTIK_BOOTSTRAP_TOKEN"]; ok {
		t.Fatalf("stored dynamic bootstrap token as static secret")
	}
}

func TestResetStoresStaticEnvBeforeTeardown(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte("GITHUB_OAUTH_CLIENT_ID=github-id\n"), 0600); err != nil {
		t.Fatalf("write env: %v", err)
	}

	store := &recordingSecretStore{
		values: map[string]string{},
		putErr: fmt.Errorf("op is not signed in"),
	}
	backend := &resetBackend{}
	cfg := config.DefaultConfig()
	cfg.Domain = "example.com"
	cfg.ComposeDir = dir
	inst := New(cfg, store, runner.NewMockRunner(), runner.NewHTTPClient())
	inst.Backend = backend
	inst.State.StoreStaticEnv = true

	if err := inst.Reset(context.Background(), nil); err == nil {
		t.Fatalf("Reset returned nil, want static secret store error")
	}
	if backend.teardownCalled {
		t.Fatalf("teardown ran before static env values were safely stored")
	}
	if _, err := os.Stat(envPath); err != nil {
		t.Fatalf("env file was touched after failed static store: %v", err)
	}
}

func TestResetEnsuresVaultBeforeTeardownForStaticEnvStore(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte("GITHUB_OAUTH_CLIENT_ID=github-id\n"), 0600); err != nil {
		t.Fatalf("write env: %v", err)
	}

	store := &recordingSecretStore{
		values:    map[string]string{},
		ensureErr: fmt.Errorf("op is not signed in"),
	}
	backend := &resetBackend{}
	cfg := config.DefaultConfig()
	cfg.Domain = "example.com"
	cfg.ComposeDir = dir
	inst := New(cfg, store, runner.NewMockRunner(), runner.NewHTTPClient())
	inst.Backend = backend
	inst.State.StoreStaticEnv = true

	if err := inst.Reset(context.Background(), nil); err == nil {
		t.Fatalf("Reset returned nil, want vault ensure error")
	}
	if backend.teardownCalled {
		t.Fatalf("teardown ran before vault access was verified")
	}
}

func TestRestoreStaticEnvFromSecretStoreWritesEnvFile(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	store := &recordingSecretStore{values: map[string]string{
		"GITHUB_OAUTH_CLIENT_ID":     "github-id",
		"GITHUB_OAUTH_CLIENT_SECRET": "github-secret",
		"TURNSTILE_SITE_KEY":         "turnstile-site",
		"AUTHENTIK_BOOTSTRAP_TOKEN":  "dynamic-token",
	}}

	cfg := config.DefaultConfig()
	cfg.Domain = "example.com"
	cfg.ComposeDir = dir
	inst := New(cfg, store, runner.NewMockRunner(), runner.NewHTTPClient())

	if err := inst.RestoreStaticEnv(context.Background()); err != nil {
		t.Fatalf("RestoreStaticEnv: %v", err)
	}

	got, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("read env: %v", err)
	}
	text := string(got)
	for _, want := range []string{
		"GITHUB_OAUTH_CLIENT_ID=github-id",
		"GITHUB_OAUTH_CLIENT_SECRET=github-secret",
		"TURNSTILE_SITE_KEY=turnstile-site",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("restored env missing %q in:\n%s", want, text)
		}
	}
	if strings.Contains(text, "AUTHENTIK_BOOTSTRAP_TOKEN") {
		t.Fatalf("restored dynamic secret into static env:\n%s", text)
	}
}

type recordingSecretStore struct {
	values    map[string]string
	putErr    error
	ensureErr error
}

func (s *recordingSecretStore) Get(_ context.Context, key string) (string, error) {
	return s.values[key], nil
}

func (s *recordingSecretStore) Put(_ context.Context, key, value string) error {
	if s.putErr != nil {
		return s.putErr
	}
	s.values[key] = value
	return nil
}

func (s *recordingSecretStore) Generate(context.Context, string, int, secrets.GenerateOpts) (string, error) {
	return "", nil
}

func (s *recordingSecretStore) Delete(context.Context, string) error {
	return nil
}

func (s *recordingSecretStore) EnsureVault(context.Context) error {
	if s.ensureErr != nil {
		return s.ensureErr
	}
	return nil
}
