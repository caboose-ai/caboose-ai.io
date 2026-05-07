package sonarqube

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/caboose-ai/caboose-ai.io/internal/services"
)

type mockHTTP struct {
	do func(req *http.Request) (*http.Response, error)
}

func (m mockHTTP) Do(req *http.Request) (*http.Response, error) {
	return m.do(req)
}

func TestConfigureRotatesDefaultAdminPassword(t *testing.T) {
	var changed bool
	cfg := New("http://sonar.local", mockHTTP{do: func(req *http.Request) (*http.Response, error) {
		switch req.Method + " " + req.URL.Path {
		case "GET /api/authentication/validate":
			_, pass, _ := req.BasicAuth()
			if pass == "new-pass" {
				return httpResponse(200, `{"valid":false}`), nil
			}
			if pass == defaultPassword {
				return httpResponse(200, `{"valid":true}`), nil
			}
		case "POST /api/users/change_password":
			body, _ := io.ReadAll(req.Body)
			changed = strings.Contains(string(body), "password=new-pass")
			return httpResponse(204, ""), nil
		}
		t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		return nil, nil
	}}, "new-pass")

	result, err := cfg.Configure(context.Background(), services.ConfigureOpts{})
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if result.Status != services.StatusUpdated {
		t.Fatalf("status = %s, want %s", result.Status, services.StatusUpdated)
	}
	if !changed {
		t.Fatal("password change request did not include generated password")
	}
}

func TestConfigureSkipsWhenGeneratedPasswordAlreadyWorks(t *testing.T) {
	cfg := New("http://sonar.local", mockHTTP{do: func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/api/authentication/validate" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
		_, pass, _ := req.BasicAuth()
		if pass != "new-pass" {
			t.Fatalf("password = %q, want generated password only", pass)
		}
		return httpResponse(200, `{"valid":true}`), nil
	}}, "new-pass")

	result, err := cfg.Configure(context.Background(), services.ConfigureOpts{})
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if result.Status != services.StatusAlreadyConfigured {
		t.Fatalf("status = %s, want %s", result.Status, services.StatusAlreadyConfigured)
	}
}

func httpResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}
