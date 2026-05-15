package social

import (
	"context"
	"fmt"

	"github.com/caboose-ai/caboose-ai.io/internal/config"
	"github.com/caboose-ai/caboose-ai.io/internal/service"
	"github.com/caboose-ai/caboose-ai.io/services/authentik"
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

const googleUserMatchingMode = "email_link"

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

func (c *Configurator) Configure(ctx context.Context, opts service.ConfigureOpts) (*service.ConfigureResult, error) {
	configured := 0
	googleConfigured := false

	var authFlowPK, enrollFlowPK string
	if !opts.DryRun {
		if f, err := c.AK.GetFlow(ctx, "default-source-authentication"); err == nil {
			authFlowPK = f.PK
		}
		if f, err := c.AK.GetFlow(ctx, "default-source-enrollment"); err == nil {
			enrollFlowPK = f.PK
		}
	}

	if creds := c.Social.GitHub; creds != nil && creds.ClientID != "" && creds.ClientSecret != "" {
		if opts.DryRun {
			configured++
		} else {
			err := c.AK.UpsertSource(ctx, githubSourceParams(*creds, authFlowPK, enrollFlowPK))
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
			err := c.AK.UpsertSource(ctx, googleSourceParams(*creds, authFlowPK, enrollFlowPK))
			if err != nil {
				return nil, err
			}
			googleConfigured = true
			configured++
		}
	}

	if configured == 0 {
		return &service.ConfigureResult{Status: service.StatusSkipped, Message: "No social login credentials provided"}, nil
	}

	if !opts.DryRun && googleConfigured {
		if err := c.bindGoogleSourceToLoginFlow(ctx); err != nil {
			return nil, fmt.Errorf("binding sources to login flow: %w", err)
		}
	}

	status := service.StatusCreated
	if opts.DryRun {
		status = service.StatusDryRun
	}
	return &service.ConfigureResult{
		Status:  status,
		Message: fmt.Sprintf("%d social login source(s) configured", configured),
	}, nil
}

func githubSourceParams(creds config.OAuthCredentials, authFlowPK, enrollFlowPK string) authentik.UpsertSourceParams {
	promoted := false
	return authentik.UpsertSourceParams{
		Name:               "GitHub",
		Slug:               "github",
		Enabled:            false,
		Promoted:           &promoted,
		ProviderType:       "github",
		ConsumerKey:        creds.ClientID,
		ConsumerSecret:     creds.ClientSecret,
		AuthenticationFlow: authFlowPK,
		EnrollmentFlow:     enrollFlowPK,
	}
}

func googleSourceParams(creds config.OAuthCredentials, authFlowPK, enrollFlowPK string) authentik.UpsertSourceParams {
	promoted := false
	return authentik.UpsertSourceParams{
		Name:               "Google",
		Slug:               "google",
		Enabled:            true,
		Promoted:           &promoted,
		ProviderType:       "google",
		UserMatchingMode:   googleUserMatchingMode,
		ConsumerKey:        creds.ClientID,
		ConsumerSecret:     creds.ClientSecret,
		AuthenticationFlow: authFlowPK,
		EnrollmentFlow:     enrollFlowPK,
	}
}

func (c *Configurator) bindGoogleSourceToLoginFlow(ctx context.Context) error {
	stage, err := c.AK.GetIdentificationStage(ctx, "default-authentication-flow")
	if err != nil || stage == nil {
		return err
	}

	sources, err := c.AK.ListSources(ctx)
	if err != nil {
		return err
	}

	pks := googleLoginSourcePKs(sources)
	if len(pks) == 0 {
		return fmt.Errorf("enabled google source not found")
	}

	return c.AK.SetIdentificationStageSources(ctx, stage.PK, pks)
}

func googleLoginSourcePKs(sources []authentik.OAuthSource) []string {
	pks := make([]string, 0, 1)
	for _, source := range sources {
		if source.Slug == "google" && source.Enabled && source.PK != "" {
			pks = append(pks, source.PK)
		}
	}
	return pks
}
