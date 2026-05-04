package authentik

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestGetDefaultBrand(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		body      string
		wantErr   bool
		wantUUID  string
		httpErr   error
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
