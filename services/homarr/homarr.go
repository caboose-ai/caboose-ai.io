package homarr

import (
	"context"
	"fmt"

	"github.com/caboose-ai/caboose-ai.io/internal/secrets"
	"github.com/caboose-ai/caboose-ai.io/internal/service"
	"github.com/caboose-ai/caboose-ai.io/services/authentik"
)

type Configurator struct {
	AK      *authentik.Client
	Secrets secrets.SecretStore
}

func New(ak *authentik.Client, s secrets.SecretStore) *Configurator {
	return &Configurator{AK: ak, Secrets: s}
}

func (c *Configurator) Name() string { return "Homarr OIDC" }
func (c *Configurator) Slug() string { return "homarr" }

func (c *Configurator) CheckConfigured(ctx context.Context) (bool, error) {
	id, err := c.Secrets.Get(ctx, "HOMARR_OIDC_CLIENT_ID")
	if err != nil {
		return false, err
	}
	return id != "", nil
}

func (c *Configurator) Configure(ctx context.Context, opts service.ConfigureOpts) (*service.ConfigureResult, error) {
	provider, err := c.AK.GetProvider(ctx, "homarr")
	if err != nil {
		return nil, fmt.Errorf("fetching Homarr provider from Authentik: %w", err)
	}

	if opts.DryRun {
		return &service.ConfigureResult{Status: service.StatusDryRun, Message: "would write Homarr OIDC credentials to .env"}, nil
	}

	existingID, _ := c.Secrets.Get(ctx, "HOMARR_OIDC_CLIENT_ID")
	if existingID == provider.ClientID && !opts.Force {
		return &service.ConfigureResult{Status: service.StatusAlreadyConfigured, Message: "Homarr OIDC already configured"}, nil
	}

	if err := c.Secrets.Put(ctx, "HOMARR_OIDC_CLIENT_ID", provider.ClientID); err != nil {
		return nil, err
	}
	if err := c.Secrets.Put(ctx, "HOMARR_OIDC_CLIENT_SECRET", provider.ClientSecret); err != nil {
		return nil, err
	}

	return &service.ConfigureResult{
		Status:          service.StatusCreated,
		Message:         "Homarr OIDC credentials written to .env",
		RestartRequired: true,
		Services:        []string{"homarr"},
	}, nil
}
