package servicebuilder

import (
	"testing"

	"github.com/caboose-ai/caboose-ai.io/internal/config"
)

func TestHomarrAppsExcludeNonSSOApps(t *testing.T) {
	apps := homarrApps(config.DeriveURLs("example.com"))
	present := map[string]bool{}
	for _, app := range apps {
		present[app.Name] = true
	}

	for _, name := range []string{"Ghost", "Homarr Alias", "Mattermost", "n8n", "Paperclip", "SonarQube"} {
		if present[name] {
			t.Fatalf("%s should not be shown in the SSO dashboard", name)
		}
	}
	for _, name := range []string{"Forgejo", "Portainer", "Grafana"} {
		if !present[name] {
			t.Fatalf("%s should be shown in the SSO dashboard", name)
		}
	}
}

func TestHomarrAppsUseOpenWebUIOIDCLogin(t *testing.T) {
	apps := homarrApps(config.DeriveURLs("example.com"))
	for _, app := range apps {
		if app.Name == "Open WebUI" {
			if app.URL != "https://ai.example.com/oauth/oidc/login" {
				t.Fatalf("Open WebUI dashboard URL = %q, want OIDC login URL", app.URL)
			}
			return
		}
	}
	t.Fatal("Open WebUI should be shown in the SSO dashboard")
}
