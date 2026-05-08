package paperclip

import (
	"context"
	"fmt"

	"github.com/caboose-ai/caboose-ai.io/internal/services"
	"github.com/caboose-ai/caboose-ai.io/internal/services/authentik"
)

type Configurator struct {
	AK *authentik.Client
}

func New(ak *authentik.Client) *Configurator {
	return &Configurator{AK: ak}
}

func (c *Configurator) Name() string { return "Paperclip Proxy" }
func (c *Configurator) Slug() string { return "paperclip" }

func (c *Configurator) CheckConfigured(ctx context.Context) (bool, error) {
	provider, err := c.AK.GetProxyProvider(ctx, "paperclip-proxy")
	if err != nil {
		return false, fmt.Errorf("fetching Paperclip proxy provider from Authentik: %w", err)
	}
	return provider != nil, nil
}

func (c *Configurator) Configure(ctx context.Context, opts services.ConfigureOpts) (*services.ConfigureResult, error) {
	if opts.DryRun {
		return &services.ConfigureResult{Status: services.StatusDryRun, Message: "would verify Paperclip Authentik proxy provider"}, nil
	}
	configured, err := c.CheckConfigured(ctx)
	if err != nil {
		return nil, err
	}
	if !configured {
		return &services.ConfigureResult{Status: services.StatusSkipped, Message: "Paperclip proxy provider is created during outpost provisioning"}, nil
	}
	return &services.ConfigureResult{Status: services.StatusAlreadyConfigured, Message: "Paperclip protected by Authentik forward-auth proxy"}, nil
}
