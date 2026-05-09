package forgejo

import (
	"context"
	"fmt"
	"strings"

	"github.com/caboose-ai/caboose-ai.io/internal/docker"
	"github.com/caboose-ai/caboose-ai.io/internal/secrets"
	"github.com/caboose-ai/caboose-ai.io/internal/service"
	"github.com/caboose-ai/caboose-ai.io/services/authentik"
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
		AdminUsername: adminUser,
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

func (c *Configurator) Configure(ctx context.Context, opts service.ConfigureOpts) (*service.ConfigureResult, error) {
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
		return &service.ConfigureResult{
			Status:  service.StatusDryRun,
			Message: fmt.Sprintf("would %s Forgejo OIDC auth source", action),
		}, nil
	}

	if existingID != "" && opts.Force {
		if _, err := c.Docker.ExecAs(ctx, c.Container, "git", "gitea", "admin", "auth", "delete", "--id", existingID); err != nil {
			return nil, fmt.Errorf("deleting existing auth source: %w", err)
		}
		existingID = ""
	}

	if existingID != "" {
		err = c.updateOIDC(ctx, existingID, provider.ClientID, provider.ClientSecret, discoveryURL)
		if err != nil {
			return nil, err
		}
		if err := c.ensureExternalLoginMapping(ctx, existingID); err != nil {
			return nil, err
		}
		return &service.ConfigureResult{Status: service.StatusUpdated, Message: "Forgejo OIDC updated"}, nil
	}

	err = c.addOIDC(ctx, provider.ClientID, provider.ClientSecret, discoveryURL)
	if err != nil {
		return nil, err
	}
	existingID, err = c.authSourceID(ctx)
	if err != nil {
		return nil, err
	}
	if err := c.ensureExternalLoginMapping(ctx, existingID); err != nil {
		return nil, err
	}
	return &service.ConfigureResult{Status: service.StatusCreated, Message: "Forgejo OIDC created"}, nil
}

func (c *Configurator) authSourceID(ctx context.Context) (string, error) {
	out, err := c.Docker.ExecAs(ctx, c.Container, "git", "gitea", "admin", "auth", "list")
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
	_, err := c.Docker.ExecAs(ctx, c.Container, "git",
		"gitea", "admin", "auth", "add-oauth",
		"--name", c.SourceName,
		"--provider", "openidConnect",
		"--key", clientID,
		"--secret", clientSecret,
		"--auto-discover-url", discoveryURL,
		"--scopes", "openid",
		"--scopes", "email",
		"--scopes", "profile",
	)
	return err
}

func (c *Configurator) updateOIDC(ctx context.Context, sourceID, clientID, clientSecret, discoveryURL string) error {
	_, err := c.Docker.ExecAs(ctx, c.Container, "git",
		"gitea", "admin", "auth", "update-oauth",
		"--id", sourceID,
		"--key", clientID,
		"--secret", clientSecret,
		"--auto-discover-url", discoveryURL,
		"--scopes", "openid",
		"--scopes", "email",
		"--scopes", "profile",
	)
	return err
}

func (c *Configurator) ensureExternalLoginMapping(ctx context.Context, sourceID string) error {
	if sourceID == "" {
		return fmt.Errorf("Forgejo Authentik login source not found after OIDC configuration")
	}
	user, err := c.AK.FindUser(ctx, c.AdminUsername)
	if err != nil {
		return fmt.Errorf("fetching Authentik admin user for Forgejo external-login mapping: %w", err)
	}
	if user == nil {
		return fmt.Errorf("Authentik admin user %q not found for Forgejo external-login mapping", c.AdminUsername)
	}
	if user.UID == "" {
		return fmt.Errorf("Authentik admin user %q has empty uid for Forgejo external-login mapping", c.AdminUsername)
	}
	email := user.Email
	if email == "" {
		email = c.AdminUsername + "@local"
	}

	sql := fmt.Sprintf(`
begin;
insert or replace into external_login_user (external_id, user_id, login_source_id, provider, email, name, nick_name)
select %[1]s, u.id, %[2]s, 'openidConnect', %[3]s, %[4]s, %[4]s
from user u
where u.lower_name = lower(%[4]s);
commit;`,
		sqlQuote(user.UID),
		sqlQuote(sourceID),
		sqlQuote(email),
		sqlQuote(c.AdminUsername),
	)
	if _, err := c.Docker.Exec(ctx, c.Container, "sqlite3", "/data/gitea/gitea.db", sql); err != nil {
		return fmt.Errorf("writing Forgejo external-login mapping: %w", err)
	}
	return nil
}

func sqlQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}
