package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/caboose-ai/caboose-ai.io/internal/config"
	"github.com/caboose-ai/caboose-ai.io/internal/mcpaccess"
	"github.com/caboose-ai/caboose-ai.io/internal/secrets"
)

func TestRunMCPAccessAdminApproveCreatesServiceAccountAndSealedRelease(t *testing.T) {
	pendingDir := t.TempDir()
	req, err := mcpaccess.CreateRequest(mcpaccess.RequestOptions{
		Name:       "codex on caboose",
		Endpoint:   "https://mcp.example.com",
		PendingDir: pendingDir,
	})
	if err != nil {
		t.Fatalf("CreateRequest: %v", err)
	}
	requestPath := filepath.Join(t.TempDir(), "request.json")
	writeJSONFile(t, requestPath, req)

	var sawServiceAccount bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/core/users/service_account/" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		sawServiceAccount = true
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body["name"] != "mcp-codex-on-caboose" {
			t.Fatalf("service account name = %#v", body["name"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"username":"ak-service-account-mcp-codex","token":"service-token","user_uid":"uid","user_pk":42}`))
	}))
	defer server.Close()

	releasePath := filepath.Join(t.TempDir(), "release.json")
	var stdout, stderr bytes.Buffer
	store := &memorySecretStore{values: map[string]string{
		"AUTHENTIK_BOOTSTRAP_TOKEN": "admin-token",
		"MCP_OAUTH_CLIENT_ID":       "mcp-client",
		"MCP_OAUTH_CLIENT_SECRET":   "mcp-secret",
	}}
	cfg := config.DefaultConfig()
	cfg.Domain = "example.com"

	code := RunMCPAccessAdmin(context.Background(), MCPAccessAdminOptions{
		Config:   cfg,
		Secrets:  store,
		HTTP:     server.Client(),
		Stdout:   &stdout,
		Stderr:   &stderr,
		AdminURL: server.URL,
	}, []string{"approve", requestPath, "--out", releasePath, "--expires", "24h"})
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	if !sawServiceAccount {
		t.Fatal("service account endpoint was not called")
	}
	var release mcpaccess.Release
	readJSONFile(t, releasePath, &release)
	cred, err := mcpaccess.OpenRelease(&release, pendingDir)
	if err != nil {
		t.Fatalf("OpenRelease: %v", err)
	}
	if cred.ClientID != "mcp-client" || cred.Username != "ak-service-account-mcp-codex" || cred.AppPassword != "service-token" {
		t.Fatalf("credential = %#v", cred)
	}
	if cred.Endpoint != "https://mcp.example.com" || cred.AuthentikURL != server.URL || cred.Scope != mcpaccess.ScopeHomelab {
		t.Fatalf("credential routing = %#v", cred)
	}
}

type memorySecretStore struct {
	values map[string]string
}

func (s *memorySecretStore) Get(_ context.Context, key string) (string, error) {
	return s.values[key], nil
}

func (s *memorySecretStore) Put(_ context.Context, key, value string) error {
	s.values[key] = value
	return nil
}

func (s *memorySecretStore) Generate(_ context.Context, key string, _ int, _ secrets.GenerateOpts) (string, error) {
	return s.values[key], nil
}

func (s *memorySecretStore) Delete(_ context.Context, key string) error {
	delete(s.values, key)
	return nil
}

func (s *memorySecretStore) EnsureVault(_ context.Context) error {
	return nil
}

func writeJSONFile(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readJSONFile(t *testing.T, path string, value any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := json.Unmarshal(data, value); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
}
