package install

import (
	"context"
	"testing"

	"github.com/caboose-ai/caboose-ai.io/internal/config"
	"github.com/caboose-ai/caboose-ai.io/internal/services/authentik"
	"github.com/caboose-ai/caboose-ai.io/internal/services/portainer"
)

func TestBuildServicesUsesLocalPortainerAPIForCompose(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Domain = "example.com"
	cfg.Orchestrator = "compose"

	inst := &Installer{
		Config: cfg,
		State:  NewState(),
		Secrets: &mockSecretStore{data: map[string]string{
			"GITEA_ADMIN_PASSWORD":     "gitea-pass",
			"PORTAINER_ADMIN_PASSWORD": "portainer-pass",
		}},
		AK: authentik.NewClient("https://auth.example.com", "token", nil),
	}

	if err := inst.BuildServices(context.Background()); err != nil {
		t.Fatalf("BuildServices: %v", err)
	}

	for _, svc := range inst.Services {
		if p, ok := svc.(*portainer.Configurator); ok {
			if p.PortainerURL != "http://127.0.0.1:9000" {
				t.Fatalf("PortainerURL = %q, want local API URL", p.PortainerURL)
			}
			if p.RedirectURI != "https://docker.example.com/" {
				t.Fatalf("RedirectURI = %q, want public URL", p.RedirectURI)
			}
			return
		}
	}
	t.Fatal("Portainer configurator not found")
}
