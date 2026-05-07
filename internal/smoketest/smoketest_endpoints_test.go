//go:build integration

package smoketest

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/caboose-ai/caboose-ai.io/internal/install"
)

func TestSSO_Endpoints(t *testing.T) {
	s := NewSuite(t)

	t.Run("AuthentikHealth", func(t *testing.T) {
		resp, err := s.getWithAuth(s.URLs.Authentik + "/api/v3/admin/version/")
		if err != nil {
			t.Fatalf("GET version: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		var result map[string]any
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		if _, ok := result["version_current"]; !ok {
			t.Error("response missing version_current field")
		}
		t.Logf("Authentik version: %v", result["version_current"])
	})

	t.Run("OIDCDiscovery", func(t *testing.T) {
		for _, spec := range install.DefaultProviderSpecs(s.URLs) {
			t.Run(spec.Slug, func(t *testing.T) {
				url := fmt.Sprintf("%s/application/o/%s/.well-known/openid-configuration",
					s.URLs.Authentik, spec.Slug)

				resp, err := s.get(url)
				if err != nil {
					t.Fatalf("GET OIDC discovery: %v", err)
				}
				defer resp.Body.Close()

				if resp.StatusCode != http.StatusOK {
					t.Fatalf("expected 200 for %s, got %d", spec.Slug, resp.StatusCode)
				}

				body, _ := io.ReadAll(resp.Body)
				var result map[string]any
				if err := json.Unmarshal(body, &result); err != nil {
					t.Fatalf("invalid JSON for %s: %v", spec.Slug, err)
				}
				if _, ok := result["authorization_endpoint"]; !ok {
					t.Errorf("OIDC config for %s missing authorization_endpoint", spec.Slug)
				}
			})
		}
	})

	t.Run("ServiceHealth", func(t *testing.T) {
		healthEndpoints := map[string]string{
			"forgejo":    s.URLs.Forgejo + "/api/v1/settings/api",
			"grafana":    s.URLs.Grafana + "/api/health",
			"mattermost": s.URLs.Mattermost + "/api/v4/system/ping",
		}

		for name, url := range healthEndpoints {
			t.Run(name, func(t *testing.T) {
				resp, err := s.get(url)
				if err != nil {
					t.Fatalf("GET %s health: %v", name, err)
				}
				defer resp.Body.Close()

				if resp.StatusCode != http.StatusOK {
					t.Errorf("expected 200 for %s, got %d", name, resp.StatusCode)
				}
			})
		}
	})

	t.Run("ProxyRedirects", func(t *testing.T) {
		for _, flow := range ProxyFlows(s.URLs) {
			t.Run(flow.Name, func(t *testing.T) {
				resp, err := s.get(flow.URL + "/")
				if err != nil {
					t.Fatalf("GET %s: %v", flow.Name, err)
				}
				defer resp.Body.Close()

				switch resp.StatusCode {
				case http.StatusFound, http.StatusMovedPermanently, http.StatusTemporaryRedirect:
					loc := resp.Header.Get("Location")
					if !strings.Contains(loc, "auth."+s.Domain) {
						t.Errorf("redirect for %s goes to %s, expected auth.%s", flow.Name, loc, s.Domain)
					}
				case http.StatusOK:
					body, _ := io.ReadAll(resp.Body)
					if !strings.Contains(string(body), "authentik") &&
						!strings.Contains(string(body), "Authentik") &&
						!strings.Contains(string(body), "auth."+s.Domain) {
						t.Errorf("200 response for %s doesn't appear to be Authentik login page", flow.Name)
					}
				default:
					t.Errorf("expected redirect or Authentik login for %s, got %d", flow.Name, resp.StatusCode)
				}
			})
		}
	})
}
