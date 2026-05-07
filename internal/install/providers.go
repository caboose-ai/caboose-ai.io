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
		{Name: "mattermost", Slug: "mattermost", RedirectURIs: []string{
			urls.Mattermost + "/signup/openid/complete",
		}},
		{Name: "homarr", Slug: "homarr", RedirectURIs: []string{
			urls.Dashboard + "/api/auth/callback/oidc",
			urls.DashAlias + "/api/auth/callback/oidc",
		}},
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

	flows, err := inst.ensureProviderFlows(ctx)
	if err != nil {
		return fmt.Errorf("ensuring provider flows: %w", err)
	}

	scopeMappings, err := inst.AK.ListOAuthScopeMappings(ctx)
	if err != nil {
		return fmt.Errorf("listing scope mappings: %w", err)
	}
	signingKey, err := inst.providerSigningKey(ctx)
	if err != nil {
		return fmt.Errorf("finding provider signing key: %w", err)
	}
	var mappingPKs []string
	for _, m := range scopeMappings {
		if strings.HasPrefix(m.Managed, "goauthentik.io/providers/oauth2/scope-") {
			mappingPKs = append(mappingPKs, m.PK)
		}
	}

	for _, spec := range inst.ProviderSpecs() {
		redirectURIs := providerRedirectURIs(spec.RedirectURIs)

		existing, err := inst.AK.GetProvider(ctx, spec.Name)
		if err != nil {
			progressFn(ProviderProgress{Name: spec.Name, Action: "error", Err: err})
			return fmt.Errorf("looking up provider %q: %w", spec.Name, err)
		}
		if existing != nil {
			if err := inst.AK.UpdateProviderOIDCSettings(ctx, existing.PK, redirectURIs, signingKey); err != nil {
				progressFn(ProviderProgress{Name: spec.Name, Action: "error", Err: err})
				return fmt.Errorf("updating provider %q OIDC settings: %w", spec.Name, err)
			}
			if err := inst.ensureOpenApplication(ctx, spec.Name, spec.Slug, existing.PK); err != nil {
				progressFn(ProviderProgress{Name: spec.Name, Action: "error", Err: err})
				return err
			}
			progressFn(ProviderProgress{Name: spec.Name, Action: "exists"})
			continue
		}

		progressFn(ProviderProgress{Name: spec.Name, Action: "creating"})

		provider, err := inst.AK.CreateProvider(ctx, authentik.CreateProviderParams{
			Name:              spec.Name,
			AuthorizationFlow: flows.Authorization.PK,
			InvalidationFlow:  flows.Invalidation.PK,
			ClientType:        "confidential",
			RedirectURIs:      redirectURIs,
			SigningKey:        signingKey,
			PropertyMappings:  mappingPKs,
		})
		if err != nil {
			progressFn(ProviderProgress{Name: spec.Name, Action: "error", Err: err})
			return fmt.Errorf("creating provider %q: %w", spec.Name, err)
		}

		if err := inst.ensureOpenApplication(ctx, spec.Name, spec.Slug, provider.PK); err != nil {
			progressFn(ProviderProgress{Name: spec.Name, Action: "error", Err: err})
			return err
		}

		progressFn(ProviderProgress{Name: spec.Name, Action: "created"})
	}
	return nil
}

func providerRedirectURIs(urls []string) []authentik.RedirectURI {
	redirectURIs := make([]authentik.RedirectURI, len(urls))
	for i, u := range urls {
		redirectURIs[i] = authentik.RedirectURI{MatchingMode: "strict", URL: u}
	}
	return redirectURIs
}

func (inst *Installer) providerSigningKey(ctx context.Context) (string, error) {
	keys, err := inst.AK.ListCertificateKeyPairs(ctx)
	if err != nil {
		return "", err
	}
	if len(keys) == 0 {
		return "", nil
	}
	for _, key := range keys {
		if key.Name == "authentik Internal JWT Certificate" {
			return key.PK, nil
		}
	}
	return keys[0].PK, nil
}
