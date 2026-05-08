package orchestrator

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestPaperclipComposeProfile(t *testing.T) {
	data, err := os.ReadFile("../../dev/homelab/docker-compose.yml")
	if err != nil {
		t.Fatalf("read compose: %v", err)
	}

	var compose struct {
		Services map[string]struct {
			Image       string            `yaml:"image"`
			Build       map[string]string `yaml:"build"`
			Profiles    []string          `yaml:"profiles"`
			Environment map[string]string `yaml:"environment"`
			DependsOn   []string          `yaml:"depends_on"`
			Volumes     []string          `yaml:"volumes"`
			Networks    []string          `yaml:"networks"`
		} `yaml:"services"`
		Volumes map[string]any `yaml:"volumes"`
	}
	if err := yaml.Unmarshal(data, &compose); err != nil {
		t.Fatalf("parse compose: %v", err)
	}

	paperclip, ok := compose.Services["paperclip"]
	if !ok {
		t.Fatal("paperclip service missing")
	}
	if paperclip.Image != "paperclipai/paperclip:v2026.428.0" {
		t.Fatalf("paperclip image = %q", paperclip.Image)
	}
	if paperclip.Build["context"] != "https://github.com/paperclipai/paperclip.git#v2026.428.0" {
		t.Fatalf("paperclip build context = %q", paperclip.Build["context"])
	}
	if !contains(paperclip.Profiles, "paperclip") {
		t.Fatalf("paperclip profiles = %v", paperclip.Profiles)
	}
	if !contains(paperclip.DependsOn, "paperclip-db") {
		t.Fatalf("paperclip depends_on = %v", paperclip.DependsOn)
	}
	if paperclip.Environment["PAPERCLIP_PUBLIC_URL"] != "${PAPERCLIP_PUBLIC_URL:-https://paperclip.caboose-ai.io}" {
		t.Fatalf("PAPERCLIP_PUBLIC_URL = %q", paperclip.Environment["PAPERCLIP_PUBLIC_URL"])
	}
	if paperclip.Environment["SERVE_UI"] != "true" {
		t.Fatalf("SERVE_UI = %q", paperclip.Environment["SERVE_UI"])
	}
	if paperclip.Environment["PAPERCLIP_DEPLOYMENT_MODE"] != "authenticated" {
		t.Fatalf("PAPERCLIP_DEPLOYMENT_MODE = %q", paperclip.Environment["PAPERCLIP_DEPLOYMENT_MODE"])
	}
	if paperclip.Environment["PAPERCLIP_DEPLOYMENT_EXPOSURE"] != "private" {
		t.Fatalf("PAPERCLIP_DEPLOYMENT_EXPOSURE = %q", paperclip.Environment["PAPERCLIP_DEPLOYMENT_EXPOSURE"])
	}
	if paperclip.Environment["PAPERCLIP_TELEMETRY_DISABLED"] != "1" {
		t.Fatalf("PAPERCLIP_TELEMETRY_DISABLED = %q", paperclip.Environment["PAPERCLIP_TELEMETRY_DISABLED"])
	}
	if paperclip.Environment["BETTER_AUTH_SECRET"] != "${PAPERCLIP_AUTH_SECRET}" {
		t.Fatalf("BETTER_AUTH_SECRET = %q", paperclip.Environment["BETTER_AUTH_SECRET"])
	}
	if paperclip.Environment["PAPERCLIP_AGENT_JWT_SECRET"] != "${PAPERCLIP_AUTH_SECRET}" {
		t.Fatalf("PAPERCLIP_AGENT_JWT_SECRET = %q", paperclip.Environment["PAPERCLIP_AGENT_JWT_SECRET"])
	}

	if _, ok := compose.Services["paperclip-db"]; !ok {
		t.Fatal("paperclip-db service missing")
	}
	if _, ok := compose.Volumes["paperclip_db"]; !ok {
		t.Fatal("paperclip_db volume missing")
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
