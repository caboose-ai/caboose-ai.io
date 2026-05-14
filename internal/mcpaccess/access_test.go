package mcpaccess

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRequestReleaseRoundTripSealsCredentialForRequester(t *testing.T) {
	pendingDir := t.TempDir()
	now := time.Date(2026, 5, 13, 1, 2, 3, 0, time.UTC)

	req, err := CreateRequest(RequestOptions{
		Name:       "codex on caboose",
		Endpoint:   "https://mcp.example.com",
		PendingDir: pendingDir,
		Now:        func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("CreateRequest returned error: %v", err)
	}
	if req.Version != FormatVersion || req.Name != "codex on caboose" || req.Scope != ScopeHomelab {
		t.Fatalf("request = %#v", req)
	}
	if req.ID == "" || req.PublicKey == "" {
		t.Fatalf("request omitted id or public key: %#v", req)
	}
	keyPath := filepath.Join(pendingDir, req.ID+".key")
	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("pending key was not written: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("pending key mode = %o, want 0600", mode)
	}

	cred := Credential{
		Endpoint:     "https://mcp.example.com",
		AuthentikURL: "https://auth.example.com",
		ClientID:     "mcp-client",
		Username:     "ak-service-account-codex",
		AppPassword:  "app-password-secret",
		Scope:        ScopeHomelab,
		ExpiresAt:    now.Add(24 * time.Hour),
	}
	release, err := SealRelease(req, cred, ReleaseOptions{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("SealRelease returned error: %v", err)
	}
	data, err := json.Marshal(release)
	if err != nil {
		t.Fatalf("marshal release: %v", err)
	}
	if strings.Contains(string(data), "app-password-secret") {
		t.Fatalf("release JSON leaked app password: %s", data)
	}

	got, err := OpenRelease(release, pendingDir)
	if err != nil {
		t.Fatalf("OpenRelease returned error: %v", err)
	}
	if got != cred {
		t.Fatalf("credential = %#v, want %#v", got, cred)
	}
}

func TestCredentialStoreUsesOwnerOnlyPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mcp-access.json")
	cred := Credential{
		Endpoint:     "https://mcp.example.com",
		AuthentikURL: "https://auth.example.com",
		ClientID:     "mcp-client",
		Username:     "ak-service-account-codex",
		AppPassword:  "app-password-secret",
		Scope:        ScopeHomelab,
	}

	if err := StoreCredential(path, cred); err != nil {
		t.Fatalf("StoreCredential returned error: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("credential file was not written: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("credential file mode = %o, want 0600", mode)
	}
	got, err := LoadCredential(path)
	if err != nil {
		t.Fatalf("LoadCredential returned error: %v", err)
	}
	if got != cred {
		t.Fatalf("credential = %#v, want %#v", got, cred)
	}
}

func TestMintTokenUsesAuthentikClientCredentialsForm(t *testing.T) {
	var sawRequest bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawRequest = true
		if r.URL.Path != "/application/o/token/" {
			t.Fatalf("path = %q, want /application/o/token/", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm: %v", err)
		}
		want := map[string]string{
			"grant_type": "client_credentials",
			"client_id":  "mcp-client",
			"username":   "ak-service-account-codex",
			"password":   "app-password-secret",
			"scope":      ScopeHomelab,
		}
		for key, value := range want {
			if got := r.Form.Get(key); got != value {
				t.Fatalf("form[%s] = %q, want %q", key, got, value)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"access-token-123","token_type":"Bearer","expires_in":3600}`))
	}))
	defer server.Close()

	token, err := MintToken(context.Background(), Credential{
		AuthentikURL: server.URL,
		ClientID:     "mcp-client",
		Username:     "ak-service-account-codex",
		AppPassword:  "app-password-secret",
		Scope:        ScopeHomelab,
	}, server.Client())
	if err != nil {
		t.Fatalf("MintToken returned error: %v", err)
	}
	if !sawRequest {
		t.Fatal("token endpoint was not called")
	}
	if token != "access-token-123" {
		t.Fatalf("token = %q, want access-token-123", token)
	}
}
