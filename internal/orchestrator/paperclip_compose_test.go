package orchestrator

import (
	"os"
	"strings"
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
			Ports       []string          `yaml:"ports"`
			NetworkMode string            `yaml:"network_mode"`
			Environment map[string]string `yaml:"environment"`
			DependsOn   []string          `yaml:"depends_on"`
			Volumes     []string          `yaml:"volumes"`
			Networks    []string          `yaml:"networks"`
		} `yaml:"services"`
		Volumes  map[string]any `yaml:"volumes"`
		Networks map[string]struct {
			Driver   string `yaml:"driver"`
			Internal bool   `yaml:"internal"`
		} `yaml:"networks"`
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
	if paperclip.NetworkMode != "host" {
		t.Fatalf("paperclip network_mode = %q", paperclip.NetworkMode)
	}
	if len(paperclip.Ports) != 0 {
		t.Fatalf("paperclip ports = %v, want none with host networking", paperclip.Ports)
	}
	if len(paperclip.Networks) != 0 {
		t.Fatalf("paperclip networks = %v, want none with host networking", paperclip.Networks)
	}
	if paperclip.Environment["PAPERCLIP_PUBLIC_URL"] != "${PAPERCLIP_PUBLIC_URL:-https://paperclip.caboose-ai.io}" {
		t.Fatalf("PAPERCLIP_PUBLIC_URL = %q", paperclip.Environment["PAPERCLIP_PUBLIC_URL"])
	}
	if paperclip.Environment["PAPERCLIP_RUNTIME_API_URL"] != "http://127.0.0.1:3100" {
		t.Fatalf("PAPERCLIP_RUNTIME_API_URL = %q", paperclip.Environment["PAPERCLIP_RUNTIME_API_URL"])
	}
	if paperclip.Environment["SERVE_UI"] != "true" {
		t.Fatalf("SERVE_UI = %q", paperclip.Environment["SERVE_UI"])
	}
	if paperclip.Environment["HOST"] != "127.0.0.1" {
		t.Fatalf("HOST = %q", paperclip.Environment["HOST"])
	}
	if paperclip.Environment["PAPERCLIP_DEPLOYMENT_MODE"] != "local_trusted" {
		t.Fatalf("PAPERCLIP_DEPLOYMENT_MODE = %q", paperclip.Environment["PAPERCLIP_DEPLOYMENT_MODE"])
	}
	if paperclip.Environment["PAPERCLIP_DEPLOYMENT_EXPOSURE"] != "private" {
		t.Fatalf("PAPERCLIP_DEPLOYMENT_EXPOSURE = %q", paperclip.Environment["PAPERCLIP_DEPLOYMENT_EXPOSURE"])
	}
	if paperclip.Environment["PAPERCLIP_BIND"] != "loopback" {
		t.Fatalf("PAPERCLIP_BIND = %q", paperclip.Environment["PAPERCLIP_BIND"])
	}
	if paperclip.Environment["DATABASE_URL"] != "postgres://paperclip:${PAPERCLIP_DB_PASS}@127.0.0.1:5433/paperclip?sslmode=disable" {
		t.Fatalf("DATABASE_URL = %q", paperclip.Environment["DATABASE_URL"])
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
	if !contains(paperclip.Volumes, "paperclip_data:/paperclip") {
		t.Fatalf("paperclip volumes = %v, missing paperclip_data", paperclip.Volumes)
	}
	if !contains(paperclip.Volumes, "${PAPERCLIP_WORKSPACE_ROOT:-/home/caboose/dev/caboose-ai.io}:${PAPERCLIP_WORKSPACE_ROOT:-/home/caboose/dev/caboose-ai.io}:rw") {
		t.Fatalf("paperclip volumes = %v, missing workspace bind mount", paperclip.Volumes)
	}

	paperclipDB, ok := compose.Services["paperclip-db"]
	if !ok {
		t.Fatal("paperclip-db service missing")
	}
	if !contains(paperclipDB.Profiles, "paperclip") {
		t.Fatalf("paperclip-db profiles = %v", paperclipDB.Profiles)
	}
	if !contains(paperclipDB.Ports, "${PAPERCLIP_DB_BIND_ADDRESS:-127.0.0.1}:5433:5432") {
		t.Fatalf("paperclip-db ports = %v", paperclipDB.Ports)
	}
	if !contains(paperclipDB.Networks, "paperclip-internal") {
		t.Fatalf("paperclip-db networks = %v", paperclipDB.Networks)
	}
	paperclipInternal, ok := compose.Networks["paperclip-internal"]
	if !ok {
		t.Fatal("paperclip-internal network missing")
	}
	if paperclipInternal.Driver != "bridge" {
		t.Fatalf("paperclip-internal driver = %q", paperclipInternal.Driver)
	}
	if paperclipInternal.Internal {
		t.Fatal("paperclip-internal must publish the host-loopback DB port for the host-network Paperclip app")
	}
	if _, ok := compose.Volumes["paperclip_db"]; !ok {
		t.Fatal("paperclip_db volume missing")
	}
}

func TestPaperclipEnvExampleIncludesRequiredSecrets(t *testing.T) {
	data, err := os.ReadFile("../../dev/homelab/.env.example")
	if err != nil {
		t.Fatalf("read env example: %v", err)
	}
	for _, want := range []string{
		"PAPERCLIP_DB_PASS=CHANGE_ME",
		"PAPERCLIP_DB_BIND_ADDRESS=127.0.0.1",
		"PAPERCLIP_AUTH_SECRET=CHANGE_ME",
		"PAPERCLIP_PUBLIC_URL=https://paperclip.caboose-ai.io",
		"PAPERCLIP_WORKSPACE_ROOT=/home/caboose/dev/caboose-ai.io",
	} {
		if !containsLine(string(data), want) {
			t.Fatalf(".env.example missing %q", want)
		}
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

func containsLine(text, want string) bool {
	for _, line := range strings.Split(text, "\n") {
		if line == want {
			return true
		}
	}
	return false
}
