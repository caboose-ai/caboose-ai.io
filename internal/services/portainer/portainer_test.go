package portainer

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type mockHTTP struct {
	DoFunc func(req *http.Request) (*http.Response, error)
}

func (m *mockHTTP) Do(req *http.Request) (*http.Response, error) {
	return m.DoFunc(req)
}

func httpResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestGetJWT(t *testing.T) {
	tests := []struct {
		name    string
		authFn  func(req *http.Request) (*http.Response, error)
		wantJWT string
		wantErr string
	}{
		{
			name: "success on first attempt",
			authFn: func(req *http.Request) (*http.Response, error) {
				return httpResponse(200, `{"jwt":"token-abc"}`), nil
			},
			wantJWT: "token-abc",
		},
		{
			name: "success after redirect retry",
			authFn: func() func(req *http.Request) (*http.Response, error) {
				calls := 0
				return func(req *http.Request) (*http.Response, error) {
					calls++
					if calls == 1 {
						return httpResponse(303, ""), nil
					}
					return httpResponse(200, `{"jwt":"token-retry"}`), nil
				}
			}(),
			wantJWT: "token-retry",
		},
		{
			name: "non-200 non-3xx returns immediately",
			authFn: func(req *http.Request) (*http.Response, error) {
				return httpResponse(401, "unauthorized"), nil
			},
			wantErr: "Portainer auth returned HTTP 401",
		},
		{
			name: "empty JWT",
			authFn: func(req *http.Request) (*http.Response, error) {
				return httpResponse(200, `{"jwt":""}`), nil
			},
			wantErr: "Portainer auth returned empty JWT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Configurator{
				PortainerURL: "http://localhost:9000",
				AdminPass:    "test-pass",
				AuthHTTP:     &mockHTTP{DoFunc: tt.authFn},
			}

			jwt, err := c.getJWT(context.Background())

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error %q does not contain %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if jwt != tt.wantJWT {
				t.Errorf("jwt = %q, want %q", jwt, tt.wantJWT)
			}
		})
	}
}

func TestGetJWT_RedirectExhausted(t *testing.T) {
	calls := 0
	c := &Configurator{
		PortainerURL: "http://localhost:9000",
		AdminPass:    "test-pass",
		AuthHTTP: &mockHTTP{DoFunc: func(req *http.Request) (*http.Response, error) {
			calls++
			return httpResponse(303, ""), nil
		}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	_, err := c.getJWT(ctx)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "still returning redirects") {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 5 {
		t.Errorf("expected 5 attempts, got %d", calls)
	}
}

func TestGetJWT_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	c := &Configurator{
		PortainerURL: "http://localhost:9000",
		AdminPass:    "test-pass",
		AuthHTTP: &mockHTTP{DoFunc: func(req *http.Request) (*http.Response, error) {
			return httpResponse(303, ""), nil
		}},
	}

	_, err := c.getJWT(ctx)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "waiting for Portainer auth") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInitAdmin(t *testing.T) {
	tests := []struct {
		name        string
		status      int
		wantAlready bool
		wantErr     string
	}{
		{
			name:        "fresh init",
			status:      200,
			wantAlready: false,
		},
		{
			name:        "already initialized",
			status:      409,
			wantAlready: true,
		},
		{
			name:    "server error",
			status:  500,
			wantErr: "Portainer admin init returned HTTP 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Configurator{
				PortainerURL: "http://localhost:9000",
				AdminPass:    "test-pass",
				HTTP: &mockHTTP{DoFunc: func(req *http.Request) (*http.Response, error) {
					return httpResponse(tt.status, "{}"), nil
				}},
			}

			already, err := c.initAdmin(context.Background())

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error %q does not contain %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if already != tt.wantAlready {
				t.Errorf("alreadyInit = %v, want %v", already, tt.wantAlready)
			}
		})
	}
}
