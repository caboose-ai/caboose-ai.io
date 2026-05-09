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
	"github.com/caboose-ai/caboose-ai.io/internal/docker"
	"github.com/caboose-ai/caboose-ai.io/internal/runner"
	"github.com/caboose-ai/caboose-ai.io/services/authentik"
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
		name       string
		dryRun     bool
		inspect    []byte
		inspectErr error
		exec       []byte
		execErr    error
		wantLink   string
		wantErr    string
	}{
		{
			name:     "dry run returns dummy link",
			dryRun:   true,
			wantLink: "https://auth.example.com/recovery/use-token/dry-run/",
		},
		{
			name:     "running container returns recovery link",
			inspect:  []byte("true\n"),
			exec:     []byte("Successfully created recovery link /recovery/use-token/abc123/\n"),
			wantLink: "https://auth.example.com/recovery/use-token/abc123/",
		},
		{
			name:    "stopped container returns action hint",
			inspect: []byte("false\n"),
			wantErr: "authentik container \"authentik-server\" is not running",
		},
		{
			name:       "missing container reports not running",
			inspectErr: fmt.Errorf("docker: exit status 1\nError: No such object: authentik-server"),
			wantErr:    "authentik container \"authentik-server\" is not running",
		},
		{
			name:    "unparseable recovery output returns error",
			inspect: []byte("true\n"),
			exec:    []byte("unexpected output"),
			wantErr: "could not parse recovery token",
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
				r := runner.NewMockRunner()
				r.On("docker inspect --format {{.State.Running}} authentik-server", tt.inspect, tt.inspectErr)
				r.On("docker exec authentik-server ak create_recovery_key 10 auth-admin", tt.exec, tt.execErr)
				inst.DockerExec = docker.NewExecClient(r)
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

func TestIsAuthentikTokenError(t *testing.T) {
	for _, errText := range []string{
		`POST /api/v3/flows/instances/ returned HTTP 403: {"detail":"Token invalid/expired"}`,
		"GET /api/v3/core/users/ returned HTTP 401: unauthorized",
	} {
		if !IsAuthentikTokenError(fmt.Errorf("%s", errText)) {
			t.Fatalf("IsAuthentikTokenError(%q) = false, want true", errText)
		}
	}

	if IsAuthentikTokenError(fmt.Errorf("connection refused")) {
		t.Fatal("IsAuthentikTokenError returned true for unrelated error")
	}
}

func TestRecoverAuthentikBootstrapTokenStoresAndInitializesClient(t *testing.T) {
	r := runner.NewMockRunner()
	r.OnFunc("docker exec authentik-server sh -c", func(args []string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if !strings.Contains(joined, "authentik-bootstrap-token") {
			return nil, fmt.Errorf("expected bootstrap token recovery script, got %v", args)
		}
		if strings.Contains(joined, "create_superuser") {
			return nil, fmt.Errorf("recovery script should not use Django create_superuser, got %v", args)
		}
		if !strings.Contains(joined, "Group.objects.get_or_create") || !strings.Contains(joined, "is_superuser") {
			return nil, fmt.Errorf("expected recovery script to assign a superuser group, got %v", args)
		}
		if strings.Contains(joined, "u.is_staff =") || strings.Contains(joined, "u.is_superuser =") {
			return nil, fmt.Errorf("recovery script should not assign Authentik read-only permission properties, got %v", args)
		}
		if !strings.Contains(joined, "/lifecycle/ak shell") {
			return nil, fmt.Errorf("expected Authentik lifecycle shell invocation, got %v", args)
		}
		if !strings.Contains(joined, "/ak-root/.venv/bin") {
			return nil, fmt.Errorf("expected Authentik virtualenv on PATH, got %v", args)
		}
		return []byte(`{"token":"recovered-token"}` + "\n"), nil
	})

	cfg := config.DefaultConfig()
	cfg.Domain = "example.com"
	inst := &Installer{
		Config:     cfg,
		State:      NewState(),
		Secrets:    &mockSecrets{store: map[string]string{}},
		HTTP:       &mockHTTP{DoFunc: func(req *http.Request) (*http.Response, error) { return httpResponse(200, "{}"), nil }},
		DockerExec: docker.NewExecClient(r),
	}

	if err := inst.RecoverAuthentikBootstrapToken(context.Background()); err != nil {
		t.Fatalf("RecoverAuthentikBootstrapToken: %v", err)
	}
	got, err := inst.Secrets.Get(context.Background(), "AUTHENTIK_BOOTSTRAP_TOKEN")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "recovered-token" {
		t.Fatalf("stored token = %q, want recovered-token", got)
	}
	if inst.AK == nil || inst.AK.Token != "recovered-token" {
		t.Fatalf("AK token = %#v, want recovered-token", inst.AK)
	}
}

func TestRecoverAuthentikBootstrapTokenReturnsCleanContainerError(t *testing.T) {
	r := runner.NewMockRunner()
	r.On("docker exec authentik-server sh -c", []byte(`{"error":"no active Authentik user found for bootstrap token recovery"}`+"\n"), nil)

	cfg := config.DefaultConfig()
	cfg.Domain = "example.com"
	inst := &Installer{
		Config:     cfg,
		State:      NewState(),
		Secrets:    &mockSecrets{store: map[string]string{}},
		DockerExec: docker.NewExecClient(r),
	}

	err := inst.RecoverAuthentikBootstrapToken(context.Background())
	if err == nil {
		t.Fatal("RecoverAuthentikBootstrapToken returned nil, want error")
	}
	if !strings.Contains(err.Error(), "no active Authentik user found") {
		t.Fatalf("error = %q, want clean recovery reason", err.Error())
	}
	if strings.Contains(err.Error(), "Imported related module") {
		t.Fatalf("error includes Authentik startup log spam: %q", err.Error())
	}
}

func TestConfigureBrand(t *testing.T) {
	oldWait := authentikDefaultFlowWait
	oldPoll := authentikDefaultFlowPoll
	oldBrandWait := authentikDefaultBrandWait
	oldBrandPoll := authentikDefaultBrandPoll
	authentikDefaultFlowWait = 20_000_000
	authentikDefaultFlowPoll = 1_000_000
	authentikDefaultBrandWait = 20_000_000
	authentikDefaultBrandPoll = 1_000_000
	t.Cleanup(func() {
		authentikDefaultFlowWait = oldWait
		authentikDefaultFlowPoll = oldPoll
		authentikDefaultBrandWait = oldBrandWait
		authentikDefaultBrandPoll = oldBrandPoll
	})

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
					if strings.Contains(req.URL.RawQuery, "slug=default-recovery-flow") {
						return httpResponse(200, `{"results":[{"pk":"flow-uuid-123","slug":"default-recovery-flow","name":"Default recovery flow","designation":"recovery"}]}`), nil
					}
					if strings.Contains(req.URL.RawQuery, "slug=default-password-change") {
						return httpResponse(200, `{"results":[{"pk":"pwchange-uuid","slug":"default-password-change","name":"Change Password","designation":"stage_configuration"}]}`), nil
					}
				}
				if strings.Contains(req.URL.Path, "/api/v3/flows/bindings/") && req.Method == http.MethodGet {
					return httpResponse(200, `{"results":[]}`), nil
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
			name: "password change flow becomes available after blueprint lag",
			httpFunc: func() func(req *http.Request) (*http.Response, error) {
				passwordChangeLookups := 0
				return func(req *http.Request) (*http.Response, error) {
					if strings.Contains(req.URL.Path, "/api/v3/flows/instances/") && req.Method == http.MethodGet {
						if strings.Contains(req.URL.RawQuery, "slug=default-recovery-flow") {
							return httpResponse(200, `{"results":[{"pk":"flow-uuid-123","slug":"default-recovery-flow","name":"Default recovery flow","designation":"recovery"}]}`), nil
						}
						if strings.Contains(req.URL.RawQuery, "slug=default-password-change") {
							passwordChangeLookups++
							if passwordChangeLookups == 1 {
								return httpResponse(200, `{"results":[]}`), nil
							}
							return httpResponse(200, `{"results":[{"pk":"pwchange-uuid","slug":"default-password-change","name":"Change Password","designation":"stage_configuration"}]}`), nil
						}
					}
					if strings.Contains(req.URL.Path, "/api/v3/flows/bindings/") && req.Method == http.MethodGet {
						return httpResponse(200, `{"results":[]}`), nil
					}
					if strings.Contains(req.URL.Path, "/api/v3/core/brands/") && req.Method == http.MethodGet {
						return httpResponse(200, `{"results":[{"brand_uuid":"brand-uuid-456","domain":"example.com","default":true}]}`), nil
					}
					if strings.Contains(req.URL.Path, "/api/v3/core/brands/brand-uuid-456/") && req.Method == http.MethodPatch {
						return httpResponse(200, `{}`), nil
					}
					return httpResponse(404, "not found"), nil
				}
			}(),
		},
		{
			name: "default brand becomes available after blueprint lag",
			httpFunc: func() func(req *http.Request) (*http.Response, error) {
				brandLookups := 0
				return func(req *http.Request) (*http.Response, error) {
					if strings.Contains(req.URL.Path, "/api/v3/flows/instances/") && req.Method == http.MethodGet {
						if strings.Contains(req.URL.RawQuery, "slug=default-recovery-flow") {
							return httpResponse(200, `{"results":[{"pk":"flow-uuid-123","slug":"default-recovery-flow","name":"Default recovery flow","designation":"recovery"}]}`), nil
						}
						if strings.Contains(req.URL.RawQuery, "slug=default-password-change") {
							return httpResponse(200, `{"results":[{"pk":"pwchange-uuid","slug":"default-password-change","name":"Change Password","designation":"stage_configuration"}]}`), nil
						}
					}
					if strings.Contains(req.URL.Path, "/api/v3/flows/bindings/") && req.Method == http.MethodGet {
						return httpResponse(200, `{"results":[]}`), nil
					}
					if strings.Contains(req.URL.Path, "/api/v3/core/brands/") && req.Method == http.MethodGet {
						brandLookups++
						if brandLookups == 1 {
							return httpResponse(200, `{"results":[]}`), nil
						}
						return httpResponse(200, `{"results":[{"brand_uuid":"brand-uuid-456","domain":"example.com","default":true}]}`), nil
					}
					if strings.Contains(req.URL.Path, "/api/v3/core/brands/brand-uuid-456/") && req.Method == http.MethodPatch {
						return httpResponse(200, `{}`), nil
					}
					return httpResponse(404, "not found"), nil
				}
			}(),
		},
		{
			name: "no recovery flow creates one",
			httpFunc: func(req *http.Request) (*http.Response, error) {
				if strings.Contains(req.URL.Path, "/api/v3/flows/instances/") && req.Method == http.MethodGet {
					if strings.Contains(req.URL.RawQuery, "slug=default-recovery-flow") {
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
			name: "duplicate recovery binding is ignored",
			httpFunc: func(req *http.Request) (*http.Response, error) {
				if strings.Contains(req.URL.Path, "/api/v3/flows/instances/") && req.Method == http.MethodGet {
					if strings.Contains(req.URL.RawQuery, "slug=default-recovery-flow") {
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
					if strings.Contains(req.URL.RawQuery, "flow_slug=default-recovery-flow") {
						return httpResponse(200, `{"results":[]}`), nil
					}
					return httpResponse(200, `{"results":[{"pk":"b1","stage_obj":{"pk":"stage-1","name":"prompt","component":"ak-stage-prompt-form"},"order":0}]}`), nil
				}
				if strings.Contains(req.URL.Path, "/api/v3/flows/bindings/") && req.Method == http.MethodPost {
					return httpResponse(400, `{"non_field_errors":["The fields target, stage, order must make a unique set."]}`), nil
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
					if strings.Contains(req.URL.RawQuery, "slug=default-recovery-flow") {
						return httpResponse(200, `{"results":[{"pk":"flow-uuid-123","slug":"default-recovery-flow","name":"Default recovery flow","designation":"recovery"}]}`), nil
					}
					if strings.Contains(req.URL.RawQuery, "slug=default-password-change") {
						return httpResponse(200, `{"results":[{"pk":"pwchange-uuid","slug":"default-password-change","name":"Change Password","designation":"stage_configuration"}]}`), nil
					}
				}
				if strings.Contains(req.URL.Path, "/api/v3/flows/bindings/") && req.Method == http.MethodGet {
					return httpResponse(200, `{"results":[]}`), nil
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
					if strings.Contains(req.URL.RawQuery, "slug=default-recovery-flow") {
						return httpResponse(200, `{"results":[{"pk":"flow-uuid-123","slug":"default-recovery-flow","name":"Default recovery flow","designation":"recovery"}]}`), nil
					}
					if strings.Contains(req.URL.RawQuery, "slug=default-password-change") {
						return httpResponse(200, `{"results":[{"pk":"pwchange-uuid","slug":"default-password-change","name":"Change Password","designation":"stage_configuration"}]}`), nil
					}
				}
				if strings.Contains(req.URL.Path, "/api/v3/flows/bindings/") && req.Method == http.MethodGet {
					return httpResponse(200, `{"results":[]}`), nil
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
