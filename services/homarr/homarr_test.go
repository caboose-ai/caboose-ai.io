package homarr

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/caboose-ai/caboose-ai.io/internal/docker"
	"github.com/caboose-ai/caboose-ai.io/internal/secrets"
	"github.com/caboose-ai/caboose-ai.io/internal/service"
	"github.com/caboose-ai/caboose-ai.io/services/authentik"
)

type memorySecrets struct {
	values map[string]string
}

func (m *memorySecrets) Get(_ context.Context, key string) (string, error) {
	return m.values[key], nil
}

func (m *memorySecrets) Put(_ context.Context, key, value string) error {
	m.values[key] = value
	return nil
}

func (m *memorySecrets) Generate(_ context.Context, key string, length int, _ secrets.GenerateOpts) (string, error) {
	m.values[key] = strings.Repeat("x", length)
	return m.values[key], nil
}

func (m *memorySecrets) Delete(_ context.Context, key string) error {
	delete(m.values, key)
	return nil
}

func (m *memorySecrets) EnsureVault(context.Context) error { return nil }

type mockHTTP struct {
	do func(req *http.Request) (*http.Response, error)
}

func (m mockHTTP) Do(req *http.Request) (*http.Response, error) {
	return m.do(req)
}

func TestConfigureWritesHomarrOIDCSecrets(t *testing.T) {
	store := &memorySecrets{values: map[string]string{}}
	ak := authentik.NewClient("http://localhost", "test-token", mockHTTP{do: func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/api/v3/providers/oauth2/") {
			return httpResponse(200, `{"results":[{"pk":1,"name":"homarr","client_id":"homarr-client","client_secret":"homarr-secret"}]}`), nil
		}
		t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		return nil, nil
	}})
	cfg := New(ak, store, "", nil, nil, "")

	result, err := cfg.Configure(context.Background(), service.ConfigureOpts{})
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if result.Status != service.StatusCreated {
		t.Fatalf("status = %s, want %s", result.Status, service.StatusCreated)
	}
	if got := store.values["HOMARR_OIDC_CLIENT_ID"]; got != "homarr-client" {
		t.Fatalf("client id = %q", got)
	}
	if got := store.values["HOMARR_OIDC_CLIENT_SECRET"]; got != "homarr-secret" {
		t.Fatalf("client secret = %q", got)
	}
	if !result.RestartRequired || len(result.Services) != 1 || result.Services[0] != "homarr" {
		t.Fatalf("restart result = %+v", result)
	}
}

func TestConfigureInitializesHomarrFirstRunState(t *testing.T) {
	store := &memorySecrets{values: map[string]string{
		"HOMARR_OIDC_CLIENT_ID": "homarr-client",
	}}
	var requests []string
	steps := []string{"start", "group", "settings", "finish"}
	ak := authentik.NewClient("http://localhost", "test-token", mockHTTP{do: func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/api/v3/providers/oauth2/") {
			return httpResponse(200, `{"results":[{"pk":1,"name":"homarr","client_id":"homarr-client","client_secret":"homarr-secret"}]}`), nil
		}
		t.Fatalf("unexpected Authentik request: %s %s", req.Method, req.URL.String())
		return nil, nil
	}})
	httpClient := mockHTTP{do: func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.Method+" "+req.URL.Path)
		if req.Method == http.MethodGet && req.URL.Path == "/api/trpc/onboard.currentStep" {
			if len(steps) == 0 {
				t.Fatal("too many currentStep requests")
			}
			step := steps[0]
			steps = steps[1:]
			return httpResponse(200, `{"result":{"data":{"json":{"current":"`+step+`"}}}}`), nil
		}
		if req.Method == http.MethodPost {
			return httpResponse(200, `{"result":{"data":{"json":null}}}`), nil
		}
		t.Fatalf("unexpected Homarr request: %s %s", req.Method, req.URL.String())
		return nil, nil
	}}
	mockRunner := &captureRunner{}
	cfg := New(ak, store, "http://homarr.local", httpClient, docker.NewExecClient(mockRunner), "homarr")

	result, err := cfg.Configure(context.Background(), service.ConfigureOpts{})
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if result.Status != service.StatusAlreadyConfigured {
		t.Fatalf("status = %s, want %s", result.Status, service.StatusAlreadyConfigured)
	}
	wantRequests := []string{
		"GET /api/trpc/onboard.currentStep",
		"POST /api/trpc/onboard.nextStep",
		"GET /api/trpc/onboard.currentStep",
		"POST /api/trpc/group.createInitialExternalGroup",
		"GET /api/trpc/onboard.currentStep",
		"POST /api/trpc/serverSettings.initSettings",
		"GET /api/trpc/onboard.currentStep",
	}
	if strings.Join(requests, "\n") != strings.Join(wantRequests, "\n") {
		t.Fatalf("requests:\n%s\nwant:\n%s", strings.Join(requests, "\n"), strings.Join(wantRequests, "\n"))
	}
	if len(mockRunner.Calls) != 1 {
		t.Fatalf("docker calls = %d, want 1", len(mockRunner.Calls))
	}
}

func TestEnsureDefaultBoardSeedsServiceApps(t *testing.T) {
	mockRunner := &captureRunner{}
	cfg := New(nil, nil, "", nil, docker.NewExecClient(mockRunner), "homarr")
	cfg.Apps = []App{
		{Name: "Forgejo", URL: "https://git.example.com"},
		{Name: "Grafana", URL: "https://grafana.example.com"},
		{Name: "Paperclip", URL: "https://paperclip.example.com"},
	}

	if err := cfg.ensureDefaultBoard(context.Background()); err != nil {
		t.Fatalf("ensureDefaultBoard: %v", err)
	}
	script := mockRunner.Stdin
	for _, want := range []string{
		`"name":"Forgejo"`,
		`"href":"https://git.example.com"`,
		`"iconUrl":"https://cdn.jsdelivr.net/gh/homarr-labs/dashboard-icons/svg/forgejo.svg"`,
		`homelab_app_forgejo`,
		`homelab_item_forgejo`,
		`"name":"Grafana"`,
		`"href":"https://grafana.example.com"`,
		`"iconUrl":"https://cdn.jsdelivr.net/gh/homarr-labs/dashboard-icons/svg/grafana.svg"`,
		`"name":"Paperclip"`,
		`"href":"https://paperclip.example.com"`,
		`"iconUrl":"https://cdn.jsdelivr.net/gh/homarr-labs/dashboard-icons/svg/paperclip.svg"`,
		`homelab_app_paperclip`,
		`homelab_item_paperclip`,
		`delete from item_layout`,
		`delete from item`,
		`delete from app`,
		`run(boardId, "Home", 0)`,
		`update board set is_public = 0 where id = ?`,
		`insert into boardGroupPermission`,
		`permission) values (?, ?, ?) on conflict`,
		`{name: "everyone", permission: "view"}`,
		`{name: "authentik Admins", permission: "full"}`,
		`insert into app`,
		`insert into item`,
		`insert into item_layout`,
		`homelab_tablet_layout`,
		`homelab_desktop_layout`,
		`column_count = excluded.column_count`,
		`breakpoint = excluded.breakpoint`,
		`on conflict(item_id, section_id, layout_id) do update`,
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("seed script missing %q:\n%s", want, script)
		}
	}
}

func httpResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       ioNopCloser{strings.NewReader(body)},
		Header:     make(http.Header),
	}
}

type ioNopCloser struct {
	*strings.Reader
}

func (ioNopCloser) Close() error { return nil }

type captureRunner struct {
	Calls []string
	Stdin string
}

func (r *captureRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return r.RunWithStdin(ctx, nil, name, args...)
}

func (r *captureRunner) RunWithStdin(_ context.Context, stdin io.Reader, name string, args ...string) ([]byte, error) {
	r.Calls = append(r.Calls, name+" "+strings.Join(args, " "))
	if stdin != nil {
		data, _ := io.ReadAll(stdin)
		r.Stdin = string(data)
	}
	return []byte("ok"), nil
}
