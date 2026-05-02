package social

import (
	"context"
	"fmt"

	"github.com/caboose-ai/caboose-ai.io/internal/config"
	"github.com/caboose-ai/caboose-ai.io/internal/services"
	"github.com/caboose-ai/caboose-ai.io/internal/services/authentik"
)

type Configurator struct {
	AK     *authentik.Client
	Social config.SocialConfig
}

func New(ak *authentik.Client, social config.SocialConfig) *Configurator {
	return &Configurator{AK: ak, Social: social}
}

func (c *Configurator) Name() string { return "Social Login" }
func (c *Configurator) Slug() string { return "social" }

func (c *Configurator) CheckConfigured(ctx context.Context) (bool, error) {
	if c.Social.GitHub != nil && c.Social.GitHub.ClientID != "" {
		pk, _ := c.AK.GetSourcePK(ctx, "github")
		if pk == "" {
			return false, nil
		}
	}
	if c.Social.Google != nil && c.Social.Google.ClientID != "" {
		pk, _ := c.AK.GetSourcePK(ctx, "google")
		if pk == "" {
			return false, nil
		}
	}
	return true, nil
}

func (c *Configurator) Configure(ctx context.Context, opts services.ConfigureOpts) (*services.ConfigureResult, error) {
	configured := 0

	if creds := c.Social.GitHub; creds != nil && creds.ClientID != "" && creds.ClientSecret != "" {
		if opts.DryRun {
			configured++
		} else {
			err := c.AK.UpsertSource(ctx, authentik.UpsertSourceParams{
				Name:           "GitHub",
				Slug:           "github",
				Enabled:        true,
				ProviderType:   "github",
				ConsumerKey:    creds.ClientID,
				ConsumerSecret: creds.ClientSecret,
			})
			if err != nil {
				return nil, err
			}
			configured++
		}
	}

	if creds := c.Social.Google; creds != nil && creds.ClientID != "" && creds.ClientSecret != "" {
		if opts.DryRun {
			configured++
		} else {
			err := c.AK.UpsertSource(ctx, authentik.UpsertSourceParams{
				Name:           "Google",
				Slug:           "google",
				Enabled:        true,
				ProviderType:   "google",
				ConsumerKey:    creds.ClientID,
				ConsumerSecret: creds.ClientSecret,
			})
			if err != nil {
				return nil, err
			}
			configured++
		}
	}

	if configured == 0 {
		return &services.ConfigureResult{Status: services.StatusSkipped, Message: "No social login credentials provided"}, nil
	}

	status := services.StatusCreated
	if opts.DryRun {
		status = services.StatusDryRun
	}
	return &services.ConfigureResult{
		Status:  status,
		Message: fmt.Sprintf("%d social login source(s) configured", configured),
	}, nil
}
