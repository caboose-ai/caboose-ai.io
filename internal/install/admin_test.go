package install

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/caboose-ai/caboose-ai.io/internal/config"
	"github.com/caboose-ai/caboose-ai.io/internal/services/authentik"
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
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}
}

func TestGenerateAdminRecoveryLink(t *testing.T) {
	tests := []struct {
		name     string
		dryRun   bool
		httpFunc func(req *http.Request) (*http.Response, error)
		wantLink string
		wantErr  string
	}{
		{
			name:   "dry run returns dummy link",
			dryRun: true,
			// httpFunc not needed for dry run
			wantLink: "https://auth.example.com/if/flow/default-recovery-flow/?token=dry-run",
		},
		{
			name: "user found returns recovery link",
			httpFunc: func(req *http.Request) (*http.Response, error) {
				if strings.Contains(req.URL.Path, "/api/v3/core/users/") && req.Method == http.MethodGet {
					return httpResponse(200, `{"results":[{"pk":7,"username":"auth-admin"}]}`), nil
				}
				if strings.Contains(req.URL.Path, "/recovery/") && req.Method == http.MethodPost {
					return httpResponse(200, `{"link":"https://auth.example.com/recovery?token=abc123"}`), nil
				}
				return httpResponse(404, "not found"), nil
			},
			wantLink: "https://auth.example.com/recovery?token=abc123",
		},
		{
			name: "user not found returns error",
			httpFunc: func(req *http.Request) (*http.Response, error) {
				if strings.Contains(req.URL.Path, "/api/v3/core/users/") && req.Method == http.MethodGet {
					return httpResponse(200, `{"results":[]}`), nil
				}
				return httpResponse(404, "not found"), nil
			},
			wantErr: "auth-admin user not found",
		},
		{
			name: "FindUser API error",
			httpFunc: func(req *http.Request) (*http.Response, error) {
				return nil, fmt.Errorf("connection refused")
			},
			wantErr: "finding auth-admin user",
		},
		{
			name: "recovery link unavailable returns empty",
			httpFunc: func(req *http.Request) (*http.Response, error) {
				if strings.Contains(req.URL.Path, "/api/v3/core/users/") && req.Method == http.MethodGet {
					return httpResponse(200, `{"results":[{"pk":7,"username":"auth-admin"}]}`), nil
				}
				return httpResponse(400, `{"non_field_errors":"No recovery flow set."}`), nil
			},
			wantLink: "",
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

			if !tt.dryRun {
				inst.AK = authentik.NewClient("http://localhost", "test-token", &mockHTTP{DoFunc: tt.httpFunc})
			}

			var progressCalls []string
			progressFn := func(msg string) {
				progressCalls = append(progressCalls, msg)
			}

			link, err := inst.GenerateAdminRecoveryLink(context.Background(), progressFn)

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
			if link != tt.wantLink {
				t.Errorf("link = %q, want %q", link, tt.wantLink)
			}

			if tt.dryRun {
				if len(progressCalls) > 0 {
					t.Error("dry run should not emit progress messages")
				}
			} else {
				if len(progressCalls) == 0 {
					t.Error("expected at least one progress call")
				}
			}
		})
	}
}

func TestConfigureBrand(t *testing.T) {
	tests := []struct {
		name     string
		dryRun   bool
		httpFunc func(req *http.Request) (*http.Response, error)
		wantErr  string
	}{
		{
			name:   "dry run skips",
			dryRun: true,
		},
		{
			name: "success",
			httpFunc: func(req *http.Request) (*http.Response, error) {
				if strings.Contains(req.URL.Path, "/api/v3/flows/instances/") && req.Method == http.MethodGet {
					return httpResponse(200, `{"results":[{"pk":"flow-uuid-123","slug":"default-recovery-flow","name":"Default recovery flow","designation":"recovery"}]}`), nil
				}
				if strings.Contains(req.URL.Path, "/api/v3/core/brands/") && req.Method == http.MethodGet {
					return httpResponse(200, `{"results":[{"brand_uuid":"brand-uuid-456","domain":"example.com","default":true}]}`), nil
				}
				if strings.Contains(req.URL.Path, "/api/v3/core/brands/brand-uuid-456/") && req.Method == http.MethodPatch {
					return httpResponse(200, `{}`), nil
				}
				return httpResponse(404, "not found"), nil
			},
		},
		{
			name: "no recovery flow creates one",
			httpFunc: func(req *http.Request) (*http.Response, error) {
				if strings.Contains(req.URL.Path, "/api/v3/flows/instances/") && req.Method == http.MethodGet {
					if strings.Contains(req.URL.RawQuery, "designation=recovery") {
						return httpResponse(200, `{"results":[]}`), nil
					}
					if strings.Contains(req.URL.RawQuery, "slug=default-password-change") {
						return httpResponse(200, `{"results":[{"pk":"pwchange-uuid","slug":"default-password-change","name":"Change Password","designation":"stage_configuration"}]}`), nil
					}
				}
				if strings.Contains(req.URL.Path, "/api/v3/flows/instances/") && req.Method == http.MethodPost {
					return httpResponse(201, `{"pk":"new-flow-uuid","slug":"default-recovery-flow","name":"Default recovery flow","designation":"recovery"}`), nil
				}
				if strings.Contains(req.URL.Path, "/api/v3/flows/bindings/") && req.Method == http.MethodGet {
					return httpResponse(200, `{"results":[{"pk":"b1","stage_obj":{"pk":"stage-1","name":"prompt","component":"ak-stage-prompt-form"},"order":0},{"pk":"b2","stage_obj":{"pk":"stage-2","name":"write","component":"ak-stage-user-write-form"},"order":1}]}`), nil
				}
				if strings.Contains(req.URL.Path, "/api/v3/flows/bindings/") && req.Method == http.MethodPost {
					return httpResponse(201, `{}`), nil
				}
				if strings.Contains(req.URL.Path, "/api/v3/core/brands/") && req.Method == http.MethodGet {
					return httpResponse(200, `{"results":[{"brand_uuid":"brand-uuid-456","domain":"example.com","default":true}]}`), nil
				}
				if strings.Contains(req.URL.Path, "/api/v3/core/brands/brand-uuid-456/") && req.Method == http.MethodPatch {
					return httpResponse(200, `{}`), nil
				}
				return httpResponse(404, "not found"), nil
			},
		},
		{
			name: "create flow fails returns error",
			httpFunc: func(req *http.Request) (*http.Response, error) {
				if strings.Contains(req.URL.Path, "/api/v3/flows/instances/") && req.Method == http.MethodGet {
					return httpResponse(200, `{"results":[]}`), nil
				}
				if strings.Contains(req.URL.Path, "/api/v3/flows/instances/") && req.Method == http.MethodPost {
					return httpResponse(400, `{"slug":["flow with this slug already exists"]}`), nil
				}
				return httpResponse(404, "not found"), nil
			},
			wantErr: "ensuring recovery flow",
		},
		{
			name: "brand not found",
			httpFunc: func(req *http.Request) (*http.Response, error) {
				if strings.Contains(req.URL.Path, "/api/v3/flows/instances/") && req.Method == http.MethodGet {
					return httpResponse(200, `{"results":[{"pk":"flow-uuid-123","slug":"default-recovery-flow","name":"Default recovery flow","designation":"recovery"}]}`), nil
				}
				if strings.Contains(req.URL.Path, "/api/v3/core/brands/") && req.Method == http.MethodGet {
					return httpResponse(200, `{"results":[]}`), nil
				}
				return httpResponse(404, "not found"), nil
			},
			wantErr: "getting default brand",
		},
		{
			name: "patch fails",
			httpFunc: func(req *http.Request) (*http.Response, error) {
				if strings.Contains(req.URL.Path, "/api/v3/flows/instances/") && req.Method == http.MethodGet {
					return httpResponse(200, `{"results":[{"pk":"flow-uuid-123","slug":"default-recovery-flow","name":"Default recovery flow","designation":"recovery"}]}`), nil
				}
				if strings.Contains(req.URL.Path, "/api/v3/core/brands/") && req.Method == http.MethodGet {
					return httpResponse(200, `{"results":[{"brand_uuid":"brand-uuid-456","domain":"example.com","default":true}]}`), nil
				}
				if req.Method == http.MethodPatch {
					return httpResponse(500, "internal server error"), nil
				}
				return httpResponse(404, "not found"), nil
			},
			wantErr: "setting recovery flow",
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

			if !tt.dryRun {
				inst.AK = authentik.NewClient("http://localhost", "test-token", &mockHTTP{DoFunc: tt.httpFunc})
			}

			err := inst.ConfigureBrand(context.Background())

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
		})
	}
}
