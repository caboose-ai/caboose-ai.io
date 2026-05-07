package mattermost

import (
	"context"
	"fmt"
	"strings"

	"github.com/caboose-ai/caboose-ai.io/internal/docker"
	"github.com/caboose-ai/caboose-ai.io/internal/secrets"
	"github.com/caboose-ai/caboose-ai.io/internal/services"
)

type Configurator struct {
	Docker        *docker.ExecClient
	Secrets       secrets.SecretStore
	Container     string
	AdminUsername string
	AdminEmail    string
	TeamName      string
	TeamDisplay   string
}

func New(d *docker.ExecClient, s secrets.SecretStore, container, adminEmail string) *Configurator {
	return &Configurator{
		Docker:        d,
		Secrets:       s,
		Container:     container,
		AdminUsername: "auth-admin",
		AdminEmail:    adminEmail,
		TeamName:      "caboose",
		TeamDisplay:   "Caboose",
	}
}

func (c *Configurator) Name() string { return "Mattermost local admin" }
func (c *Configurator) Slug() string { return "mattermost" }

func (c *Configurator) CheckConfigured(ctx context.Context) (bool, error) {
	if !c.containerExists(ctx) {
		return false, nil
	}
	out, err := c.Docker.Exec(ctx, c.Container, "/mattermost/bin/mmctl", "--local", "user", "search", c.AdminUsername, "--json")
	if err != nil {
		return false, nil
	}
	return strings.Contains(string(out), `"username": "`+c.AdminUsername+`"`), nil
}

func (c *Configurator) Configure(ctx context.Context, opts services.ConfigureOpts) (*services.ConfigureResult, error) {
	if !c.containerExists(ctx) {
		return &services.ConfigureResult{Status: services.StatusSkipped, Message: "Mattermost container not found"}, nil
	}

	if opts.DryRun {
		return &services.ConfigureResult{Status: services.StatusDryRun, Message: "would create Mattermost local admin and default team"}, nil
	}

	adminPass, err := c.Secrets.Get(ctx, "MATTERMOST_ADMIN_PASSWORD")
	if err != nil {
		return nil, fmt.Errorf("retrieving MATTERMOST_ADMIN_PASSWORD: %w", err)
	}

	created := false
	if ok, _ := c.CheckConfigured(ctx); !ok || opts.Force {
		_, err := c.Docker.Exec(ctx, c.Container,
			"/mattermost/bin/mmctl", "--local", "user", "create",
			"--email", c.AdminEmail,
			"--username", c.AdminUsername,
			"--password", adminPass,
			"--system-admin",
			"--email-verified",
			"--disable-welcome-email",
		)
		if err != nil && !strings.Contains(err.Error(), "already exists") {
			return nil, fmt.Errorf("creating Mattermost admin user: %w", err)
		}
		created = true
	}

	if _, err := c.Docker.Exec(ctx, c.Container,
		"/mattermost/bin/mmctl", "--local", "team", "create",
		"--name", c.TeamName,
		"--display-name", c.TeamDisplay,
	); err != nil && !strings.Contains(err.Error(), "already exists") {
		return nil, fmt.Errorf("creating Mattermost team: %w", err)
	}

	if _, err := c.Docker.Exec(ctx, c.Container,
		"/mattermost/bin/mmctl", "--local", "team", "users", "add", c.TeamName, c.AdminUsername,
	); err != nil && !strings.Contains(err.Error(), "already") {
		return nil, fmt.Errorf("adding Mattermost admin to team: %w", err)
	}

	if !created && !opts.Force {
		return &services.ConfigureResult{Status: services.StatusAlreadyConfigured, Message: "Mattermost local admin already configured"}, nil
	}
	return &services.ConfigureResult{Status: services.StatusCreated, Message: "Mattermost local admin and team configured"}, nil
}

func (c *Configurator) containerExists(ctx context.Context) bool {
	_, err := c.Docker.Exec(ctx, c.Container, "true")
	return err == nil
}
