package portainer

import (
	"context"
	"encoding/json"
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

func TestCheckConfiguredRequiresOAuthAndLocalEndpoint(t *testing.T) {
	tests := []struct {
		name          string
		settingsBody  string
		endpointsBody string
		want          bool
	}{
		{
			name:          "configured",
			settingsBody:  `{"OAuthSettings":{"ClientID":"client-id","UserIdentifier":"email","SSO":true}}`,
			endpointsBody: `[{"Id":1,"Name":"local","Type":1,"URL":"unix:///var/run/docker.sock"}]`,
			want:          true,
		},
		{
			name:          "missing local endpoint",
			settingsBody:  `{"OAuthSettings":{"ClientID":"client-id","UserIdentifier":"email","SSO":true}}`,
			endpointsBody: `[]`,
			want:          false,
		},
		{
			name:          "oauth uses wrong claim",
			settingsBody:  `{"OAuthSettings":{"ClientID":"client-id","UserIdentifier":"username","SSO":true}}`,
			endpointsBody: `[{"Id":1,"Name":"local","Type":1,"URL":"unix:///var/run/docker.sock"}]`,
			want:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Configurator{
				PortainerURL: "http://localhost:9000",
				AdminPass:    "test-pass",
				AuthHTTP: &mockHTTP{DoFunc: func(req *http.Request) (*http.Response, error) {
					if req.Method != http.MethodPost || req.URL.Path != "/api/auth" {
						t.Fatalf("unexpected auth request: %s %s", req.Method, req.URL.Path)
					}
					return httpResponse(http.StatusOK, `{"jwt":"jwt-token"}`), nil
				}},
			}

			c.HTTP = &mockHTTP{DoFunc: func(req *http.Request) (*http.Response, error) {
				if req.Header.Get("Authorization") != "Bearer jwt-token" {
					t.Fatalf("Authorization header = %q", req.Header.Get("Authorization"))
				}
				switch req.URL.Path {
				case "/api/settings":
					return httpResponse(http.StatusOK, tt.settingsBody), nil
				case "/api/endpoints":
					return httpResponse(http.StatusOK, tt.endpointsBody), nil
				default:
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
					return nil, nil
				}
			}}

			configured, err := c.CheckConfigured(context.Background())
			if err != nil {
				t.Fatalf("CheckConfigured: %v", err)
			}
			if configured != tt.want {
				t.Fatalf("configured = %v, want %v", configured, tt.want)
			}
		})
	}
}

func TestInitAdmin(t *testing.T) {
	tests := []struct {
		name        string
		status      int
		header      http.Header
		body        string
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
			name:   "admin init timeout redirect",
			status: 303,
			header: http.Header{
				"Redirect-Reason": []string{"AdminInitTimeout"},
			},
			body:    `{"message":"Administrator initialization timeout"}`,
			wantErr: "AdminInitTimeout",
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
					body := tt.body
					if body == "" {
						body = "{}"
					}
					resp := httpResponse(tt.status, body)
					resp.Header = tt.header
					return resp, nil
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

func TestApplyOAuthEnablesSSO(t *testing.T) {
	var payload struct {
		AuthenticationMethod int `json:"AuthenticationMethod"`
		OAuthSettings        struct {
			ClientID string `json:"ClientID"`
			SSO      bool   `json:"SSO"`
		} `json:"OAuthSettings"`
	}

	c := &Configurator{
		PortainerURL: "http://localhost:9000",
		AuthentikURL: "https://auth.example.com",
		RedirectURI:  "https://docker.example.com/",
		HTTP: &mockHTTP{DoFunc: func(req *http.Request) (*http.Response, error) {
			if req.Method != http.MethodPut || req.URL.Path != "/api/settings" {
				t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			}
			if req.Header.Get("Authorization") != "Bearer jwt-token" {
				t.Fatalf("Authorization header = %q", req.Header.Get("Authorization"))
			}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decoding payload: %v", err)
			}
			return httpResponse(http.StatusOK, `{}`), nil
		}},
	}

	if err := c.applyOAuth(context.Background(), "jwt-token", "client-id", "client-secret"); err != nil {
		t.Fatalf("applyOAuth: %v", err)
	}

	if payload.AuthenticationMethod != 3 {
		t.Fatalf("AuthenticationMethod = %d, want 3", payload.AuthenticationMethod)
	}
	if payload.OAuthSettings.ClientID != "client-id" {
		t.Fatalf("ClientID = %q, want client-id", payload.OAuthSettings.ClientID)
	}
	if !payload.OAuthSettings.SSO {
		t.Fatal("OAuthSettings.SSO = false, want true")
	}
}

func TestEnsureLocalEndpointCreatesMissingDockerSocket(t *testing.T) {
	calls := 0
	c := &Configurator{
		PortainerURL: "http://localhost:9000",
		HTTP: &mockHTTP{DoFunc: func(req *http.Request) (*http.Response, error) {
			calls++
			if req.Header.Get("Authorization") != "Bearer jwt-token" {
				t.Fatalf("Authorization header = %q", req.Header.Get("Authorization"))
			}
			switch calls {
			case 1:
				if req.Method != http.MethodGet || req.URL.Path != "/api/endpoints" {
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
				}
				return httpResponse(http.StatusOK, `[]`), nil
			case 2:
				if req.Method != http.MethodPost || req.URL.Path != "/api/endpoints" {
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
				}
				if err := req.ParseMultipartForm(1024); err != nil {
					t.Fatalf("ParseMultipartForm: %v", err)
				}
				if got := req.FormValue("Name"); got != "local" {
					t.Fatalf("Name = %q, want local", got)
				}
				if got := req.FormValue("EndpointCreationType"); got != "1" {
					t.Fatalf("EndpointCreationType = %q, want 1", got)
				}
				if got := req.FormValue("URL"); got != "unix:///var/run/docker.sock" {
					t.Fatalf("URL = %q, want Docker socket", got)
				}
				return httpResponse(http.StatusOK, `{"Id":1}`), nil
			default:
				t.Fatalf("unexpected extra request %d", calls)
				return nil, nil
			}
		}},
	}

	created, err := c.ensureLocalEndpoint(context.Background(), "jwt-token")
	if err != nil {
		t.Fatalf("ensureLocalEndpoint: %v", err)
	}
	if !created {
		t.Fatal("created = false, want true")
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
}

func TestEnsureLocalEndpointSkipsExistingDockerSocket(t *testing.T) {
	calls := 0
	c := &Configurator{
		PortainerURL: "http://localhost:9000",
		HTTP: &mockHTTP{DoFunc: func(req *http.Request) (*http.Response, error) {
			calls++
			if req.Method != http.MethodGet || req.URL.Path != "/api/endpoints" {
				t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			}
			return httpResponse(http.StatusOK, `[{"Id":1,"Name":"local","Type":1,"URL":"unix:///var/run/docker.sock"}]`), nil
		}},
	}

	created, err := c.ensureLocalEndpoint(context.Background(), "jwt-token")
	if err != nil {
		t.Fatalf("ensureLocalEndpoint: %v", err)
	}
	if created {
		t.Fatal("created = true, want false")
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestEnsureAdminUserPromotesConfiguredEmail(t *testing.T) {
	calls := 0
	c := &Configurator{
		PortainerURL: "http://localhost:9000",
		AdminEmail:   "admin@example.com",
		HTTP: &mockHTTP{DoFunc: func(req *http.Request) (*http.Response, error) {
			calls++
			if req.Header.Get("Authorization") != "Bearer jwt-token" {
				t.Fatalf("Authorization header = %q", req.Header.Get("Authorization"))
			}
			switch calls {
			case 1:
				if req.Method != http.MethodGet || req.URL.Path != "/api/users" {
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
				}
				return httpResponse(http.StatusOK, `[{"Id":2,"Username":"admin@example.com","Role":2}]`), nil
			case 2:
				if req.Method != http.MethodPut || req.URL.Path != "/api/users/2" {
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
				}
				var payload struct {
					Role int `json:"role"`
				}
				if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
					t.Fatalf("decoding payload: %v", err)
				}
				if payload.Role != 1 {
					t.Fatalf("role = %d, want 1", payload.Role)
				}
				return httpResponse(http.StatusOK, `{}`), nil
			default:
				t.Fatalf("unexpected extra request %d", calls)
				return nil, nil
			}
		}},
	}

	updated, err := c.ensureAdminUser(context.Background(), "jwt-token")
	if err != nil {
		t.Fatalf("ensureAdminUser: %v", err)
	}
	if !updated {
		t.Fatal("updated = false, want true")
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
}

func TestEnsureAdminUserSkipsAlreadyAdmin(t *testing.T) {
	calls := 0
	c := &Configurator{
		PortainerURL: "http://localhost:9000",
		AdminEmail:   "admin@example.com",
		HTTP: &mockHTTP{DoFunc: func(req *http.Request) (*http.Response, error) {
			calls++
			return httpResponse(http.StatusOK, `[{"Id":2,"Username":"admin@example.com","Role":1}]`), nil
		}},
	}

	updated, err := c.ensureAdminUser(context.Background(), "jwt-token")
	if err != nil {
		t.Fatalf("ensureAdminUser: %v", err)
	}
	if updated {
		t.Fatal("updated = true, want false")
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}
