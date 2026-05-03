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

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	os.WriteFile(path, []byte(`
domain: test.example.com
email: admin@test.example.com
compose_dir: /opt/homelab
n8n_user: testuser
op_vault: TestVault
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
	if cfg.Social.GitHub == nil || cfg.Social.GitHub.ClientID != "gh-id" {
		t.Errorf("Social.GitHub = %+v", cfg.Social.GitHub)
	}
}

func TestValidate(t *testing.T) {
	cfg := DefaultConfig()
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for empty domain")
	}

	cfg.Domain = "example.com"
	if err := cfg.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.OPVault != "Homelab" {
		t.Errorf("default OPVault = %q, want Homelab", cfg.OPVault)
	}
	if cfg.ComposeDir != "dev/homelab" {
		t.Errorf("default ComposeDir = %q, want dev/homelab", cfg.ComposeDir)
	}
}
