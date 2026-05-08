package homarr

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/caboose-ai/caboose-ai.io/internal/secrets"
	"github.com/caboose-ai/caboose-ai.io/internal/service"
	"github.com/caboose-ai/caboose-ai.io/services/authentik"
)

type memorySecrets struct {
	values map[string]string
}

func (m *memorySecrets) Get(_ context.Context, key string) (string, error) {
	return m.values[key], nil
}

func (m *memorySecrets) Put(_ context.Context, key, value string) error {
	m.values[key] = value
	return nil
}

func (m *memorySecrets) Generate(_ context.Context, key string, length int, _ secrets.GenerateOpts) (string, error) {
	m.values[key] = strings.Repeat("x", length)
	return m.values[key], nil
}

func (m *memorySecrets) Delete(_ context.Context, key string) error {
	delete(m.values, key)
	return nil
}

func (m *memorySecrets) EnsureVault(context.Context) error { return nil }

type mockHTTP struct {
	do func(req *http.Request) (*http.Response, error)
}

func (m mockHTTP) Do(req *http.Request) (*http.Response, error) {
	return m.do(req)
}

func TestConfigureWritesHomarrOIDCSecrets(t *testing.T) {
	store := &memorySecrets{values: map[string]string{}}
	ak := authentik.NewClient("http://localhost", "test-token", mockHTTP{do: func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/api/v3/providers/oauth2/") {
			return httpResponse(200, `{"results":[{"pk":1,"name":"homarr","client_id":"homarr-client","client_secret":"homarr-secret"}]}`), nil
		}
		t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		return nil, nil
	}})
	cfg := New(ak, store)

	result, err := cfg.Configure(context.Background(), service.ConfigureOpts{})
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if result.Status != service.StatusCreated {
		t.Fatalf("status = %s, want %s", result.Status, service.StatusCreated)
	}
	if got := store.values["HOMARR_OIDC_CLIENT_ID"]; got != "homarr-client" {
		t.Fatalf("client id = %q", got)
	}
	if got := store.values["HOMARR_OIDC_CLIENT_SECRET"]; got != "homarr-secret" {
		t.Fatalf("client secret = %q", got)
	}
	if !result.RestartRequired || len(result.Services) != 1 || result.Services[0] != "homarr" {
		t.Fatalf("restart result = %+v", result)
	}
}

func httpResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       ioNopCloser{strings.NewReader(body)},
		Header:     make(http.Header),
	}
}

type ioNopCloser struct {
	*strings.Reader
}

func (ioNopCloser) Close() error { return nil }
