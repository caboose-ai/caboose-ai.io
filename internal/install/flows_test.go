package install

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/caboose-ai/caboose-ai.io/internal/config"
	"github.com/caboose-ai/caboose-ai.io/services/authentik"
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

func TestConfigureLogoutFlowsBindsGoogleLogoutRedirect(t *testing.T) {
	var redirectBody map[string]any
	var bindingBodies []map[string]any
	flows := map[string]string{
		"default-provider-authorization-implicit-consent": "auth-flow-pk",
		"default-provider-invalidation-flow":              "provider-invalidation-flow-pk",
		"default-invalidation-flow":                       "default-invalidation-flow-pk",
	}
	httpClient := &mockHTTP{DoFunc: func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/api/v3/flows/instances/"):
			slug := req.URL.Query().Get("slug")
			pk := flows[slug]
			if pk == "" {
				return httpResponse(200, `{"results":[]}`), nil
			}
			return httpResponse(200, `{"results":[{"pk":"`+pk+`","slug":"`+slug+`","name":"`+slug+`","designation":"invalidation"}]}`), nil
		case req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/api/v3/stages/user_logout/"):
			return httpResponse(200, `{"results":[{"pk":"logout-stage-pk","name":"default-invalidation-logout"}]}`), nil
		case req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/api/v3/stages/redirect/"):
			return httpResponse(200, `{"results":[]}`), nil
		case req.Method == http.MethodPost && strings.Contains(req.URL.Path, "/api/v3/stages/redirect/"):
			if err := json.Unmarshal([]byte(readRequestBody(t, req)), &redirectBody); err != nil {
				t.Fatalf("parsing redirect body: %v", err)
			}
			return httpResponse(201, `{"pk":"redirect-stage-pk","name":"google-account-logout-redirect","mode":"static","target_static":"https://accounts.google.com/Logout"}`), nil
		case req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/api/v3/flows/bindings/"):
			return httpResponse(200, `{"results":[]}`), nil
		case req.Method == http.MethodPost && strings.Contains(req.URL.Path, "/api/v3/flows/bindings/"):
			var body map[string]any
			if err := json.Unmarshal([]byte(readRequestBody(t, req)), &body); err != nil {
				t.Fatalf("parsing binding body: %v", err)
			}
			bindingBodies = append(bindingBodies, body)
			return httpResponse(201, `{"pk":"binding-pk"}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}}
	inst := &Installer{
		Config: config.DefaultConfig(),
		State:  NewState(),
		AK:     authentik.NewClient("http://localhost", "test-token", httpClient),
	}

	if err := inst.ConfigureLogoutFlows(context.Background()); err != nil {
		t.Fatalf("ConfigureLogoutFlows: %v", err)
	}

	if redirectBody["target_static"] != googleAccountLogoutURL {
		t.Fatalf("redirect target_static = %#v, want %q", redirectBody["target_static"], googleAccountLogoutURL)
	}
	if redirectBody["mode"] != "static" {
		t.Fatalf("redirect mode = %#v, want static", redirectBody["mode"])
	}
	if len(bindingBodies) != 4 {
		t.Fatalf("created %d bindings, want 4: %#v", len(bindingBodies), bindingBodies)
	}

	want := map[string]map[string]int{
		"default-invalidation-flow-pk":  {"logout-stage-pk": 0, "redirect-stage-pk": 10},
		"provider-invalidation-flow-pk": {"logout-stage-pk": 0, "redirect-stage-pk": 10},
	}
	for _, body := range bindingBodies {
		target, _ := body["target"].(string)
		stage, _ := body["stage"].(string)
		order, _ := body["order"].(float64)
		stages := want[target]
		if stages == nil {
			t.Fatalf("unexpected binding target %q in body %#v", target, body)
		}
		wantOrder, ok := stages[stage]
		if !ok {
			t.Fatalf("unexpected binding stage %q for target %q", stage, target)
		}
		if int(order) != wantOrder {
			t.Fatalf("binding order for %s/%s = %v, want %d", target, stage, order, wantOrder)
		}
		delete(stages, stage)
	}
	for target, stages := range want {
		if len(stages) != 0 {
			t.Fatalf("missing bindings for %s: %#v", target, stages)
		}
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
