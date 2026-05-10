package servicebuilder

import (
	"context"
	"os"
	"testing"

	"github.com/caboose-ai/caboose-ai.io/internal/config"
	"github.com/caboose-ai/caboose-ai.io/internal/secrets"
	"github.com/caboose-ai/caboose-ai.io/internal/service"
	"github.com/caboose-ai/caboose-ai.io/services/authentik"
)

func TestServiceManifestsOwnDashboardVisibility(t *testing.T) {
	manifests := loadTestManifests(t)

	visible := []string{"authentik", "forgejo", "woodpecker", "portainer", "grafana", "open-webui", "openclaw", "homarr", "paperclip"}
	for _, slug := range visible {
		t.Run(slug, func(t *testing.T) {
			manifest := manifests[slug]
			if manifest.Dashboard.Show == nil || !*manifest.Dashboard.Show {
				t.Fatalf("%s dashboard.show = %v, want true", slug, manifest.Dashboard.Show)
			}
		})
	}

	hidden := []string{"ghost", "mattermost", "sonarqube"}
	for _, slug := range hidden {
		t.Run(slug, func(t *testing.T) {
			manifest := manifests[slug]
			if manifest.Dashboard.Show == nil || *manifest.Dashboard.Show {
				t.Fatalf("%s dashboard.show = %v, want false", slug, manifest.Dashboard.Show)
			}
		})
	}
}

func TestServiceManifestsOwnSSOMetadata(t *testing.T) {
	manifests := loadTestManifests(t)

	tests := map[string]string{
		"authentik":  "identity_provider",
		"forgejo":    "oidc",
		"woodpecker": "proxy",
		"openclaw":   "proxy",
		"ghost":      "proxy",
		"paperclip":  "proxy",
		"social":     "sources",
		"sonarqube":  "local",
	}

	for slug, want := range tests {
		t.Run(slug, func(t *testing.T) {
			if got := manifests[slug].SSO.Mode; got != want {
				t.Fatalf("%s sso.mode = %q, want %q", slug, got, want)
			}
		})
	}
}

func TestHomarrAppsFollowManifestDashboardVisibility(t *testing.T) {
	apps := homarrApps(config.DeriveURLs("example.com"), loadTestManifests(t))
	present := map[string]bool{}
	for _, app := range apps {
		present[app.Name] = true
	}

	for _, name := range []string{"Ghost", "Homarr Alias", "Mattermost", "SonarQube"} {
		if present[name] {
			t.Fatalf("%s should not be shown in the SSO dashboard", name)
		}
	}
	for _, name := range []string{"Forgejo", "Portainer", "Grafana", "Paperclip"} {
		if !present[name] {
			t.Fatalf("%s should be shown in the SSO dashboard", name)
		}
	}
}

func TestHomarrAppsUseOpenWebUIOIDCLogin(t *testing.T) {
	apps := homarrApps(config.DeriveURLs("example.com"), loadTestManifests(t))
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

func TestBuildFallsBackToCompiledDashboardMetadataOutsideRepo(t *testing.T) {
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	})
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	configurators, err := Build(context.Background(), Dependencies{
		Config:        config.DefaultConfig(),
		Secrets:       emptySecretStore{},
		Authentik:     authentik.NewClient("https://auth.example.com", "token", nil),
		AllowDefaults: true,
	})
	if err != nil {
		t.Fatalf("Build outside repo: %v", err)
	}
	if len(configurators) == 0 {
		t.Fatal("Build returned no configurators")
	}
}

func loadTestManifests(t *testing.T) map[string]service.Manifest {
	t.Helper()
	manifests, err := service.LoadManifests("../../services")
	if err != nil {
		t.Fatalf("LoadManifests: %v", err)
	}
	return manifests
}

type emptySecretStore struct{}

func (emptySecretStore) Get(context.Context, string) (string, error) { return "", nil }
func (emptySecretStore) Put(context.Context, string, string) error   { return nil }
func (emptySecretStore) Generate(context.Context, string, int, secrets.GenerateOpts) (string, error) {
	return "", nil
}
func (emptySecretStore) Delete(context.Context, string) error { return nil }
func (emptySecretStore) EnsureVault(context.Context) error    { return nil }
