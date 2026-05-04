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
			name: "GenerateRecoveryLink API error",
			httpFunc: func(req *http.Request) (*http.Response, error) {
				if strings.Contains(req.URL.Path, "/api/v3/core/users/") && req.Method == http.MethodGet {
					return httpResponse(200, `{"results":[{"pk":7,"username":"auth-admin"}]}`), nil
				}
				return httpResponse(500, "internal server error"), nil
			},
			wantErr: "generating recovery link",
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
