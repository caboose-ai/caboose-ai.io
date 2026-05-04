package install

import (
	"context"
	"fmt"
	"strings"

	"github.com/caboose-ai/caboose-ai.io/internal/config"
	"github.com/caboose-ai/caboose-ai.io/internal/services/authentik"
)


type ProviderSpec struct {
	Name         string
	Slug         string
	RedirectURIs []string
}

type ProviderProgress struct {
	Name   string
	Action string // "exists", "creating", "created", "error"
	Err    error
}

func DefaultProviderSpecs(urls config.URLs) []ProviderSpec {
	return []ProviderSpec{
		{Name: "forgejo", Slug: "forgejo", RedirectURIs: []string{urls.Forgejo + "/user/oauth2/Authentik/callback"}},
		{Name: "grafana", Slug: "grafana", RedirectURIs: []string{urls.Grafana + "/login/generic_oauth"}},
		{Name: "portainer", Slug: "portainer", RedirectURIs: []string{urls.Portainer + "/"}},
		{Name: "open-webui", Slug: "open-webui", RedirectURIs: []string{urls.OpenWebUI + "/oauth/oidc/callback"}},
		{Name: "mattermost", Slug: "mattermost", RedirectURIs: []string{urls.Mattermost + "/signup/gitlab/complete"}},
	}
}

func (inst *Installer) ProviderSpecs() []ProviderSpec {
	return DefaultProviderSpecs(inst.Config.URLs())
}

func (inst *Installer) ProvisionProviders(ctx context.Context, progressFn func(ProviderProgress)) error {
	if progressFn == nil {
		progressFn = func(ProviderProgress) {}
	}

	if inst.State.DryRun {
		for _, spec := range inst.ProviderSpecs() {
			progressFn(ProviderProgress{Name: spec.Name, Action: "created"})
		}
		return nil
	}

	authFlow, err := inst.AK.GetFlow(ctx, "default-provider-authorization-implicit-consent")
	if err != nil {
		return fmt.Errorf("looking up authorization flow: %w", err)
	}

	invalidationFlow, err := inst.AK.GetFlow(ctx, "default-provider-invalidation-flow")
	if err != nil {
		return fmt.Errorf("looking up invalidation flow: %w", err)
	}

	scopeMappings, err := inst.AK.ListOAuthScopeMappings(ctx)
	if err != nil {
		return fmt.Errorf("listing scope mappings: %w", err)
	}
	var mappingPKs []string
	for _, m := range scopeMappings {
		if strings.HasPrefix(m.Managed, "goauthentik.io/providers/oauth2/scope-") {
			mappingPKs = append(mappingPKs, m.PK)
		}
	}

	for _, spec := range inst.ProviderSpecs() {
		existing, err := inst.AK.GetProvider(ctx, spec.Name)
		if err != nil {
			progressFn(ProviderProgress{Name: spec.Name, Action: "error", Err: err})
			return fmt.Errorf("looking up provider %q: %w", spec.Name, err)
		}
		if existing != nil {
			progressFn(ProviderProgress{Name: spec.Name, Action: "exists"})
			continue
		}

		progressFn(ProviderProgress{Name: spec.Name, Action: "creating"})

		redirectURIs := make([]authentik.RedirectURI, len(spec.RedirectURIs))
		for i, u := range spec.RedirectURIs {
			redirectURIs[i] = authentik.RedirectURI{MatchingMode: "strict", URL: u}
		}

		provider, err := inst.AK.CreateProvider(ctx, authentik.CreateProviderParams{
			Name:              spec.Name,
			AuthorizationFlow: authFlow.PK,
			InvalidationFlow:  invalidationFlow.PK,
			ClientType:        "confidential",
			RedirectURIs:      redirectURIs,
			PropertyMappings:  mappingPKs,
		})
		if err != nil {
			progressFn(ProviderProgress{Name: spec.Name, Action: "error", Err: err})
			return fmt.Errorf("creating provider %q: %w", spec.Name, err)
		}

		app, err := inst.AK.GetApplication(ctx, spec.Slug)
		if err != nil {
			progressFn(ProviderProgress{Name: spec.Name, Action: "error", Err: err})
			return fmt.Errorf("looking up application %q: %w", spec.Slug, err)
		}
		if app == nil {
			if _, err := inst.AK.CreateApplication(ctx, spec.Name, spec.Slug, provider.PK); err != nil {
				progressFn(ProviderProgress{Name: spec.Name, Action: "error", Err: err})
				return fmt.Errorf("creating application %q: %w", spec.Slug, err)
			}
		}

		progressFn(ProviderProgress{Name: spec.Name, Action: "created"})
	}
	return nil
}
