package authentik

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestListFlowStageBindings(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		body      string
		wantErr   bool
		wantCount int
	}{
		{
			name:      "found bindings",
			status:    200,
			body:      `{"results":[{"pk":"b1","target":"flow-pk","stage":"s1","stage_obj":{"pk":"s1","name":"stage-1","component":"ak-stage"},"order":0},{"pk":"b2","target":"other-flow-pk","stage":"s2","stage_obj":{"pk":"s2","name":"stage-2","component":"ak-stage"},"order":1},{"pk":"b3","target":"flow-pk","stage":"s3","stage_obj":{"pk":"s3","name":"stage-3","component":"ak-stage"},"order":2}]}`,
			wantCount: 2,
		},
		{
			name:      "empty results",
			status:    200,
			body:      `{"results":[]}`,
			wantCount: 0,
		},
		{
			name:    "http error",
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
						if strings.Contains(req.URL.Path, "/api/v3/flows/instances/") {
							return mockResponse(200, `{"results":[{"pk":"flow-pk","slug":"enrollment","name":"Enrollment","designation":"enrollment"}]}`), nil
						}
						if !strings.Contains(req.URL.String(), "flow_slug=enrollment") {
							t.Errorf("expected flow_slug query param, got %s", req.URL.String())
						}
						return mockResponse(tt.status, tt.body), nil
					},
				},
			}

			bindings, err := client.ListFlowStageBindings(context.Background(), "enrollment")
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(bindings) != tt.wantCount {
				t.Errorf("got %d bindings, want %d", len(bindings), tt.wantCount)
			}
		})
	}
}

func TestGetFlowStageBinding(t *testing.T) {
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
			body:   `{"results":[{"pk":"wrong-flow","target":"other-flow-pk","stage":"s1","stage_obj":{"pk":"s1","name":"captcha","component":"ak-stage-captcha"},"order":5},{"pk":"wrong-stage","target":"flow-pk","stage":"s2","stage_obj":{"pk":"s2","name":"other","component":"ak-stage"},"order":6},{"pk":"b1","target":"flow-pk","stage":"s1","stage_obj":{"pk":"s1","name":"captcha","component":"ak-stage-captcha"},"order":5}]}`,
			wantPK: "b1",
		},
		{
			name:    "not found",
			status:  200,
			body:    `{"results":[]}`,
			wantNil: true,
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
						if strings.Contains(req.URL.Path, "/api/v3/flows/instances/") {
							return mockResponse(200, `{"results":[{"pk":"flow-pk","slug":"enrollment","name":"Enrollment","designation":"enrollment"}]}`), nil
						}
						return mockResponse(tt.status, tt.body), nil
					},
				},
			}

			binding, err := client.GetFlowStageBinding(context.Background(), "enrollment", "s1")
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
				if binding != nil {
					t.Fatalf("expected nil, got %+v", binding)
				}
				return
			}
			if binding.PK != tt.wantPK {
				t.Errorf("PK = %q, want %q", binding.PK, tt.wantPK)
			}
		})
	}
}

func TestCreateFlowStageBinding(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		wantErr bool
	}{
		{
			name:   "created",
			status: 201,
			body:   `{"pk":"new-b1","target":"flow-pk","stage":"stage-pk","order":10}`,
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

			err := client.CreateFlowStageBinding(context.Background(), "flow-pk", "stage-pk", 10)
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
