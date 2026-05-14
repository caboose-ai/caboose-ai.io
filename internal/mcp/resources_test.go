package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/caboose-ai/caboose-ai.io/internal/config"
	"github.com/caboose-ai/caboose-ai.io/internal/docker"
	"github.com/caboose-ai/caboose-ai.io/internal/health"
	"github.com/caboose-ai/caboose-ai.io/internal/service"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestExtractServiceFromURI(t *testing.T) {
	tests := []struct {
		uri  string
		want string
	}{
		{"homelab://services/forgejo/status", "forgejo"},
		{"homelab://services/open-webui/status", "open-webui"},
		{"homelab://services//status", ""},
		{"invalid", ""},
	}
	for _, tt := range tests {
		got := extractServiceFromURI(tt.uri)
		if got != tt.want {
			t.Errorf("extractServiceFromURI(%q) = %q, want %q", tt.uri, got, tt.want)
		}
	}
}

func TestExtractServiceManifestFromURI(t *testing.T) {
	got := extractServiceManifestFromURI("homelab://services/forgejo/manifest")
	if got != "forgejo" {
		t.Fatalf("extractServiceManifestFromURI = %q, want forgejo", got)
	}
}

func TestExtractServiceCardFromURI(t *testing.T) {
	got := extractServiceCardFromURI("homelab://services/paperclip/card")
	if got != "paperclip" {
		t.Fatalf("extractServiceCardFromURI = %q, want paperclip", got)
	}
}

func TestHandleServiceManifestResource(t *testing.T) {
	s := &Server{
		registry: service.NewRegistry(map[string]service.Manifest{
			"forgejo": {
				Slug:            "forgejo",
				DisplayName:     "Forgejo",
				ComposeServices: []string{"forgejo"},
				Configurator:    "forgejo",
			},
		}, nil),
	}

	req := &sdkmcp.ReadResourceRequest{
		Params: &sdkmcp.ReadResourceParams{URI: "homelab://services/forgejo/manifest"},
	}

	result, err := s.handleServiceManifestResource(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := result.Contents[0].Text
	if !strings.Contains(text, `"slug": "forgejo"`) {
		t.Fatalf("manifest response missing slug: %q", text)
	}
}

func TestHandleServiceCardsResource(t *testing.T) {
	r := &mockRunner{err: context.Canceled}
	show := true
	s := &Server{
		cfg:     &config.Config{Domain: "example.com"},
		compose: docker.NewComposeClient(r, "/test"),
		registry: service.NewRegistry(map[string]service.Manifest{
			"paperclip": {
				Slug:            "paperclip",
				DisplayName:     "Paperclip",
				URLKey:          "paperclip",
				ComposeServices: []string{"paperclip", "paperclip-db"},
				Secrets:         []string{"PAPERCLIP_DB_PASS"},
				Configurator:    "paperclip",
				SmokeFlow:       "paperclip",
				Dashboard:       service.DashboardSpec{Show: &show},
				SSO:             service.SSOSpec{Mode: "proxy"},
				Health:          service.HealthSpec{URLKey: "paperclip", Path: "/api/health", Kind: "http"},
				Docs:            []string{"README.md"},
			},
		}, nil),
	}

	req := &sdkmcp.ReadResourceRequest{
		Params: &sdkmcp.ReadResourceParams{URI: "homelab://services/cards"},
	}

	result, err := s.handleServiceCardsResource(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(r.lastArgs) != 0 {
		t.Fatalf("compose runner args = %v, want none", r.lastArgs)
	}
	var index struct {
		Services []service.Card `json:"services"`
	}
	if err := json.Unmarshal([]byte(result.Contents[0].Text), &index); err != nil {
		t.Fatalf("unmarshal cards: %v\n%s", err, result.Contents[0].Text)
	}
	if len(index.Services) != 1 {
		t.Fatalf("services length = %d, want 1", len(index.Services))
	}
	card := index.Services[0]
	if card.Slug != "paperclip" || card.ResolvedURL != "https://paperclip.example.com" {
		t.Fatalf("card slug/url = %q/%q", card.Slug, card.ResolvedURL)
	}
	if card.SSOMode != "proxy" || card.Health.Path != "/api/health" {
		t.Fatalf("card sso/health = %q/%+v", card.SSOMode, card.Health)
	}
}

func TestHandleServiceCardResource(t *testing.T) {
	r := &mockRunner{err: context.Canceled}
	s := &Server{
		cfg:     &config.Config{Domain: "example.com"},
		compose: docker.NewComposeClient(r, "/test"),
		registry: service.NewRegistry(map[string]service.Manifest{
			"openclaw": {
				Slug:        "openclaw",
				DisplayName: "OpenClaw",
				Runtime:     service.RuntimeExternal,
				URLKey:      "openclaw",
				SmokeFlow:   "openclaw",
				SSO:         service.SSOSpec{Mode: "proxy"},
				Health:      service.HealthSpec{URLKey: "openclaw", Path: "/", Kind: "http"},
				Docs:        []string{"README.md"},
			},
		}, nil),
	}

	req := &sdkmcp.ReadResourceRequest{
		Params: &sdkmcp.ReadResourceParams{URI: "homelab://services/openclaw/card"},
	}

	result, err := s.handleServiceCardResource(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(r.lastArgs) != 0 {
		t.Fatalf("compose runner args = %v, want none", r.lastArgs)
	}
	var card service.Card
	if err := json.Unmarshal([]byte(result.Contents[0].Text), &card); err != nil {
		t.Fatalf("unmarshal card: %v\n%s", err, result.Contents[0].Text)
	}
	if card.Slug != "openclaw" || card.Runtime != service.RuntimeExternal {
		t.Fatalf("card identity/runtime = %q/%q", card.Slug, card.Runtime)
	}
	if card.ResolvedURL != "https://openclaw.example.com" {
		t.Fatalf("resolved URL = %q", card.ResolvedURL)
	}
}

func TestHandleServiceStatusResource(t *testing.T) {
	r := &mockRunner{output: []byte(`[{"Name":"forgejo","State":"running"}]`)}
	s := &Server{
		compose:  docker.NewComposeClient(r, "/test"),
		checkers: []health.Checker{&mockChecker{name: "Forgejo", err: nil}},
		services: []service.ServiceConfigurator{
			&mockServiceConfigurator{slug: "forgejo", name: "Forgejo", result: &service.ConfigureResult{Status: service.StatusAlreadyConfigured}},
		},
	}

	req := &sdkmcp.ReadResourceRequest{
		Params: &sdkmcp.ReadResourceParams{URI: "homelab://services/forgejo/status"},
	}

	result, err := s.handleServiceStatusResource(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := result.Contents[0].Text
	if !strings.Contains(text, `"running": true`) {
		t.Errorf("expected running true: %q", text)
	}
	if !strings.Contains(text, `"healthy": true`) {
		t.Errorf("expected healthy true: %q", text)
	}
	if !strings.Contains(text, `"configured": true`) {
		t.Errorf("expected configured true: %q", text)
	}
}

func TestHandleServiceStatusResourceIncludesAllManifestComposeServices(t *testing.T) {
	r := &mockRunner{output: []byte(`[{"Name":"paperclip","State":"running"},{"Name":"paperclip-db","State":"running"}]`)}
	s := &Server{
		compose: docker.NewComposeClient(r, "/test"),
		registry: service.NewRegistry(map[string]service.Manifest{
			"paperclip": {
				Slug:            "paperclip",
				DisplayName:     "Paperclip",
				ComposeServices: []string{"paperclip", "paperclip-db"},
			},
		}, nil),
	}

	req := &sdkmcp.ReadResourceRequest{
		Params: &sdkmcp.ReadResourceParams{URI: "homelab://services/paperclip/status"},
	}

	result, err := s.handleServiceStatusResource(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := result.Contents[0].Text
	for _, want := range []string{`"paperclip": true`, `"paperclip-db": true`} {
		if !strings.Contains(text, want) {
			t.Fatalf("status response missing %s: %q", want, text)
		}
	}
}

func TestHandleServiceStatusResourceSkipsComposeForExternalRuntime(t *testing.T) {
	r := &mockRunner{err: context.Canceled}
	s := &Server{
		compose: docker.NewComposeClient(r, "/test"),
		registry: service.NewRegistry(map[string]service.Manifest{
			"openclaw": {
				Slug:        "openclaw",
				DisplayName: "OpenClaw",
				Runtime:     service.RuntimeExternal,
				URLKey:      "openclaw",
				SmokeFlow:   "openclaw",
			},
		}, nil),
	}

	req := &sdkmcp.ReadResourceRequest{
		Params: &sdkmcp.ReadResourceParams{URI: "homelab://services/openclaw/status"},
	}

	result, err := s.handleServiceStatusResource(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(r.lastArgs) != 0 {
		t.Fatalf("compose runner args = %v, want none", r.lastArgs)
	}
	text := result.Contents[0].Text
	if !strings.Contains(text, `"runtime": "external"`) {
		t.Errorf("expected external runtime in status: %q", text)
	}
	if !strings.Contains(text, "external service has no local compose container") {
		t.Errorf("expected no local compose message: %q", text)
	}
	if strings.Contains(text, `"error"`) {
		t.Errorf("external service status should not report compose error: %q", text)
	}
}

func TestHandleDiagnoseServicePrompt(t *testing.T) {
	r := &mockRunner{output: []byte("some log output")}
	s := &Server{
		compose:  docker.NewComposeClient(r, "/test"),
		checkers: []health.Checker{&mockChecker{name: "Forgejo", err: nil}},
	}

	req := &sdkmcp.GetPromptRequest{
		Params: &sdkmcp.GetPromptParams{
			Name:      "diagnose-service",
			Arguments: map[string]string{"service": "forgejo"},
		},
	}

	result, err := s.handleDiagnoseServicePrompt(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := result.Messages[0].Content.(*sdkmcp.TextContent).Text
	if !strings.Contains(text, "Diagnostic Report: forgejo") {
		t.Errorf("expected diagnostic header: %q", text)
	}
	if !strings.Contains(text, "Healthy") {
		t.Errorf("expected health section: %q", text)
	}
}

func TestHandleFullStackReportPrompt(t *testing.T) {
	r := &mockRunner{output: []byte(`[{"Name":"forgejo"}]`)}
	s := &Server{
		compose:  docker.NewComposeClient(r, "/test"),
		checkers: []health.Checker{&mockChecker{name: "Forgejo", err: nil}},
		services: []service.ServiceConfigurator{
			&mockServiceConfigurator{slug: "forgejo", name: "Forgejo", result: &service.ConfigureResult{Status: service.StatusAlreadyConfigured}},
		},
	}

	result, err := s.handleFullStackReportPrompt(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := result.Messages[0].Content.(*sdkmcp.TextContent).Text
	if !strings.Contains(text, "Full Stack Report") {
		t.Errorf("expected report header: %q", text)
	}
	if !strings.Contains(text, "Forgejo: healthy") {
		t.Errorf("expected Forgejo health: %q", text)
	}
	if !strings.Contains(text, "Forgejo: configured") {
		t.Errorf("expected Forgejo config status: %q", text)
	}
}

func TestURLsResource(t *testing.T) {
	urls := config.DeriveURLs("example.com")
	if !strings.Contains(urls.Authentik, "auth.example.com") {
		t.Errorf("expected auth URL, got %q", urls.Authentik)
	}
}

func TestStaticResourceReadsFile(t *testing.T) {
	dir := t.TempDir()
	content := "test: true\nservices:\n  forgejo:\n    image: gitea"
	os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte(content), 0644)

	s := &Server{cfg: &config.Config{ComposeDir: dir}}

	req := &sdkmcp.ReadResourceRequest{
		Params: &sdkmcp.ReadResourceParams{URI: "homelab://config/docker-compose"},
	}

	handler := func(ctx context.Context, req *sdkmcp.ReadResourceRequest) (*sdkmcp.ReadResourceResult, error) {
		path := filepath.Join(s.cfg.ComposeDir, "docker-compose.yml")
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		return &sdkmcp.ReadResourceResult{
			Contents: []*sdkmcp.ResourceContents{{URI: req.Params.URI, Text: string(data)}},
		}, nil
	}

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Contents[0].Text != content {
		t.Errorf("expected file content, got %q", result.Contents[0].Text)
	}
}
