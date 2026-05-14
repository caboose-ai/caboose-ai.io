package mcp

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/caboose-ai/caboose-ai.io/internal/config"
)

func TestVerifyTokenIntrospectsBearerToken(t *testing.T) {
	store := &mockSecretStore{data: map[string]string{
		"MCP_OAUTH_CLIENT_ID":     "mcp-client",
		"MCP_OAUTH_CLIENT_SECRET": "mcp-secret",
	}}
	rawToken := "token+with&reserved=chars"

	httpClient := introspectionHTTPClient{t: t, token: rawToken}

	s := &Server{
		cfg:     &config.Config{Domain: "example.com"},
		http:    httpClient,
		secrets: store,
	}

	info, err := s.verifyToken(context.Background(), rawToken, nil)
	if err != nil {
		t.Fatalf("verifyToken returned error: %v", err)
	}
	if info.UserID != "ak-service-account-mcp" {
		t.Fatalf("UserID = %q", info.UserID)
	}
	if len(info.Scopes) != 2 || info.Scopes[0] != "homelab" || info.Scopes[1] != "profile" {
		t.Fatalf("scopes = %#v", info.Scopes)
	}
}

type introspectionHTTPClient struct {
	t     *testing.T
	token string
}

func (c introspectionHTTPClient) Do(r *http.Request) (*http.Response, error) {
	c.t.Helper()
	if r.URL.String() != "https://auth.example.com/application/o/introspect/" {
		c.t.Fatalf("URL = %q, want introspection endpoint", r.URL.String())
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/application/o/introspect/" {
			c.t.Fatalf("path = %q, want introspection endpoint", r.URL.Path)
		}
		clientID, clientSecret, ok := r.BasicAuth()
		if !ok || clientID != "mcp-client" || clientSecret != "mcp-secret" {
			c.t.Fatalf("basic auth = %q/%q ok=%v", clientID, clientSecret, ok)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			c.t.Fatalf("read body: %v", err)
		}
		if strings.Contains(string(body), c.token) {
			c.t.Fatalf("body was not form-encoded: %q", body)
		}
		r.Body = io.NopCloser(strings.NewReader(string(body)))
		if err := r.ParseForm(); err != nil {
			c.t.Fatalf("ParseForm: %v", err)
		}
		if got := r.Form.Get("token"); got != c.token {
			c.t.Fatalf("token = %q, want %q", got, c.token)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"active":true,"username":"ak-service-account-mcp","scope":"homelab profile"}`))
	})
	w := &responseRecorder{header: http.Header{}}
	handler.ServeHTTP(w, r)
	return &http.Response{
		StatusCode: w.statusCode(),
		Header:     w.header,
		Body:       io.NopCloser(strings.NewReader(w.body.String())),
	}, nil
}

type responseRecorder struct {
	header http.Header
	body   strings.Builder
	status int
}

func (r *responseRecorder) Header() http.Header {
	return r.header
}

func (r *responseRecorder) Write(data []byte) (int, error) {
	return r.body.Write(data)
}

func (r *responseRecorder) WriteHeader(status int) {
	r.status = status
}

func (r *responseRecorder) statusCode() int {
	if r.status == 0 {
		return http.StatusOK
	}
	return r.status
}
