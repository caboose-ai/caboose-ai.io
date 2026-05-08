package n8n

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/caboose-ai/caboose-ai.io/internal/service"
)

type mockHTTP struct {
	do func(req *http.Request) (*http.Response, error)
}

func (m mockHTTP) Do(req *http.Request) (*http.Response, error) {
	return m.do(req)
}

func TestConfigureCreatesOwnerWhenSetupRequired(t *testing.T) {
	var sawSetup bool
	cfg := New("http://n8n.local", mockHTTP{do: func(req *http.Request) (*http.Response, error) {
		switch req.Method + " " + req.URL.Path {
		case "GET /rest/settings":
			return httpResponse(200, `{"data":{"userManagement":{"showSetupOnFirstLoad":true}}}`), nil
		case "POST /rest/owner/setup":
			body, _ := io.ReadAll(req.Body)
			sawSetup = strings.Contains(string(body), `"email":"admin@example.com"`) &&
				strings.Contains(string(body), `"password":"SecretPass1!"`)
			return httpResponse(200, `{}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	}}, "admin@example.com", "SecretPass1!")

	result, err := cfg.Configure(context.Background(), service.ConfigureOpts{})
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if result.Status != service.StatusCreated {
		t.Fatalf("status = %s, want %s", result.Status, service.StatusCreated)
	}
	if !sawSetup {
		t.Fatal("owner setup payload did not include expected credentials")
	}
}

func TestConfigureSkipsWhenOwnerExists(t *testing.T) {
	cfg := New("http://n8n.local", mockHTTP{do: func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/rest/settings" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
		return httpResponse(200, `{"data":{"userManagement":{"showSetupOnFirstLoad":false}}}`), nil
	}}, "admin@example.com", "SecretPass1!")

	result, err := cfg.Configure(context.Background(), service.ConfigureOpts{})
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if result.Status != service.StatusAlreadyConfigured {
		t.Fatalf("status = %s, want %s", result.Status, service.StatusAlreadyConfigured)
	}
}

func httpResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}
