package authentik

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestUpsertSourcePromotedOmitEmpty(t *testing.T) {
	tests := []struct {
		name          string
		promoted      *bool
		wantPromoted  bool
		wantFieldSent bool
	}{
		{
			name: "omitted",
		},
		{
			name:          "true",
			promoted:      boolPtr(true),
			wantPromoted:  true,
			wantFieldSent: true,
		},
		{
			name:          "false",
			promoted:      boolPtr(false),
			wantPromoted:  false,
			wantFieldSent: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var patchBody map[string]any
			requests := 0
			client := &Client{
				BaseURL: "http://localhost",
				Token:   "test-token",
				HTTP: &mockHTTPClient{
					DoFunc: func(req *http.Request) (*http.Response, error) {
						requests++
						switch requests {
						case 1:
							if req.Method != http.MethodGet {
								t.Fatalf("first request method = %s, want GET", req.Method)
							}
							return mockResponse(200, `{"results":[{"pk":"source-pk","slug":"github"}]}`), nil
						case 2:
							if req.Method != http.MethodPatch {
								t.Fatalf("second request method = %s, want PATCH", req.Method)
							}
							data, err := io.ReadAll(req.Body)
							if err != nil {
								t.Fatalf("reading PATCH body: %v", err)
							}
							if err := json.Unmarshal(data, &patchBody); err != nil {
								t.Fatalf("parsing PATCH body: %v", err)
							}
							return mockResponse(200, `{}`), nil
						default:
							t.Fatalf("unexpected extra request %d", requests)
							return nil, nil
						}
					},
				},
			}

			err := client.UpsertSource(context.Background(), UpsertSourceParams{
				Name:           "GitHub",
				Slug:           "github",
				Enabled:        true,
				Promoted:       tt.promoted,
				ProviderType:   "github",
				ConsumerKey:    "client-id",
				ConsumerSecret: "client-secret",
			})
			if err != nil {
				t.Fatalf("UpsertSource returned error: %v", err)
			}
			if requests != 2 {
				t.Fatalf("requests = %d, want 2", requests)
			}

			got, ok := patchBody["promoted"]
			if ok != tt.wantFieldSent {
				t.Fatalf("promoted field present = %v, want %v (body: %#v)", ok, tt.wantFieldSent, patchBody)
			}
			if !tt.wantFieldSent {
				return
			}
			if got != tt.wantPromoted {
				t.Fatalf("promoted = %#v, want %v", got, tt.wantPromoted)
			}
		})
	}
}

func boolPtr(v bool) *bool {
	return &v
}
