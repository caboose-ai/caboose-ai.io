package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

type testConfigurator struct {
	slug string
	name string
}

func (c testConfigurator) Name() string { return c.name }
func (c testConfigurator) Slug() string { return c.slug }
func (c testConfigurator) CheckConfigured(context.Context) (bool, error) {
	return true, nil
}
func (c testConfigurator) Configure(context.Context, ConfigureOpts) (*ConfigureResult, error) {
	return &ConfigureResult{Status: StatusAlreadyConfigured}, nil
}

func TestLoadManifestsIncludesConfiguredAndOperationalServices(t *testing.T) {
	manifests, err := LoadManifests("../../services")
	if err != nil {
		t.Fatalf("LoadManifests: %v", err)
	}

	for _, slug := range []string{"forgejo", "openclaw", "paperclip"} {
		if _, ok := manifests[slug]; !ok {
			t.Fatalf("manifest %q missing from loaded registry", slug)
		}
	}
	if manifests["forgejo"].Configurator != "forgejo" {
		t.Fatalf("forgejo configurator = %q, want forgejo", manifests["forgejo"].Configurator)
	}
	if manifests["forgejo"].Runtime != RuntimeCompose {
		t.Fatalf("forgejo runtime = %q, want %q", manifests["forgejo"].Runtime, RuntimeCompose)
	}
	if manifests["openclaw"].Runtime != RuntimeExternal {
		t.Fatalf("openclaw runtime = %q, want %q", manifests["openclaw"].Runtime, RuntimeExternal)
	}
	if len(manifests["openclaw"].ComposeServices) != 0 {
		t.Fatalf("openclaw compose services = %v, want none", manifests["openclaw"].ComposeServices)
	}
}

func TestPaperclipManifestDeclaresProxySSOAndDashboardVisibility(t *testing.T) {
	manifests, err := LoadManifests("../../services")
	if err != nil {
		t.Fatalf("LoadManifests: %v", err)
	}

	paperclip := manifests["paperclip"]
	if paperclip.SSO.Mode != "proxy" {
		t.Fatalf("paperclip sso.mode = %q, want proxy", paperclip.SSO.Mode)
	}
	if paperclip.Dashboard.Show == nil || !*paperclip.Dashboard.Show {
		t.Fatalf("paperclip dashboard.show = %v, want true", paperclip.Dashboard.Show)
	}
	if paperclip.Health.Path != "/api/health" {
		t.Fatalf("paperclip health.path = %q, want /api/health", paperclip.Health.Path)
	}
}

func TestRegistryMatchesManifestsToConfigurators(t *testing.T) {
	registry := NewRegistry(map[string]Manifest{
		"forgejo": {
			Slug:            "forgejo",
			DisplayName:     "Forgejo",
			Runtime:         RuntimeCompose,
			ComposeServices: []string{"forgejo"},
			Configurator:    "forgejo",
		},
		"openclaw": {
			Slug:        "openclaw",
			DisplayName: "OpenClaw",
			Runtime:     RuntimeExternal,
		},
	}, []ServiceConfigurator{
		testConfigurator{slug: "forgejo", name: "Forgejo"},
	})

	if _, ok := registry.Configurator("forgejo"); !ok {
		t.Fatal("forgejo configurator missing")
	}
	if _, ok := registry.Configurator("openclaw"); ok {
		t.Fatal("openclaw should not have a configurator")
	}
	if got := registry.ConfigurableSlugs(); len(got) != 1 || got[0] != "forgejo" {
		t.Fatalf("ConfigurableSlugs = %v, want [forgejo]", got)
	}
}

func TestLoadManifestsRejectsUnknownRuntime(t *testing.T) {
	root := t.TempDir()
	serviceDir := filepath.Join(root, "bad-service")
	if err := os.Mkdir(serviceDir, 0o755); err != nil {
		t.Fatalf("mkdir service dir: %v", err)
	}
	manifest := []byte("slug: bad-service\ndisplay_name: Bad Service\nruntime: externl\n")
	if err := os.WriteFile(filepath.Join(serviceDir, "service.yaml"), manifest, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	_, err := LoadManifests(root)
	if err == nil {
		t.Fatal("LoadManifests succeeded, want unknown runtime error")
	}
}
