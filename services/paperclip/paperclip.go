package paperclip

import (
	"context"
	"fmt"

	"github.com/caboose-ai/caboose-ai.io/internal/service"
	"github.com/caboose-ai/caboose-ai.io/services/authentik"
)

type Configurator struct {
	AK           *authentik.Client
	ExternalHost string
}

func New(ak *authentik.Client, externalHost ...string) *Configurator {
	cfg := &Configurator{AK: ak}
	if len(externalHost) > 0 {
		cfg.ExternalHost = externalHost[0]
	}
	return cfg
}

func (c *Configurator) Name() string { return "Paperclip Proxy" }
func (c *Configurator) Slug() string { return "paperclip" }

func (c *Configurator) CheckConfigured(ctx context.Context) (bool, error) {
	if c.AK == nil {
		return false, fmt.Errorf("Authentik client is required")
	}
	provider, err := c.AK.GetProxyProvider(ctx, "paperclip-proxy")
	if err != nil {
		return false, fmt.Errorf("fetching Paperclip proxy provider from Authentik: %w", err)
	}
	if provider == nil {
		return false, nil
	}
	if !c.providerMatches(provider) {
		return false, nil
	}
	if c.ExternalHost != "" && provider.ExternalHost != c.ExternalHost {
		return false, nil
	}

	app, err := c.AK.GetApplication(ctx, "paperclip-proxy")
	if err != nil {
		return false, fmt.Errorf("fetching Paperclip application from Authentik: %w", err)
	}
	if app == nil || app.PolicyEngineMode != "all" {
		return false, nil
	}

	outpost, err := c.AK.GetEmbeddedOutpost(ctx)
	if err != nil {
		return false, fmt.Errorf("fetching Authentik embedded outpost: %w", err)
	}
	for _, pk := range outpost.Providers {
		if pk == provider.PK {
			return true, nil
		}
	}
	return false, nil
}

func (c *Configurator) Configure(ctx context.Context, opts service.ConfigureOpts) (*service.ConfigureResult, error) {
	if opts.DryRun {
		return &service.ConfigureResult{Status: service.StatusDryRun, Message: "would verify and repair Paperclip Authentik proxy provider"}, nil
	}
	if c.AK == nil {
		return nil, fmt.Errorf("Authentik client is required")
	}
	provider, err := c.AK.GetProxyProvider(ctx, "paperclip-proxy")
	if err != nil {
		return nil, fmt.Errorf("fetching Paperclip proxy provider from Authentik: %w", err)
	}
	if provider == nil {
		return &service.ConfigureResult{Status: service.StatusSkipped, Message: "Paperclip proxy provider is created during outpost provisioning"}, nil
	}

	updated := false
	if !c.providerMatches(provider) {
		params := authentik.UpdateProxyProviderParams{Mode: "forward_single"}
		if c.ExternalHost != "" {
			params.ExternalHost = c.ExternalHost
		}
		if err := c.AK.UpdateProxyProvider(ctx, provider.PK, params); err != nil {
			return nil, fmt.Errorf("repairing Paperclip proxy provider: %w", err)
		}
		updated = true
	}

	app, err := c.AK.GetApplication(ctx, "paperclip-proxy")
	if err != nil {
		return nil, fmt.Errorf("fetching Paperclip application from Authentik: %w", err)
	}
	if app == nil {
		if _, err := c.AK.CreateApplication(ctx, "paperclip-proxy", "paperclip-proxy", provider.PK); err != nil {
			return nil, fmt.Errorf("creating Paperclip application in Authentik: %w", err)
		}
		updated = true
	} else if app.PolicyEngineMode != "all" {
		if err := c.AK.EnsureApplicationOpen(ctx, "paperclip-proxy"); err != nil {
			return nil, fmt.Errorf("opening Paperclip application policy engine mode: %w", err)
		}
		updated = true
	}

	outpost, err := c.AK.GetEmbeddedOutpost(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetching Authentik embedded outpost: %w", err)
	}
	if !containsProviderPK(outpost.Providers, provider.PK) {
		providers := append([]int{}, outpost.Providers...)
		providers = append(providers, provider.PK)
		if err := c.AK.UpdateOutpost(ctx, outpost.PK, providers, c.AK.BaseURL); err != nil {
			return nil, fmt.Errorf("binding Paperclip proxy provider to embedded outpost: %w", err)
		}
		updated = true
	}

	if updated {
		return &service.ConfigureResult{Status: service.StatusUpdated, Message: "Paperclip Authentik forward-auth proxy repaired"}, nil
	}
	return &service.ConfigureResult{Status: service.StatusAlreadyConfigured, Message: "Paperclip protected by Authentik forward-auth proxy"}, nil
}

func (c *Configurator) providerMatches(provider *authentik.ProxyProvider) bool {
	if provider.Mode != "forward_single" {
		return false
	}
	return c.ExternalHost == "" || provider.ExternalHost == c.ExternalHost
}

func containsProviderPK(providers []int, want int) bool {
	for _, pk := range providers {
		if pk == want {
			return true
		}
	}
	return false
}
