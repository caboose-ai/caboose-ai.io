package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/caboose-ai/caboose-ai.io/internal/config"
)

func TestPrintOAuthSetup(t *testing.T) {
	var out bytes.Buffer
	PrintOAuthSetup(&out, &config.Config{Domain: "example.com"})
	text := out.String()

	want := []string{
		"https://auth.example.com/source/oauth/callback/github/",
		"https://auth.example.com/source/oauth/callback/google/",
		"Hostname: auth.example.com",
		"GITHUB_OAUTH_CLIENT_ID",
		"GOOGLE_OAUTH_CLIENT_SECRET",
		"TURNSTILE_SECRET_KEY",
	}
	for _, s := range want {
		if !strings.Contains(text, s) {
			t.Fatalf("setup output missing %q:\n%s", s, text)
		}
	}
}

func TestRunOAuthSetupCreatesTurnstileWidget(t *testing.T) {
	var gotAuth string
	var gotRequest turnstileWidgetCreateRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/accounts/account-123/challenges/widgets" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&gotRequest); err != nil {
			t.Fatalf("decoding request: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"success": true,
			"errors": [],
			"messages": [],
			"result": {
				"sitekey": "site-key",
				"secret": "secret-key"
			}
		}`))
	}))
	defer server.Close()

	var out bytes.Buffer
	cfg := &config.Config{Domain: "example.com"}
	result, err := RunOAuthSetup(context.Background(), &out, cfg, OAuthSetupOptions{
		CreateTurnstile:      true,
		CloudflareAccountID:  "account-123",
		CloudflareAPIToken:   "cf-token",
		CloudflareAPIBaseURL: server.URL,
		TurnstileWidgetName:  "Test Widget",
		HTTPClient:           server.Client(),
	})
	if err != nil {
		t.Fatalf("RunOAuthSetup returned error: %v", err)
	}

	if gotAuth != "Bearer cf-token" {
		t.Fatalf("Authorization header = %q", gotAuth)
	}
	if gotRequest.Mode != "managed" {
		t.Fatalf("mode = %q", gotRequest.Mode)
	}
	if gotRequest.Name != "Test Widget" {
		t.Fatalf("name = %q", gotRequest.Name)
	}
	if len(gotRequest.Domains) != 1 || gotRequest.Domains[0] != "auth.example.com" {
		t.Fatalf("domains = %#v", gotRequest.Domains)
	}
	if result.Turnstile == nil || result.Turnstile.SiteKey != "site-key" || result.Turnstile.SecretKey != "secret-key" {
		t.Fatalf("unexpected turnstile result: %#v", result.Turnstile)
	}
	if cfg.Turnstile.SiteKey != "site-key" || cfg.Turnstile.SecretKey != "secret-key" {
		t.Fatalf("config turnstile keys were not populated: %#v", cfg.Turnstile)
	}

	text := out.String()
	want := []string{
		"Created via Cloudflare API",
		"TURNSTILE_SITE_KEY=site-key",
		"TURNSTILE_SECRET_KEY=secret-key",
	}
	for _, s := range want {
		if !strings.Contains(text, s) {
			t.Fatalf("setup output missing %q:\n%s", s, text)
		}
	}
}

func TestRunOAuthSetupRequiresTurnstileCredentials(t *testing.T) {
	var out bytes.Buffer
	_, err := RunOAuthSetup(context.Background(), &out, &config.Config{Domain: "example.com"}, OAuthSetupOptions{
		CreateTurnstile: true,
	})
	if err == nil {
		t.Fatal("RunOAuthSetup returned nil error")
	}
	if !strings.Contains(err.Error(), "cloudflare account id is required") {
		t.Fatalf("error = %q", err.Error())
	}
}
