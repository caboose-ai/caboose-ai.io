package install

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/caboose-ai/caboose-ai.io/internal/config"
	"github.com/caboose-ai/caboose-ai.io/internal/services/authentik"
)

func TestEnsureProviderFlowsCreatesMissingDefaults(t *testing.T) {
	var created []string
	httpClient := &mockHTTP{DoFunc: func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/api/v3/flows/instances/") {
			return httpResponse(200, `{"results":[]}`), nil
		}
		if req.Method == http.MethodPost && strings.Contains(req.URL.Path, "/api/v3/flows/instances/") {
			switch {
			case strings.Contains(req.URL.RawQuery, "unused"):
				t.Fatal("unexpected query on create")
			case strings.Contains(readRequestBody(t, req), "default-provider-authorization-implicit-consent"):
				created = append(created, "authorization")
				return httpResponse(201, `{"pk":"auth-flow-pk","slug":"default-provider-authorization-implicit-consent","name":"Provider authorization implicit consent","designation":"authorization"}`), nil
			default:
				created = append(created, "invalidation")
				return httpResponse(201, `{"pk":"invalidation-flow-pk","slug":"default-provider-invalidation-flow","name":"Provider invalidation flow","designation":"invalidation"}`), nil
			}
		}
		return httpResponse(404, "not found"), nil
	}}
	inst := &Installer{
		Config: config.DefaultConfig(),
		State:  NewState(),
		AK:     authentik.NewClient("http://localhost", "test-token", httpClient),
	}

	flows, err := inst.ensureProviderFlows(context.Background())
	if err != nil {
		t.Fatalf("ensureProviderFlows: %v", err)
	}
	if flows.Authorization.PK != "auth-flow-pk" {
		t.Fatalf("authorization flow pk = %q", flows.Authorization.PK)
	}
	if flows.Invalidation.PK != "invalidation-flow-pk" {
		t.Fatalf("invalidation flow pk = %q", flows.Invalidation.PK)
	}
	if strings.Join(created, ",") != "authorization,invalidation" {
		t.Fatalf("created flows = %v", created)
	}
}

func readRequestBody(t *testing.T, req *http.Request) string {
	t.Helper()
	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("read request body: %v", err)
	}
	return string(body)
}
