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

	// Use positional parameters ($1/$2/$3) so values are never embedded in the shell
	// command string, then pass them to jq via --arg to prevent injection.
	// configPath is a package-level constant, so it is always safe to embed directly.
	const jqScript = `jq --arg clientID "$1" --arg clientSecret "$2" --arg authentikURL "$3" \
'.GitLabSettings.Enable = true
| .GitLabSettings.Id = $clientID
| .GitLabSettings.Secret = $clientSecret
| .GitLabSettings.AuthEndpoint = ($authentikURL + "/application/o/authorize/")
| .GitLabSettings.TokenEndpoint = ($authentikURL + "/application/o/token/")
| .GitLabSettings.UserAPIEndpoint = ($authentikURL + "/application/o/userinfo/")
| .GitLabSettings.Scope = "openid email profile"' ` + configPath + ` > ` + configPath + `.tmp && mv ` + configPath + `.tmp ` + configPath

	_, err = c.Docker.Exec(ctx, c.Container,
		"sh", "-c", jqScript,
		"--",                 // positional $0
		provider.ClientID,    // positional $1
		provider.ClientSecret, // positional $2
		c.AuthentikURL,       // positional $3
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
