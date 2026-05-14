package authentik

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCreateOAuthScopeMappingPostsHomelabScope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/v3/propertymappings/provider/scope/" {
			t.Fatalf("path = %q, want scope mapping endpoint", r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body["scope_name"] != "homelab" {
			t.Fatalf("scope_name = %#v, want homelab", body["scope_name"])
		}
		if body["expression"] == "" {
			t.Fatal("expression was empty")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"pk":"mapping-pk","name":"Homelab MCP","scope_name":"homelab","managed":""}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "admin-token", server.Client())
	mapping, err := client.CreateOAuthScopeMapping(context.Background(), CreateOAuthScopeMappingParams{
		Name:        "Homelab MCP",
		ScopeName:   "homelab",
		Description: "Homelab MCP access",
		Expression:  `return {}`,
	})
	if err != nil {
		t.Fatalf("CreateOAuthScopeMapping returned error: %v", err)
	}
	if mapping.PK != "mapping-pk" {
		t.Fatalf("mapping pk = %q, want mapping-pk", mapping.PK)
	}
}

func TestCreateServiceAccountReturnsOneTimeToken(t *testing.T) {
	expires := time.Date(2027, 5, 13, 0, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/v3/core/users/service_account/" {
			t.Fatalf("path = %q, want service account endpoint", r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body["name"] != "mcp-codex" {
			t.Fatalf("name = %#v, want mcp-codex", body["name"])
		}
		if body["create_group"] != false || body["expiring"] != true {
			t.Fatalf("body = %#v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"username":"ak-service-account-mcp-codex","token":"one-time-token","user_uid":"uid","user_pk":42}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "admin-token", server.Client())
	account, err := client.CreateServiceAccount(context.Background(), CreateServiceAccountParams{
		Name:        "mcp-codex",
		CreateGroup: false,
		Expiring:    true,
		Expires:     expires,
	})
	if err != nil {
		t.Fatalf("CreateServiceAccount returned error: %v", err)
	}
	if account.Username != "ak-service-account-mcp-codex" || account.Token != "one-time-token" {
		t.Fatalf("service account = %#v", account)
	}
}
