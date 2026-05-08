package service

import (
	"context"
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
	if len(manifests["openclaw"].ComposeServices) == 0 {
		t.Fatalf("openclaw manifest should name at least one compose service")
	}
}

func TestRegistryMatchesManifestsToConfigurators(t *testing.T) {
	registry := NewRegistry(map[string]Manifest{
		"forgejo": {
			Slug:            "forgejo",
			DisplayName:     "Forgejo",
			ComposeServices: []string{"forgejo"},
			Configurator:    "forgejo",
		},
		"openclaw": {
			Slug:            "openclaw",
			DisplayName:     "OpenClaw",
			ComposeServices: []string{"openclaw"},
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
