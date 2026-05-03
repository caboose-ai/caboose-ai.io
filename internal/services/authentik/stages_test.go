package authentik

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type mockHTTPClient struct {
	DoFunc func(req *http.Request) (*http.Response, error)
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return m.DoFunc(req)
}

func mockResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}
}

func TestGetCaptchaStage(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		body       string
		wantNil    bool
		wantErr    bool
		wantPK     string
		httpErr    error
	}{
		{
			name:   "found",
			status: 200,
			body:   `{"results":[{"pk":"abc-123","name":"turnstile"}]}`,
			wantPK: "abc-123",
		},
		{
			name:    "not found",
			status:  200,
			body:    `{"results":[]}`,
			wantNil: true,
		},
		{
			name:    "http error",
			status:  500,
			body:    `internal server error`,
			wantErr: true,
		},
		{
			name:    "request error",
			httpErr: io.ErrUnexpectedEOF,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{
				BaseURL: "http://localhost",
				Token:   "test-token",
				HTTP: &mockHTTPClient{
					DoFunc: func(req *http.Request) (*http.Response, error) {
						if tt.httpErr != nil {
							return nil, tt.httpErr
						}
						if !strings.Contains(req.URL.Path, "/api/v3/stages/captcha/") {
							t.Errorf("unexpected path: %s", req.URL.Path)
						}
						return mockResponse(tt.status, tt.body), nil
					},
				},
			}

			stage, err := client.GetCaptchaStage(context.Background(), "turnstile")
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantNil {
				if stage != nil {
					t.Fatalf("expected nil, got %+v", stage)
				}
				return
			}
			if stage.PK != tt.wantPK {
				t.Errorf("PK = %q, want %q", stage.PK, tt.wantPK)
			}
		})
	}
}

func TestCreateCaptchaStage(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		wantErr bool
		wantPK  string
	}{
		{
			name:   "created",
			status: 201,
			body:   `{"pk":"new-123","name":"turnstile"}`,
			wantPK: "new-123",
		},
		{
			name:    "server error",
			status:  500,
			body:    `internal server error`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{
				BaseURL: "http://localhost",
				Token:   "test-token",
				HTTP: &mockHTTPClient{
					DoFunc: func(req *http.Request) (*http.Response, error) {
						if req.Method != http.MethodPost {
							t.Errorf("expected POST, got %s", req.Method)
						}
						return mockResponse(tt.status, tt.body), nil
					},
				},
			}

			stage, err := client.CreateCaptchaStage(context.Background(), CreateCaptchaStageParams{
				Name:       "turnstile",
				PublicKey:  "pub-key",
				PrivateKey: "priv-key",
				JsURL:      "https://challenges.cloudflare.com/turnstile/v0/api.js",
				ApiURL:     "https://challenges.cloudflare.com/turnstile/v0/siteverify",
			})
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if stage.PK != tt.wantPK {
				t.Errorf("PK = %q, want %q", stage.PK, tt.wantPK)
			}
		})
	}
}

func TestGetUserWriteStage(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		wantErr bool
		wantPK  string
	}{
		{
			name:   "found",
			status: 200,
			body:   `{"pk":"write-1","name":"default-user-write","create_users_as_inactive":false}`,
			wantPK: "write-1",
		},
		{
			name:    "not found",
			status:  404,
			body:    `{"detail":"Not found."}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{
				BaseURL: "http://localhost",
				Token:   "test-token",
				HTTP: &mockHTTPClient{
					DoFunc: func(req *http.Request) (*http.Response, error) {
						return mockResponse(tt.status, tt.body), nil
					},
				},
			}

			stage, err := client.GetUserWriteStage(context.Background(), "write-1")
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if stage.PK != tt.wantPK {
				t.Errorf("PK = %q, want %q", stage.PK, tt.wantPK)
			}
		})
	}
}

func TestPatchUserWriteStage(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		wantErr bool
	}{
		{
			name:   "success",
			status: 200,
			body:   `{"pk":"write-1","name":"default-user-write","create_users_as_inactive":true}`,
		},
		{
			name:    "server error",
			status:  500,
			body:    `internal server error`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{
				BaseURL: "http://localhost",
				Token:   "test-token",
				HTTP: &mockHTTPClient{
					DoFunc: func(req *http.Request) (*http.Response, error) {
						if req.Method != http.MethodPatch {
							t.Errorf("expected PATCH, got %s", req.Method)
						}
						return mockResponse(tt.status, tt.body), nil
					},
				},
			}

			err := client.PatchUserWriteStage(context.Background(), "write-1", true)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
