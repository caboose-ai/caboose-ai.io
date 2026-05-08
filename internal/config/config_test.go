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
		{"N8N", urls.N8N, "https://n8n.example.com"},
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

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	os.WriteFile(path, []byte(`
domain: test.example.com
email: admin@test.example.com
compose_dir: /opt/homelab
n8n_user: testuser
op_vault: TestVault
op_static_vault: StaticVault
orchestrator: kubernetes
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
	if cfg.N8NUser != "testuser" {
		t.Errorf("N8NUser = %q", cfg.N8NUser)
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
	if cfg.Kubernetes.Namespace != "homelab" {
		t.Errorf("default Kubernetes Namespace = %q", cfg.Kubernetes.Namespace)
	}
}
