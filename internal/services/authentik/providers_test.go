package authentik

import (
	"context"
	"net/http"
	"testing"
)

func TestGetApplicationMatchesExactSlug(t *testing.T) {
	client := &Client{
		BaseURL: "http://localhost",
		Token:   "test-token",
		HTTP: &mockHTTPClient{
			DoFunc: func(req *http.Request) (*http.Response, error) {
				if req.URL.Path != "/api/v3/core/applications/" {
					t.Errorf("unexpected path: %s", req.URL.Path)
				}
				if got := req.URL.Query().Get("search"); got != "grafana" {
					t.Errorf("search query = %q, want grafana", got)
				}
				body := `{"results":[` +
					`{"pk":"wrong","slug":"authentik-default","name":"Authentik","policy_engine_mode":"all"},` +
					`{"pk":"right","slug":"grafana","name":"grafana","policy_engine_mode":"any"}` +
					`]}`
				return mockResponse(200, body), nil
			},
		},
	}

	app, err := client.GetApplication(context.Background(), "grafana")
	if err != nil {
		t.Fatalf("GetApplication returned error: %v", err)
	}
	if app == nil {
		t.Fatal("GetApplication returned nil")
	}
	if app.PK != "right" {
		t.Errorf("PK = %q, want right", app.PK)
	}
	if app.Slug != "grafana" {
		t.Errorf("Slug = %q, want grafana", app.Slug)
	}
}

func TestGetApplicationReturnsNilWithoutExactSlug(t *testing.T) {
	client := &Client{
		BaseURL: "http://localhost",
		Token:   "test-token",
		HTTP: &mockHTTPClient{
			DoFunc: func(req *http.Request) (*http.Response, error) {
				body := `{"results":[{"pk":"wrong","slug":"authentik-default","name":"Authentik","policy_engine_mode":"all"}]}`
				return mockResponse(200, body), nil
			},
		},
	}

	app, err := client.GetApplication(context.Background(), "grafana")
	if err != nil {
		t.Fatalf("GetApplication returned error: %v", err)
	}
	if app != nil {
		t.Fatalf("GetApplication returned %+v, want nil", app)
	}
}
