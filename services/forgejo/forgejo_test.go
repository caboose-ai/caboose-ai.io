package forgejo

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/caboose-ai/caboose-ai.io/internal/docker"
	"github.com/caboose-ai/caboose-ai.io/internal/runner"
	"github.com/caboose-ai/caboose-ai.io/internal/service"
	"github.com/caboose-ai/caboose-ai.io/services/authentik"
)

type mockHTTP struct {
	DoFunc func(req *http.Request) (*http.Response, error)
}

func (m *mockHTTP) Do(req *http.Request) (*http.Response, error) {
	return m.DoFunc(req)
}

func response(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}
}

func TestConfigureRepairsExternalLoginMapping(t *testing.T) {
	r := runner.NewMockRunner()
	r.On("docker exec -u git forgejo gitea admin auth list", []byte("1 OAuth2 Authentik\n"), nil)
	r.On("docker exec -u git forgejo gitea admin auth update-oauth", []byte{}, nil)
	r.OnFunc("docker exec forgejo sqlite3 /data/gitea/gitea.db", func(args []string) ([]byte, error) {
		sql := args[len(args)-1]
		for _, want := range []string{
			"external_login_user",
			"authentik-user-uid",
			"openidConnect",
			"auth-admin",
			"cxm6467@gmail.com",
		} {
			if !strings.Contains(sql, want) {
				t.Fatalf("external login SQL missing %q: %s", want, sql)
			}
		}
		return []byte{}, nil
	})

	ak := authentik.NewClient("https://auth.example.com", "token", &mockHTTP{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			switch {
			case strings.Contains(req.URL.Path, "/api/v3/providers/oauth2/"):
				return response(200, `{"results":[{"pk":1,"name":"forgejo","client_id":"client","client_secret":"secret"}]}`), nil
			case strings.Contains(req.URL.Path, "/api/v3/core/users/"):
				return response(200, `{"results":[{"pk":7,"uid":"authentik-user-uid","username":"auth-admin","email":"cxm6467@gmail.com","is_active":true}]}`), nil
			default:
				t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
				return response(404, "not found"), nil
			}
		},
	})

	c := New(ak, docker.NewExecClient(r), nil, "forgejo", "auth-admin", "https://auth.example.com")
	if _, err := c.Configure(context.Background(), service.ConfigureOpts{}); err != nil {
		t.Fatalf("Configure: %v", err)
	}
}
