package authentik

import (
	"bytes"
	"context"
	"encoding/json"
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
		name    string
		status  int
		body    string
		wantNil bool
		wantErr bool
		wantPK  string
		httpErr error
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

func TestGetIdentificationStageIncludesLoginPresentation(t *testing.T) {
	client := &Client{
		BaseURL: "http://localhost",
		Token:   "test-token",
		HTTP: &mockHTTPClient{
			DoFunc: func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet {
					t.Fatalf("method = %s, want GET", req.Method)
				}
				if !strings.Contains(req.URL.Path, "/api/v3/stages/identification/") {
					t.Fatalf("unexpected path: %s", req.URL.Path)
				}
				return mockResponse(200, `{"results":[{"pk":"stage-1","name":"default-authentication-identification","sources":["google-pk"],"user_fields":[],"show_source_labels":true}]}`), nil
			},
		},
	}

	stage, err := client.GetIdentificationStage(context.Background(), "default-authentication-flow")
	if err != nil {
		t.Fatalf("GetIdentificationStage returned error: %v", err)
	}
	if stage == nil {
		t.Fatal("GetIdentificationStage returned nil stage")
	}
	if len(stage.UserFields) != 0 {
		t.Fatalf("UserFields = %#v, want empty", stage.UserFields)
	}
	if !stage.ShowSourceLabels {
		t.Fatal("ShowSourceLabels = false, want true")
	}
}

func TestSetIdentificationStageSourcesMakesLoginSourceOnly(t *testing.T) {
	var patchBody map[string]any
	client := &Client{
		BaseURL: "http://localhost",
		Token:   "test-token",
		HTTP: &mockHTTPClient{
			DoFunc: func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodPatch {
					t.Fatalf("method = %s, want PATCH", req.Method)
				}
				if !strings.Contains(req.URL.Path, "/api/v3/stages/identification/stage-1/") {
					t.Fatalf("unexpected path: %s", req.URL.Path)
				}
				data, err := io.ReadAll(req.Body)
				if err != nil {
					t.Fatalf("reading request body: %v", err)
				}
				if err := json.Unmarshal(data, &patchBody); err != nil {
					t.Fatalf("parsing request body: %v", err)
				}
				return mockResponse(200, `{}`), nil
			},
		},
	}

	if err := client.SetIdentificationStageSources(context.Background(), "stage-1", []string{"google-pk"}); err != nil {
		t.Fatalf("SetIdentificationStageSources returned error: %v", err)
	}

	if got, ok := patchBody["show_source_labels"].(bool); !ok || !got {
		t.Fatalf("show_source_labels = %#v, want true (body: %#v)", patchBody["show_source_labels"], patchBody)
	}
	fields, ok := patchBody["user_fields"].([]any)
	if !ok {
		t.Fatalf("user_fields = %#v, want array (body: %#v)", patchBody["user_fields"], patchBody)
	}
	if len(fields) != 0 {
		t.Fatalf("user_fields = %#v, want empty", fields)
	}
	sources, ok := patchBody["sources"].([]any)
	if !ok || len(sources) != 1 || sources[0] != "google-pk" {
		t.Fatalf("sources = %#v, want [google-pk]", patchBody["sources"])
	}
}

func TestPatchIdentificationStageCanRestoreLocalLogin(t *testing.T) {
	var patchBody map[string]any
	client := &Client{
		BaseURL: "http://localhost",
		Token:   "test-token",
		HTTP: &mockHTTPClient{
			DoFunc: func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodPatch {
					t.Fatalf("method = %s, want PATCH", req.Method)
				}
				data, err := io.ReadAll(req.Body)
				if err != nil {
					t.Fatalf("reading request body: %v", err)
				}
				if err := json.Unmarshal(data, &patchBody); err != nil {
					t.Fatalf("parsing request body: %v", err)
				}
				return mockResponse(200, `{}`), nil
			},
		},
	}

	err := client.PatchIdentificationStage(context.Background(), "stage-1", []string{}, []string{"username", "email"}, false)
	if err != nil {
		t.Fatalf("PatchIdentificationStage returned error: %v", err)
	}

	if got, ok := patchBody["show_source_labels"].(bool); !ok || got {
		t.Fatalf("show_source_labels = %#v, want false", patchBody["show_source_labels"])
	}
	fields, ok := patchBody["user_fields"].([]any)
	if !ok || len(fields) != 2 || fields[0] != "username" || fields[1] != "email" {
		t.Fatalf("user_fields = %#v, want [username email]", patchBody["user_fields"])
	}
	sources, ok := patchBody["sources"].([]any)
	if !ok || len(sources) != 0 {
		t.Fatalf("sources = %#v, want empty", patchBody["sources"])
	}
}

func TestPatchIdentificationStageSerializesNilSlicesAsArrays(t *testing.T) {
	var patchBody map[string]any
	client := &Client{
		BaseURL: "http://localhost",
		Token:   "test-token",
		HTTP: &mockHTTPClient{
			DoFunc: func(req *http.Request) (*http.Response, error) {
				data, err := io.ReadAll(req.Body)
				if err != nil {
					t.Fatalf("reading request body: %v", err)
				}
				if err := json.Unmarshal(data, &patchBody); err != nil {
					t.Fatalf("parsing request body: %v", err)
				}
				return mockResponse(200, `{}`), nil
			},
		},
	}

	err := client.PatchIdentificationStage(context.Background(), "stage-1", nil, nil, true)
	if err != nil {
		t.Fatalf("PatchIdentificationStage returned error: %v", err)
	}

	if sources, ok := patchBody["sources"].([]any); !ok || len(sources) != 0 {
		t.Fatalf("sources = %#v, want empty array", patchBody["sources"])
	}
	if fields, ok := patchBody["user_fields"].([]any); !ok || len(fields) != 0 {
		t.Fatalf("user_fields = %#v, want empty array", patchBody["user_fields"])
	}
}
