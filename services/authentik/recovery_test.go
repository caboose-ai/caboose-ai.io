package authentik

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestGenerateRecoveryLink(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		body     string
		wantErr  bool
		wantLink string
		httpErr  error
	}{
		{
			name:     "success",
			status:   200,
			body:     `{"link":"https://auth.example.com/recovery?token=abc123"}`,
			wantLink: "https://auth.example.com/recovery?token=abc123",
		},
		{
			name:    "server error",
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
						if req.Method != http.MethodPost {
							t.Errorf("expected POST, got %s", req.Method)
						}
						if !strings.Contains(req.URL.Path, "/api/v3/core/users/42/recovery/") {
							t.Errorf("unexpected path: %s", req.URL.Path)
						}
						return mockResponse(tt.status, tt.body), nil
					},
				},
			}

			link, err := client.GenerateRecoveryLink(context.Background(), 42)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if link != tt.wantLink {
				t.Errorf("link = %q, want %q", link, tt.wantLink)
			}
		})
	}
}
