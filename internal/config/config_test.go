package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDeriveURLs(t *testing.T) {
	urls := DeriveURLs("example.com")

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"Authentik", urls.Authentik, "https://auth.example.com"},
		{"Forgejo", urls.Forgejo, "https://git.example.com"},
		{"Woodpecker", urls.Woodpecker, "https://ci.example.com"},
		{"Portainer", urls.Portainer, "https://docker.example.com"},
		{"Grafana", urls.Grafana, "https://grafana.example.com"},
		{"OpenWebUI", urls.OpenWebUI, "https://ai.example.com"},
		{"Mattermost", urls.Mattermost, "https://chat.example.com"},
		{"Dashboard", urls.Dashboard, "https://example.com"},
		{"DashAlias", urls.DashAlias, "https://dash.example.com"},
		{"OpenClaw", urls.OpenClaw, "https://openclaw.example.com"},
		{"SonarQube", urls.SonarQube, "https://sonar.example.com"},
		{"Ghost", urls.Ghost, "https://blog.example.com"},
		{"Paperclip", urls.Paperclip, "https://paperclip.example.com"},
		{"CI", urls.CI, "https://ci.example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %q, want %q", tt.got, tt.want)
			}
		})
	}
}

func TestServiceLinksIncludeOpenClaw(t *testing.T) {
	urls := DeriveURLs("example.com")

	var found bool
	for _, link := range urls.ServiceLinks() {
		if link.Name == "OpenClaw" {
			found = true
			if link.URL != "https://openclaw.example.com" {
				t.Fatalf("OpenClaw URL = %q", link.URL)
			}
		}
	}
	if !found {
		t.Fatal("OpenClaw missing from service links")
	}
}

func TestServiceLinksIncludePaperclip(t *testing.T) {
	urls := DeriveURLs("example.com")

	var found bool
	for _, link := range urls.ServiceLinks() {
		if link.Name == "Paperclip" {
			found = true
			if link.URL != "https://paperclip.example.com" {
				t.Fatalf("Paperclip URL = %q", link.URL)
			}
		}
	}
	if !found {
		t.Fatal("Paperclip missing from service links")
	}
}

func TestURLsLookupByKey(t *testing.T) {
	urls := DeriveURLs("example.com")

	tests := []struct {
		key  string
		want string
	}{
		{"authentik", "https://auth.example.com"},
		{"forgejo", "https://git.example.com"},
		{"open_webui", "https://ai.example.com"},
		{"open-webui", "https://ai.example.com"},
		{"sonarqube", "https://sonar.example.com"},
		{"dashboard", "https://example.com"},
		{"dash_alias", "https://dash.example.com"},
		{"paperclip", "https://paperclip.example.com"},
		{"ci", "https://ci.example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got, ok := urls.Lookup(tt.key)
			if !ok {
				t.Fatalf("Lookup(%q) returned ok=false", tt.key)
			}
			if got != tt.want {
				t.Fatalf("Lookup(%q) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}

func TestURLsLookupUnknownKey(t *testing.T) {
	urls := DeriveURLs("example.com")

	got, ok := urls.Lookup("missing")
	if ok {
		t.Fatalf("Lookup(missing) returned ok=true with %q", got)
	}
	if got != "" {
		t.Fatalf("Lookup(missing) = %q, want empty string", got)
	}
}

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	os.WriteFile(path, []byte(`
domain: test.example.com
email: admin@test.example.com
compose_dir: /opt/homelab
op_vault: TestVault
op_static_vault: StaticVault
orchestrator: kubernetes
serve_mode: local
kubernetes:
  namespace: lab
  kubeconfig: /tmp/kubeconfig
  context: prod
social:
  github:
    client_id: gh-id
    client_secret: gh-secret
`), 0644)

	cfg, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("LoadFromFile: %v", err)
	}

	if cfg.Domain != "test.example.com" {
		t.Errorf("Domain = %q, want %q", cfg.Domain, "test.example.com")
	}
	if cfg.Email != "admin@test.example.com" {
		t.Errorf("Email = %q", cfg.Email)
	}
	if cfg.ComposeDir != "/opt/homelab" {
		t.Errorf("ComposeDir = %q", cfg.ComposeDir)
	}
	if cfg.OPVault != "TestVault" {
		t.Errorf("OPVault = %q", cfg.OPVault)
	}
	if cfg.OPStaticVault != "StaticVault" {
		t.Errorf("OPStaticVault = %q", cfg.OPStaticVault)
	}
	if cfg.Social.GitHub == nil || cfg.Social.GitHub.ClientID != "gh-id" {
		t.Errorf("Social.GitHub = %+v", cfg.Social.GitHub)
	}
	if cfg.Orchestrator != "kubernetes" || cfg.Kubernetes.Namespace != "lab" {
		t.Errorf("kubernetes config not loaded: %+v", cfg.Kubernetes)
	}
	if cfg.ServeMode != "local" {
		t.Errorf("ServeMode = %q", cfg.ServeMode)
	}
}

func TestLoadWithOverridesAppliesCLIValuesAfterFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	os.WriteFile(path, []byte(`
domain: file.example.com
email: admin@file.example.com
compose_dir: /srv/file
op_vault: FileDynamic
op_static_vault: FileStatic
orchestrator: compose
serve_mode: public
kubernetes:
  namespace: file-ns
  kubeconfig: /tmp/file-kubeconfig
  context: file-context
`), 0644)

	cfg, err := LoadWithOverrides(path, Overrides{
		Domain:         "cli.example.com",
		Email:          "admin@cli.example.com",
		ComposeDir:     "/srv/cli",
		OPVault:        "CliDynamic",
		OPStaticVault:  "CliStatic",
		Orchestrator:   "kubernetes",
		ServeMode:      "local",
		KubeNamespace:  "cli-ns",
		Kubeconfig:     "/tmp/cli-kubeconfig",
		KubeContext:    "cli-context",
		DryRun:         true,
		Force:          true,
		Verbose:        true,
		NonInteractive: true,
		SecretsEnvOnly: true,
	})
	if err != nil {
		t.Fatalf("LoadWithOverrides: %v", err)
	}

	if cfg.Domain != "cli.example.com" {
		t.Errorf("Domain = %q", cfg.Domain)
	}
	if cfg.Email != "admin@cli.example.com" {
		t.Errorf("Email = %q", cfg.Email)
	}
	if cfg.ComposeDir != "/srv/cli" {
		t.Errorf("ComposeDir = %q", cfg.ComposeDir)
	}
	if cfg.OPVault != "CliDynamic" {
		t.Errorf("OPVault = %q", cfg.OPVault)
	}
	if cfg.OPStaticVault != "CliStatic" {
		t.Errorf("OPStaticVault = %q", cfg.OPStaticVault)
	}
	if cfg.Orchestrator != "kubernetes" {
		t.Errorf("Orchestrator = %q", cfg.Orchestrator)
	}
	if cfg.ServeMode != "local" {
		t.Errorf("ServeMode = %q", cfg.ServeMode)
	}
	if cfg.Kubernetes.Namespace != "cli-ns" {
		t.Errorf("Kubernetes.Namespace = %q", cfg.Kubernetes.Namespace)
	}
	if cfg.Kubernetes.Kubeconfig != "/tmp/cli-kubeconfig" {
		t.Errorf("Kubernetes.Kubeconfig = %q", cfg.Kubernetes.Kubeconfig)
	}
	if cfg.Kubernetes.Context != "cli-context" {
		t.Errorf("Kubernetes.Context = %q", cfg.Kubernetes.Context)
	}
	if !cfg.DryRun || !cfg.Force || !cfg.Verbose || !cfg.NonInteractive || !cfg.SecretsEnvOnly {
		t.Errorf("runtime flags not applied: %+v", cfg)
	}
}

func TestLoadWithOverridesKeepsFileAndDefaultValuesForEmptyStrings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	os.WriteFile(path, []byte(`
domain: file.example.com
compose_dir: /srv/file
kubernetes:
  namespace: file-ns
`), 0644)

	cfg, err := LoadWithOverrides(path, Overrides{
		Domain:        "",
		ComposeDir:    "",
		KubeNamespace: "",
	})
	if err != nil {
		t.Fatalf("LoadWithOverrides: %v", err)
	}

	if cfg.Domain != "file.example.com" {
		t.Errorf("Domain = %q", cfg.Domain)
	}
	if cfg.ComposeDir != "/srv/file" {
		t.Errorf("ComposeDir = %q", cfg.ComposeDir)
	}
	if cfg.Kubernetes.Namespace != "file-ns" {
		t.Errorf("Kubernetes.Namespace = %q", cfg.Kubernetes.Namespace)
	}
	if cfg.Email != "cxm6467@gmail.com" {
		t.Errorf("default Email = %q", cfg.Email)
	}
}

func TestLoadWithOverridesUsesDefaultsWhenPathEmpty(t *testing.T) {
	cfg, err := LoadWithOverrides("", Overrides{
		Domain:         "cli.example.com",
		ComposeDir:     "/srv/cli",
		SecretsEnvOnly: true,
	})
	if err != nil {
		t.Fatalf("LoadWithOverrides: %v", err)
	}

	if cfg.Domain != "cli.example.com" {
		t.Errorf("Domain = %q", cfg.Domain)
	}
	if cfg.ComposeDir != "/srv/cli" {
		t.Errorf("ComposeDir = %q", cfg.ComposeDir)
	}
	if cfg.Orchestrator != "compose" {
		t.Errorf("default Orchestrator = %q", cfg.Orchestrator)
	}
	if !cfg.SecretsEnvOnly {
		t.Error("SecretsEnvOnly was not applied")
	}
}

func TestValidate(t *testing.T) {
	cfg := DefaultConfig()
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for empty domain")
	}

	cfg.Domain = "example.com"
	cfg.Orchestrator = "bad"
	if err := cfg.Validate(); err == nil {
		t.Errorf("expected orchestrator validation error")
	}

	cfg.Orchestrator = "kubernetes"
	if err := cfg.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	cfg.ServeMode = "bad"
	if err := cfg.Validate(); err == nil {
		t.Errorf("expected serve_mode validation error")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.OPVault != "Homelab - Dynamic" {
		t.Errorf("default OPVault = %q, want Homelab - Dynamic", cfg.OPVault)
	}
	if cfg.OPStaticVault != "Homelab - Static" {
		t.Errorf("default OPStaticVault = %q, want Homelab - Static", cfg.OPStaticVault)
	}
	if cfg.ComposeDir != "dev/homelab" {
		t.Errorf("default ComposeDir = %q, want dev/homelab", cfg.ComposeDir)
	}
	if cfg.Orchestrator != "compose" {
		t.Errorf("default Orchestrator = %q", cfg.Orchestrator)
	}
	if cfg.ServeMode != "public" {
		t.Errorf("default ServeMode = %q", cfg.ServeMode)
	}
	if cfg.BindAddress() != "127.0.0.1" {
		t.Errorf("default BindAddress = %q", cfg.BindAddress())
	}
	cfg.ServeMode = "local"
	if cfg.BindAddress() != "0.0.0.0" {
		t.Errorf("local BindAddress = %q", cfg.BindAddress())
	}
	if cfg.Kubernetes.Namespace != "homelab" {
		t.Errorf("default Kubernetes Namespace = %q", cfg.Kubernetes.Namespace)
	}
}
