package forgejo

import (
	"context"
	"fmt"
	"strings"

	"github.com/caboose-ai/caboose-ai.io/internal/docker"
	"github.com/caboose-ai/caboose-ai.io/internal/secrets"
	"github.com/caboose-ai/caboose-ai.io/internal/services"
	"github.com/caboose-ai/caboose-ai.io/internal/services/authentik"
)

type Configurator struct {
	AK            *authentik.Client
	Docker        *docker.ExecClient
	Secrets       secrets.SecretStore
	Container     string
	AdminUsername string
	AuthentikURL  string
	SourceName    string
}

func New(ak *authentik.Client, d *docker.ExecClient, s secrets.SecretStore, container, adminUser, authentikURL string) *Configurator {
	return &Configurator{
		AK:            ak,
		Docker:        d,
		Secrets:       s,
		Container:     container,
		AdminUsername:  adminUser,
		AuthentikURL:  authentikURL,
		SourceName:    "Authentik",
	}
}

func (c *Configurator) Name() string { return "Forgejo OIDC" }
func (c *Configurator) Slug() string { return "forgejo" }

func (c *Configurator) CheckConfigured(ctx context.Context) (bool, error) {
	id, err := c.authSourceID(ctx)
	if err != nil {
		return false, err
	}
	return id != "", nil
}

func (c *Configurator) Configure(ctx context.Context, opts services.ConfigureOpts) (*services.ConfigureResult, error) {
	provider, err := c.AK.GetProvider(ctx, "forgejo")
	if err != nil {
		return nil, fmt.Errorf("fetching Forgejo provider from Authentik: %w", err)
	}

	discoveryURL := fmt.Sprintf("%s/application/o/forgejo/.well-known/openid-configuration", c.AuthentikURL)
	existingID, err := c.authSourceID(ctx)
	if err != nil {
		return nil, err
	}

	if opts.DryRun {
		action := "create"
		if existingID != "" {
			action = "update"
		}
		return &services.ConfigureResult{
			Status:  services.StatusDryRun,
			Message: fmt.Sprintf("would %s Forgejo OIDC auth source", action),
		}, nil
	}

	if existingID != "" && opts.Force {
		if _, err := c.Docker.Exec(ctx, c.Container, "gitea", "admin", "auth", "delete", "--id", existingID); err != nil {
			return nil, fmt.Errorf("deleting existing auth source: %w", err)
		}
		existingID = ""
	}

	if existingID != "" {
		err = c.updateOIDC(ctx, existingID, provider.ClientID, provider.ClientSecret, discoveryURL)
		if err != nil {
			return nil, err
		}
		return &services.ConfigureResult{Status: services.StatusUpdated, Message: "Forgejo OIDC updated"}, nil
	}

	err = c.addOIDC(ctx, provider.ClientID, provider.ClientSecret, discoveryURL)
	if err != nil {
		return nil, err
	}
	return &services.ConfigureResult{Status: services.StatusCreated, Message: "Forgejo OIDC created"}, nil
}

func (c *Configurator) authSourceID(ctx context.Context) (string, error) {
	out, err := c.Docker.Exec(ctx, c.Container, "gitea", "admin", "auth", "list")
	if err != nil {
		return "", fmt.Errorf("listing auth sources: %w", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, c.SourceName) {
			fields := strings.Fields(line)
			if len(fields) > 0 {
				return fields[0], nil
			}
		}
	}
	return "", nil
}

func (c *Configurator) addOIDC(ctx context.Context, clientID, clientSecret, discoveryURL string) error {
	_, err := c.Docker.Exec(ctx, c.Container,
		"gitea", "admin", "auth", "add-oauth",
		"--name", c.SourceName,
		"--provider", "openidConnect",
		"--key", clientID,
		"--secret", clientSecret,
		"--auto-discover-url", discoveryURL,
	)
	return err
}

func (c *Configurator) updateOIDC(ctx context.Context, sourceID, clientID, clientSecret, discoveryURL string) error {
	_, err := c.Docker.Exec(ctx, c.Container,
		"gitea", "admin", "auth", "update-oauth",
		"--id", sourceID,
		"--key", clientID,
		"--secret", clientSecret,
		"--auto-discover-url", discoveryURL,
	)
	return err
}
