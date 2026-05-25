package tui

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/caboose-ai/caboose-ai.io/internal/config"
	"github.com/caboose-ai/caboose-ai.io/internal/install"
	"github.com/caboose-ai/caboose-ai.io/internal/secrets"
	"github.com/caboose-ai/caboose-ai.io/internal/tui/views"
)

func TestRestartCompleteShowsSummaryWithoutBootstrappingPaperclip(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Domain = "example.test"
	inst := install.New(cfg, nil, nil, nil)
	inst.State.Domain = cfg.Domain

	model := NewApp(inst)
	updated, cmd := model.Update(restartCompleteMsg{})
	if cmd != nil {
		t.Fatal("restartCompleteMsg returned a command; want immediate summary without Paperclip bootstrap")
	}

	app, ok := updated.(AppModel)
	if !ok {
		t.Fatalf("updated model = %T, want AppModel", updated)
	}
	if app.currentProgress != "" {
		t.Fatalf("currentProgress = %q, want empty", app.currentProgress)
	}
	if _, ok := app.activeView.(views.SummaryModel); !ok {
		t.Fatalf("activeView = %T, want views.SummaryModel", app.activeView)
	}
}

func TestRunInitAndRenameConfiguresLogoutFlows(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Domain = "example.test"
	httpClient := &tuiInstallHTTP{}
	inst := install.New(cfg, &tuiSecretStore{values: map[string]string{
		"AUTHENTIK_BOOTSTRAP_TOKEN": "test-token",
	}}, nil, httpClient)

	msg := NewApp(inst).runInitAndRename()()
	if _, ok := msg.(adminInitDoneMsg); !ok {
		t.Fatalf("runInitAndRename returned %T (%#v), want adminInitDoneMsg", msg, msg)
	}
	if !httpClient.createdGoogleLogoutRedirect {
		t.Fatal("runInitAndRename did not configure the Google logout redirect stage")
	}
}

type tuiSecretStore struct {
	values map[string]string
}

func (s *tuiSecretStore) Get(_ context.Context, key string) (string, error) {
	return s.values[key], nil
}

func (s *tuiSecretStore) Put(_ context.Context, key, value string) error {
	s.values[key] = value
	return nil
}

func (s *tuiSecretStore) Generate(_ context.Context, key string, _ int, _ secrets.GenerateOpts) (string, error) {
	value := "generated-" + key
	s.values[key] = value
	return value, nil
}

func (s *tuiSecretStore) Delete(_ context.Context, key string) error {
	delete(s.values, key)
	return nil
}

func (s *tuiSecretStore) EnsureVault(context.Context) error {
	return nil
}

type tuiInstallHTTP struct {
	createdGoogleLogoutRedirect bool
}

func (h *tuiInstallHTTP) Do(req *http.Request) (*http.Response, error) {
	switch {
	case req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/api/v3/core/users/"):
		return tuiHTTPResponse(200, `{"results":[{"pk":1,"username":"auth-admin"}]}`), nil
	case req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/api/v3/core/brands/"):
		return tuiHTTPResponse(200, `{"results":[{"brand_uuid":"brand-uuid","default":true,"branding_custom_css":""}]}`), nil
	case req.Method == http.MethodPatch && strings.Contains(req.URL.Path, "/api/v3/core/brands/"):
		return tuiHTTPResponse(200, `{}`), nil
	case req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/api/v3/flows/instances/"):
		return tuiFlowResponse(req.URL.Query().Get("slug")), nil
	case req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/api/v3/flows/bindings/"):
		return tuiHTTPResponse(200, `{"results":[]}`), nil
	case req.Method == http.MethodPost && strings.Contains(req.URL.Path, "/api/v3/flows/bindings/"):
		return tuiHTTPResponse(201, `{"pk":"binding-pk"}`), nil
	case req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/api/v3/stages/user_logout/"):
		return tuiHTTPResponse(200, `{"results":[{"pk":"logout-stage-pk","name":"default-invalidation-logout"}]}`), nil
	case req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/api/v3/stages/redirect/"):
		return tuiHTTPResponse(200, `{"results":[]}`), nil
	case req.Method == http.MethodPost && strings.Contains(req.URL.Path, "/api/v3/stages/redirect/"):
		h.createdGoogleLogoutRedirect = true
		return tuiHTTPResponse(201, `{"pk":"redirect-stage-pk","name":"google-account-logout-redirect","mode":"static","target_static":"https://accounts.google.com/Logout"}`), nil
	default:
		return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
	}
}

func tuiFlowResponse(slug string) *http.Response {
	pks := map[string]string{
		"default-recovery-flow":                           "recovery-flow-pk",
		"default-password-change":                         "password-change-flow-pk",
		"default-provider-authorization-implicit-consent": "provider-authorization-flow-pk",
		"default-provider-invalidation-flow":              "provider-invalidation-flow-pk",
		"default-invalidation-flow":                       "default-invalidation-flow-pk",
	}
	pk := pks[slug]
	if pk == "" {
		return tuiHTTPResponse(200, `{"results":[]}`)
	}
	return tuiHTTPResponse(200, fmt.Sprintf(`{"results":[{"pk":%q,"slug":%q,"name":%q,"designation":"invalidation"}]}`, pk, slug, slug))
}

func tuiHTTPResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
