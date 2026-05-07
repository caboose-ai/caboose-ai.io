package install

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/caboose-ai/caboose-ai.io/internal/config"
	"github.com/caboose-ai/caboose-ai.io/internal/secrets"
	"github.com/caboose-ai/caboose-ai.io/internal/services/authentik"
)

// mockSecrets implements secrets.SecretStore for testing.
type mockSecrets struct {
	store map[string]string
}

func (m *mockSecrets) Get(_ context.Context, key string) (string, error) {
	return m.store[key], nil
}
func (m *mockSecrets) Put(_ context.Context, key, value string) error {
	m.store[key] = value
	return nil
}
func (m *mockSecrets) Generate(_ context.Context, key string, length int, _ secrets.GenerateOpts) (string, error) {
	return "", nil
}
func (m *mockSecrets) Delete(_ context.Context, key string) error {
	delete(m.store, key)
	return nil
}
func (m *mockSecrets) EnsureVault(_ context.Context) error { return nil }

func TestSetupCaptcha(t *testing.T) {
	tests := []struct {
		name      string
		dryRun    bool
		secrets   map[string]string
		httpFunc  func(req *http.Request) (*http.Response, error)
		wantErr   string
		wantCalls int // minimum number of progress calls expected
		noCalls   bool
	}{
		{
			name:    "no creds skips everything",
			secrets: map[string]string{},
			httpFunc: func(req *http.Request) (*http.Response, error) {
				return nil, fmt.Errorf("should not be called")
			},
			noCalls: true,
		},
		{
			name:    "missing secret key skips",
			secrets: map[string]string{"TURNSTILE_SITE_KEY": "site123"},
			httpFunc: func(req *http.Request) (*http.Response, error) {
				return nil, fmt.Errorf("should not be called")
			},
			noCalls: true,
		},
		{
			name:   "dry run skips everything",
			dryRun: true,
			secrets: map[string]string{
				"TURNSTILE_SITE_KEY":   "site123",
				"TURNSTILE_SECRET_KEY": "secret456",
			},
			noCalls: true,
		},
		{
			name: "stage already exists, binding already exists",
			secrets: map[string]string{
				"TURNSTILE_SITE_KEY":   "site123",
				"TURNSTILE_SECRET_KEY": "secret456",
			},
			httpFunc: func(req *http.Request) (*http.Response, error) {
				// GetCaptchaStage — stage exists
				if strings.Contains(req.URL.Path, "/api/v3/stages/captcha/") && req.Method == http.MethodGet {
					return httpResponse(200, `{"results":[{"pk":"stage-pk-1","name":"turnstile-captcha"}]}`), nil
				}
				// GetFlow
				if strings.Contains(req.URL.Path, "/api/v3/flows/instances/") && req.Method == http.MethodGet {
					return httpResponse(200, `{"results":[{"pk":"flow-pk-1","slug":"default-authentication-flow","name":"Auth Flow"}]}`), nil
				}
				// GetFlowStageBinding — binding exists
				if strings.Contains(req.URL.Path, "/api/v3/flows/bindings/") && req.Method == http.MethodGet {
					return httpResponse(200, `{"results":[{"pk":"bind-pk-1","target":"flow-pk-1","stage":"stage-pk-1","stage_obj":{"pk":"stage-pk-1","name":"turnstile-captcha","component":"ak-stage-captcha"},"order":0}]}`), nil
				}
				return httpResponse(404, "not found"), nil
			},
			wantCalls: 2,
		},
		{
			name: "full creation flow",
			secrets: map[string]string{
				"TURNSTILE_SITE_KEY":   "site123",
				"TURNSTILE_SECRET_KEY": "secret456",
			},
			httpFunc: func(req *http.Request) (*http.Response, error) {
				// GetCaptchaStage — not found
				if strings.Contains(req.URL.Path, "/api/v3/stages/captcha/") && req.Method == http.MethodGet {
					return httpResponse(200, `{"results":[]}`), nil
				}
				// CreateCaptchaStage
				if strings.Contains(req.URL.Path, "/api/v3/stages/captcha/") && req.Method == http.MethodPost {
					return httpResponse(201, `{"pk":"new-stage-pk","name":"turnstile-captcha"}`), nil
				}
				// GetFlow
				if strings.Contains(req.URL.Path, "/api/v3/flows/instances/") && req.Method == http.MethodGet {
					return httpResponse(200, `{"results":[{"pk":"flow-pk-1","slug":"default-authentication-flow","name":"Auth Flow"}]}`), nil
				}
				// GetFlowStageBinding — not found
				if strings.Contains(req.URL.Path, "/api/v3/flows/bindings/") && req.Method == http.MethodGet {
					return httpResponse(200, `{"results":[]}`), nil
				}
				// CreateFlowStageBinding
				if strings.Contains(req.URL.Path, "/api/v3/flows/bindings/") && req.Method == http.MethodPost {
					return httpResponse(201, `{"pk":"bind-pk-1"}`), nil
				}
				return httpResponse(404, "not found"), nil
			},
			wantCalls: 2,
		},
		{
			name: "stage exists but binding missing creates binding",
			secrets: map[string]string{
				"TURNSTILE_SITE_KEY":   "site123",
				"TURNSTILE_SECRET_KEY": "secret456",
			},
			httpFunc: func(req *http.Request) (*http.Response, error) {
				// GetCaptchaStage — exists
				if strings.Contains(req.URL.Path, "/api/v3/stages/captcha/") && req.Method == http.MethodGet {
					return httpResponse(200, `{"results":[{"pk":"stage-pk-1","name":"turnstile-captcha"}]}`), nil
				}
				// GetFlow
				if strings.Contains(req.URL.Path, "/api/v3/flows/instances/") && req.Method == http.MethodGet {
					return httpResponse(200, `{"results":[{"pk":"flow-pk-1","slug":"default-authentication-flow","name":"Auth Flow"}]}`), nil
				}
				// GetFlowStageBinding — not found
				if strings.Contains(req.URL.Path, "/api/v3/flows/bindings/") && req.Method == http.MethodGet {
					return httpResponse(200, `{"results":[]}`), nil
				}
				// CreateFlowStageBinding
				if strings.Contains(req.URL.Path, "/api/v3/flows/bindings/") && req.Method == http.MethodPost {
					return httpResponse(201, `{"pk":"bind-pk-1"}`), nil
				}
				return httpResponse(404, "not found"), nil
			},
			wantCalls: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.Domain = "example.com"

			ms := &mockSecrets{store: tt.secrets}

			inst := &Installer{
				Config:  cfg,
				State:   NewState(),
				Secrets: ms,
			}
			inst.State.DryRun = tt.dryRun

			if tt.httpFunc != nil && !tt.dryRun {
				inst.AK = authentik.NewClient("http://localhost", "test-token", &mockHTTP{DoFunc: tt.httpFunc})
			}

			var progressCalls []string
			progressFn := func(msg string) {
				progressCalls = append(progressCalls, msg)
			}

			err := inst.SetupCaptcha(context.Background(), progressFn)

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

			if tt.noCalls {
				if len(progressCalls) > 0 {
					t.Errorf("expected no progress calls, got %v", progressCalls)
				}
			} else if len(progressCalls) < tt.wantCalls {
				t.Errorf("expected at least %d progress calls, got %d: %v", tt.wantCalls, len(progressCalls), progressCalls)
			}
		})
	}
}

func TestSetupInactiveEnrollment(t *testing.T) {
	oldWait := authentikDefaultFlowWait
	oldPoll := authentikDefaultFlowPoll
	authentikDefaultFlowWait = 20_000_000
	authentikDefaultFlowPoll = 1_000_000
	t.Cleanup(func() {
		authentikDefaultFlowWait = oldWait
		authentikDefaultFlowPoll = oldPoll
	})

	tests := []struct {
		name     string
		dryRun   bool
		httpFunc func(req *http.Request) (*http.Response, error)
		wantErr  string
		noCalls  bool
	}{
		{
			name:    "dry run skips everything",
			dryRun:  true,
			noCalls: true,
		},
		{
			name: "finds user-write stage and patches",
			httpFunc: func(req *http.Request) (*http.Response, error) {
				if strings.Contains(req.URL.Path, "/api/v3/flows/instances/") && req.Method == http.MethodGet {
					return httpResponse(200, `{"results":[{"pk":"enrollment-flow-pk","slug":"default-source-enrollment","name":"Enrollment"}]}`), nil
				}
				// ListFlowStageBindings
				if strings.Contains(req.URL.Path, "/api/v3/flows/bindings/") && req.Method == http.MethodGet {
					return httpResponse(200, `{"results":[
						{"pk":"b1","target":"enrollment-flow-pk","stage":"s1","stage_obj":{"pk":"s1","name":"prompt","component":"ak-stage-prompt"},"order":0},
						{"pk":"b2","target":"enrollment-flow-pk","stage":"s2","stage_obj":{"pk":"s2","name":"enrollment-write","component":"ak-stage-user-write-form"},"order":1}
					]}`), nil
				}
				// PatchUserWriteStage
				if strings.Contains(req.URL.Path, "/api/v3/stages/user_write/s2/") && req.Method == http.MethodPatch {
					return httpResponse(200, `{"pk":"s2","name":"enrollment-write","create_users_as_inactive":true}`), nil
				}
				return httpResponse(404, "not found"), nil
			},
		},
		{
			name: "enrollment flow appears after blueprint lag",
			httpFunc: func() func(req *http.Request) (*http.Response, error) {
				flowLookups := 0
				return func(req *http.Request) (*http.Response, error) {
					if strings.Contains(req.URL.Path, "/api/v3/flows/instances/") && req.Method == http.MethodGet {
						flowLookups++
						if flowLookups == 1 {
							return httpResponse(200, `{"results":[]}`), nil
						}
						return httpResponse(200, `{"results":[{"pk":"enrollment-flow-pk","slug":"default-source-enrollment","name":"Enrollment"}]}`), nil
					}
					if strings.Contains(req.URL.Path, "/api/v3/flows/bindings/") && req.Method == http.MethodGet {
						return httpResponse(200, `{"results":[
							{"pk":"b1","target":"enrollment-flow-pk","stage":"s1","stage_obj":{"pk":"s1","name":"prompt","component":"ak-stage-prompt"},"order":0},
							{"pk":"b2","target":"enrollment-flow-pk","stage":"s2","stage_obj":{"pk":"s2","name":"enrollment-write","component":"ak-stage-user-write-form"},"order":1}
						]}`), nil
					}
					if strings.Contains(req.URL.Path, "/api/v3/stages/user_write/s2/") && req.Method == http.MethodPatch {
						return httpResponse(200, `{"pk":"s2","name":"enrollment-write","create_users_as_inactive":true}`), nil
					}
					return httpResponse(404, "not found"), nil
				}
			}(),
		},
		{
			name: "no user-write stage returns error",
			httpFunc: func(req *http.Request) (*http.Response, error) {
				if strings.Contains(req.URL.Path, "/api/v3/flows/instances/") && req.Method == http.MethodGet {
					return httpResponse(200, `{"results":[{"pk":"enrollment-flow-pk","slug":"default-source-enrollment","name":"Enrollment"}]}`), nil
				}
				if strings.Contains(req.URL.Path, "/api/v3/flows/bindings/") && req.Method == http.MethodGet {
					return httpResponse(200, `{"results":[
						{"pk":"b1","target":"enrollment-flow-pk","stage":"s1","stage_obj":{"pk":"s1","name":"prompt","component":"ak-stage-prompt"},"order":0}
					]}`), nil
				}
				return httpResponse(404, "not found"), nil
			},
			wantErr: "user-write stage not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.Domain = "example.com"

			inst := &Installer{
				Config: cfg,
				State:  NewState(),
			}
			inst.State.DryRun = tt.dryRun

			if tt.httpFunc != nil && !tt.dryRun {
				inst.AK = authentik.NewClient("http://localhost", "test-token", &mockHTTP{DoFunc: tt.httpFunc})
			}

			var progressCalls []string
			progressFn := func(msg string) {
				progressCalls = append(progressCalls, msg)
			}

			err := inst.SetupInactiveEnrollment(context.Background(), progressFn)

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

			if tt.noCalls {
				if len(progressCalls) > 0 {
					t.Errorf("expected no progress calls, got %v", progressCalls)
				}
			} else if len(progressCalls) == 0 {
				t.Error("expected at least one progress call")
			}
		})
	}
}
