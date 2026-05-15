package authentik

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestGetDefaultBrand(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		body     string
		wantErr  bool
		wantUUID string
		httpErr  error
	}{
		{
			name:     "success",
			status:   200,
			body:     `{"results":[{"brand_uuid":"abc-123","domain":"example.com","default":true}]}`,
			wantUUID: "abc-123",
		},
		{
			name:    "no default brand",
			status:  200,
			body:    `{"results":[]}`,
			wantErr: true,
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
						if tt.httpErr != nil {
							return nil, tt.httpErr
						}
						if req.Method != http.MethodGet {
							t.Errorf("expected GET, got %s", req.Method)
						}
						if !strings.Contains(req.URL.Path, "/api/v3/core/brands/") {
							t.Errorf("unexpected path: %s", req.URL.Path)
						}
						return mockResponse(tt.status, tt.body), nil
					},
				},
			}

			brand, err := client.GetDefaultBrand(context.Background())
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if brand.BrandUUID != tt.wantUUID {
				t.Errorf("brand_uuid = %q, want %q", brand.BrandUUID, tt.wantUUID)
			}
		})
	}
}

func TestSetBrandCustomCSS(t *testing.T) {
	var body string
	client := &Client{
		BaseURL: "http://localhost",
		Token:   "test-token",
		HTTP: &mockHTTPClient{
			DoFunc: func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodPatch {
					t.Fatalf("method = %s, want PATCH", req.Method)
				}
				if !strings.Contains(req.URL.Path, "/api/v3/core/brands/abc-123/") {
					t.Fatalf("unexpected path: %s", req.URL.Path)
				}
				data, err := io.ReadAll(req.Body)
				if err != nil {
					t.Fatalf("reading request body: %v", err)
				}
				body = string(data)
				return mockResponse(200, `{}`), nil
			},
		},
	}

	if err := client.SetBrandCustomCSS(context.Background(), "abc-123", "button { color: black; }"); err != nil {
		t.Fatalf("SetBrandCustomCSS returned error: %v", err)
	}
	if !strings.Contains(body, `"branding_custom_css":"button { color: black; }"`) {
		t.Fatalf("PATCH body = %s, want branding_custom_css", body)
	}
}

func TestEnsureGoogleSignInBrandCSS(t *testing.T) {
	t.Run("appends block", func(t *testing.T) {
		got := EnsureGoogleSignInBrandCSS("body { color: black; }\n")
		if !strings.Contains(got, "body { color: black; }") {
			t.Fatalf("existing CSS missing from %q", got)
		}
		if !strings.Contains(got, googleSignInCSSStart) || !strings.Contains(got, googleSignInCSSEnd) {
			t.Fatalf("Google CSS markers missing from %q", got)
		}
	})

	t.Run("replaces existing block", func(t *testing.T) {
		existing := "a {}\n\n" + googleSignInCSSStart + "\nold\n" + googleSignInCSSEnd + "\n\nb {}\n"
		got := EnsureGoogleSignInBrandCSS(existing)
		if strings.Contains(got, "\nold\n") {
			t.Fatalf("old block was not replaced: %q", got)
		}
		if strings.Count(got, googleSignInCSSStart) != 1 {
			t.Fatalf("marker count in %q, want 1", got)
		}
		if !strings.Contains(got, "a {}") || !strings.Contains(got, "b {}") {
			t.Fatalf("surrounding CSS was not preserved: %q", got)
		}
	})
}

func TestSetBrandRecoveryFlow(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		wantErr bool
		httpErr error
	}{
		{
			name:   "success",
			status: 200,
			body:   `{"brand_uuid":"abc-123","recovery_flow":"flow-uuid-456"}`,
		},
		{
			name:    "server error",
			status:  500,
			body:    `internal server error`,
			wantErr: true,
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
						if tt.httpErr != nil {
							return nil, tt.httpErr
						}
						if req.Method != http.MethodPatch {
							t.Errorf("expected PATCH, got %s", req.Method)
						}
						if !strings.Contains(req.URL.Path, "/api/v3/core/brands/abc-123/") {
							t.Errorf("unexpected path: %s", req.URL.Path)
						}
						return mockResponse(tt.status, tt.body), nil
					},
				},
			}

			err := client.SetBrandRecoveryFlow(context.Background(), "abc-123", "flow-uuid-456")
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
