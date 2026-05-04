package mattermost

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/caboose-ai/caboose-ai.io/internal/docker"
	"github.com/caboose-ai/caboose-ai.io/internal/services"
	"github.com/caboose-ai/caboose-ai.io/internal/services/authentik"
)

const configPath = "/mattermost/config/config.json"

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
	cfg, err := c.readConfig(ctx)
	if err != nil {
		return false, nil
	}
	gl := cfg.gitLabSettings()
	id, _ := gl["Id"].(string)
	return id != "", nil
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

	cfg, err := c.readConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("reading Mattermost config: %w", err)
	}

	if !opts.Force {
		gl := cfg.gitLabSettings()
		if id, _ := gl["Id"].(string); id == provider.ClientID {
			return &services.ConfigureResult{Status: services.StatusAlreadyConfigured, Message: "Mattermost OIDC already configured"}, nil
		}
	}

	gl := cfg.gitLabSettings()
	gl["Enable"] = true
	gl["Id"] = provider.ClientID
	gl["Secret"] = provider.ClientSecret
	gl["AuthEndpoint"] = c.AuthentikURL + "/application/o/authorize/"
	gl["TokenEndpoint"] = c.AuthentikURL + "/application/o/token/"
	gl["UserAPIEndpoint"] = c.AuthentikURL + "/application/o/userinfo/"
	gl["Scope"] = "openid email profile"
	cfg["GitLabSettings"] = gl

	if err := c.writeConfig(ctx, cfg); err != nil {
		return nil, fmt.Errorf("patching Mattermost config: %w", err)
	}

	return &services.ConfigureResult{
		Status:          services.StatusCreated,
		Message:         "Mattermost OIDC configured",
		RestartRequired: true,
		Services:        []string{"mattermost"},
	}, nil
}

type mmConfig map[string]any

func (m mmConfig) gitLabSettings() map[string]any {
	gl, ok := m["GitLabSettings"].(map[string]any)
	if !ok {
		gl = make(map[string]any)
	}
	return gl
}

func (c *Configurator) readConfig(ctx context.Context) (mmConfig, error) {
	out, err := c.Docker.Exec(ctx, c.Container, "cat", configPath)
	if err != nil {
		return nil, err
	}
	var cfg mmConfig
	if err := json.Unmarshal(out, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config.json: %w", err)
	}
	return cfg, nil
}

func (c *Configurator) writeConfig(ctx context.Context, cfg mmConfig) error {
	data, err := json.MarshalIndent(cfg, "", "    ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	_, err = c.Docker.ExecWithStdin(ctx, c.Container, bytes.NewReader(data),
		"sh", "-c", "cat > "+configPath,
	)
	return err
}

func (c *Configurator) containerExists(ctx context.Context) bool {
	_, err := c.Docker.Exec(ctx, c.Container, "true")
	return err == nil
}
