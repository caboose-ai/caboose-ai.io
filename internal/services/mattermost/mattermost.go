package mattermost

import (
	"context"
	"fmt"
	"strings"

	"github.com/caboose-ai/caboose-ai.io/internal/docker"
	"github.com/caboose-ai/caboose-ai.io/internal/services"
	"github.com/caboose-ai/caboose-ai.io/internal/services/authentik"
)

const configPath = "/opt/mattermost/config/config.json"

type Configurator struct {
	AK           *authentik.Client
	Docker       *docker.ExecClient
	Container    string
	AuthentikURL string
}

func New(ak *authentik.Client, d *docker.ExecClient, container, authentikURL string) *Configurator {
	return &Configurator{
		AK:           ak,
		Docker:       d,
		Container:    container,
		AuthentikURL: authentikURL,
	}
}

func (c *Configurator) Name() string { return "Mattermost OIDC" }
func (c *Configurator) Slug() string { return "mattermost" }

func (c *Configurator) CheckConfigured(ctx context.Context) (bool, error) {
	if !c.containerExists(ctx) {
		return false, nil
	}
	out, err := c.Docker.Exec(ctx, c.Container, "jq", "-r", ".GitLabSettings.Id", configPath)
	if err != nil {
		return false, nil
	}
	return strings.TrimSpace(string(out)) != "" && strings.TrimSpace(string(out)) != "null", nil
}

func (c *Configurator) Configure(ctx context.Context, opts services.ConfigureOpts) (*services.ConfigureResult, error) {
	if !c.containerExists(ctx) {
		return &services.ConfigureResult{Status: services.StatusSkipped, Message: "Mattermost container not found"}, nil
	}

	provider, err := c.AK.GetProvider(ctx, "mattermost")
	if err != nil {
		return nil, fmt.Errorf("fetching Mattermost provider from Authentik: %w", err)
	}

	if opts.DryRun {
		return &services.ConfigureResult{Status: services.StatusDryRun, Message: "would patch Mattermost config.json"}, nil
	}

	if !opts.Force {
		out, err := c.Docker.Exec(ctx, c.Container, "jq", "-r", ".GitLabSettings.Id", configPath)
		if err == nil && strings.TrimSpace(string(out)) == provider.ClientID {
			return &services.ConfigureResult{Status: services.StatusAlreadyConfigured, Message: "Mattermost OIDC already configured"}, nil
		}
	}

	jqFilter := fmt.Sprintf(
		`.GitLabSettings.Enable = true | .GitLabSettings.Id = "%s" | .GitLabSettings.Secret = "%s" | .GitLabSettings.AuthEndpoint = "%s/application/o/authorize/" | .GitLabSettings.TokenEndpoint = "%s/application/o/token/" | .GitLabSettings.UserAPIEndpoint = "%s/application/o/userinfo/" | .GitLabSettings.Scope = "openid email profile"`,
		provider.ClientID, provider.ClientSecret,
		c.AuthentikURL, c.AuthentikURL, c.AuthentikURL,
	)

	_, err = c.Docker.Exec(ctx, c.Container, "sh", "-c",
		fmt.Sprintf("jq '%s' %s > /tmp/mm-config.json && mv /tmp/mm-config.json %s", jqFilter, configPath, configPath),
	)
	if err != nil {
		return nil, fmt.Errorf("patching Mattermost config: %w", err)
	}

	return &services.ConfigureResult{
		Status:          services.StatusCreated,
		Message:         "Mattermost OIDC configured",
		RestartRequired: true,
		Services:        []string{"mattermost"},
	}, nil
}

func (c *Configurator) containerExists(ctx context.Context) bool {
	_, err := c.Docker.Exec(ctx, c.Container, "true")
	return err == nil
}
