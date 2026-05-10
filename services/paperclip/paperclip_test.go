package paperclip

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/caboose-ai/caboose-ai.io/internal/service"
	"github.com/caboose-ai/caboose-ai.io/services/authentik"
)

func TestConfigureDryRun(t *testing.T) {
	cfg := New(nil)
	result, err := cfg.Configure(context.Background(), service.ConfigureOpts{DryRun: true})
	if err != nil {
		t.Fatalf("Configure dry-run: %v", err)
	}
	if result.Status != service.StatusDryRun {
		t.Fatalf("status = %s, want %s", result.Status, service.StatusDryRun)
	}
}

func TestCheckConfiguredRequiresProviderApplicationAndOutpost(t *testing.T) {
	ak := authentik.NewClient("http://localhost", "token", paperclipMockHTTP{do: func(req *http.Request) (*http.Response, error) {
		if req.URL.Path == "/api/v3/providers/proxy/" {
			return paperclipHTTPResponse(200, `{"results":[]}`), nil
		}
		t.Fatalf("unexpected request: %s", req.URL.String())
		return nil, nil
	}})

	configured, err := New(ak, "https://paperclip.example.com").CheckConfigured(context.Background())
	if err != nil {
		t.Fatalf("CheckConfigured: %v", err)
	}
	if configured {
		t.Fatal("CheckConfigured = true, want false without provider")
	}
}

func TestCheckConfiguredAcceptsForwardAuthProviderApplicationAndOutpost(t *testing.T) {
	ak := authentik.NewClient("http://localhost", "token", paperclipMockHTTP{do: func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/api/v3/providers/proxy/":
			return paperclipHTTPResponse(200, `{"results":[{"pk":42,"name":"paperclip-proxy","mode":"forward_single","external_host":"https://paperclip.example.com"}]}`), nil
		case "/api/v3/core/applications/":
			return paperclipHTTPResponse(200, `{"results":[{"pk":"paperclip-proxy","slug":"paperclip-proxy","name":"paperclip-proxy","policy_engine_mode":"all"}]}`), nil
		case "/api/v3/outposts/instances/":
			return paperclipHTTPResponse(200, `{"results":[{"pk":"embedded","name":"authentik Embedded Outpost","providers":[42]}]}`), nil
		default:
			t.Fatalf("unexpected request: %s", req.URL.String())
			return nil, nil
		}
	}})

	configured, err := New(ak, "https://paperclip.example.com").CheckConfigured(context.Background())
	if err != nil {
		t.Fatalf("CheckConfigured: %v", err)
	}
	if !configured {
		t.Fatal("CheckConfigured = false, want true")
	}
}

func TestConfigureRepairsExistingProxyApplicationAndOutpost(t *testing.T) {
	var calls []string
	ak := authentik.NewClient("https://auth.example.com", "token", paperclipMockHTTP{do: func(req *http.Request) (*http.Response, error) {
		calls = append(calls, req.Method+" "+req.URL.Path)
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/api/v3/providers/proxy/":
			return paperclipHTTPResponse(200, `{"results":[{"pk":42,"name":"paperclip-proxy","mode":"forward_domain","external_host":"https://old.example.com"}]}`), nil
		case req.Method == http.MethodPatch && req.URL.Path == "/api/v3/providers/proxy/42/":
			var payload map[string]string
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode proxy patch: %v", err)
			}
			if payload["mode"] != "forward_single" || payload["external_host"] != "https://paperclip.example.com" {
				t.Fatalf("proxy patch payload = %#v", payload)
			}
			return paperclipHTTPResponse(200, `{}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/api/v3/core/applications/":
			return paperclipHTTPResponse(200, `{"results":[{"pk":"paperclip-proxy","slug":"paperclip-proxy","name":"paperclip-proxy","policy_engine_mode":"any"}]}`), nil
		case req.Method == http.MethodPatch && req.URL.Path == "/api/v3/core/applications/paperclip-proxy/":
			return paperclipHTTPResponse(200, `{}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/api/v3/outposts/instances/":
			return paperclipHTTPResponse(200, `{"results":[{"pk":"embedded","name":"authentik Embedded Outpost","providers":[7]}]}`), nil
		case req.Method == http.MethodPatch && req.URL.Path == "/api/v3/outposts/instances/embedded/":
			var payload struct {
				Providers []int `json:"providers"`
			}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode outpost patch: %v", err)
			}
			if !containsProviderPK(payload.Providers, 42) {
				t.Fatalf("outpost providers = %v, want 42", payload.Providers)
			}
			return paperclipHTTPResponse(200, `{}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}})

	result, err := New(ak, "https://paperclip.example.com").Configure(context.Background(), service.ConfigureOpts{})
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if result.Status != service.StatusUpdated {
		t.Fatalf("status = %s, want %s", result.Status, service.StatusUpdated)
	}
	wantCalls := []string{
		"GET /api/v3/providers/proxy/",
		"PATCH /api/v3/providers/proxy/42/",
		"GET /api/v3/core/applications/",
		"GET /api/v3/core/applications/",
		"PATCH /api/v3/core/applications/paperclip-proxy/",
		"GET /api/v3/outposts/instances/",
		"PATCH /api/v3/outposts/instances/embedded/",
	}
	if strings.Join(calls, "\n") != strings.Join(wantCalls, "\n") {
		t.Fatalf("calls:\n%s\nwant:\n%s", strings.Join(calls, "\n"), strings.Join(wantCalls, "\n"))
	}
}

type paperclipMockHTTP struct {
	do func(req *http.Request) (*http.Response, error)
}

func (m paperclipMockHTTP) Do(req *http.Request) (*http.Response, error) {
	return m.do(req)
}

func paperclipHTTPResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       paperclipNopCloser{strings.NewReader(body)},
		Header:     make(http.Header),
	}
}

type paperclipNopCloser struct {
	*strings.Reader
}

func (paperclipNopCloser) Close() error { return nil }

var _ io.ReadCloser = paperclipNopCloser{}
