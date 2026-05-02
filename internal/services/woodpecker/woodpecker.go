package woodpecker

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/caboose-ai/caboose-ai.io/internal/docker"
	"github.com/caboose-ai/caboose-ai.io/internal/secrets"
	"github.com/caboose-ai/caboose-ai.io/internal/services"
)

const appName = "Woodpecker CI"

type Configurator struct {
	Docker        *docker.ExecClient
	Secrets       secrets.SecretStore
	Container     string
	AdminUsername string
	AdminPass     string
	RedirectURI   string
}

func New(d *docker.ExecClient, s secrets.SecretStore, container, adminUser, adminPass, redirectURI string) *Configurator {
	return &Configurator{
		Docker:        d,
		Secrets:       s,
		Container:     container,
		AdminUsername:  adminUser,
		AdminPass:     adminPass,
		RedirectURI:   redirectURI,
	}
}

func (c *Configurator) Name() string { return "Woodpecker CI OAuth" }
func (c *Configurator) Slug() string { return "woodpecker" }

func (c *Configurator) CheckConfigured(ctx context.Context) (bool, error) {
	secret, err := c.Secrets.Get(ctx, "WOODPECKER_GITEA_SECRET")
	if err != nil {
		return false, err
	}
	return secret != "", nil
}

func (c *Configurator) Configure(ctx context.Context, opts services.ConfigureOpts) (*services.ConfigureResult, error) {
	forgejoURL, err := c.forgejoInternalURL(ctx)
	if err != nil {
		return nil, err
	}

	appID, err := c.findOAuthApp(ctx, forgejoURL)
	if err != nil {
		return nil, err
	}

	if opts.DryRun {
		action := "create"
		if appID != "" {
			action = "update"
		}
		return &services.ConfigureResult{Status: services.StatusDryRun, Message: fmt.Sprintf("would %s Woodpecker OAuth app", action)}, nil
	}

	if appID != "" {
		if opts.Force {
			if err := c.deleteOAuthApp(ctx, forgejoURL, appID); err != nil {
				return nil, err
			}
		} else {
			existingSecret, _ := c.Secrets.Get(ctx, "WOODPECKER_GITEA_SECRET")
			if existingSecret != "" {
				return &services.ConfigureResult{Status: services.StatusAlreadyConfigured, Message: "Woodpecker OAuth already configured"}, nil
			}
			return nil, fmt.Errorf("OAuth app exists but WOODPECKER_GITEA_SECRET not found; re-run with --force to recreate")
		}
	}

	clientID, clientSecret, err := c.createOAuthApp(ctx, forgejoURL)
	if err != nil {
		return nil, err
	}

	if err := c.Secrets.Put(ctx, "WOODPECKER_GITEA_CLIENT", clientID); err != nil {
		return nil, err
	}
	if err := c.Secrets.Put(ctx, "WOODPECKER_GITEA_SECRET", clientSecret); err != nil {
		return nil, err
	}

	return &services.ConfigureResult{
		Status:          services.StatusCreated,
		Message:         "Woodpecker OAuth configured",
		RestartRequired: true,
		Services:        []string{"woodpecker-server"},
	}, nil
}

func (c *Configurator) forgejoInternalURL(ctx context.Context) (string, error) {
	out, err := c.Docker.Exec(ctx, c.Container, "sh", "-c",
		"ip -4 addr show eth0 | grep -oP 'inet \\K[0-9.]+'")
	if err != nil {
		return "", fmt.Errorf("getting Forgejo container IP: %w", err)
	}
	ip := strings.TrimSpace(string(out))
	return fmt.Sprintf("http://%s:3000", ip), nil
}

func (c *Configurator) findOAuthApp(ctx context.Context, forgejoURL string) (string, error) {
	out, err := c.forgejoAPI(ctx, "GET", forgejoURL+"/api/v1/user/applications/oauth2", nil)
	if err != nil {
		return "", err
	}

	var apps []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(out, &apps); err != nil {
		return "", err
	}
	for _, app := range apps {
		if app.Name == appName {
			return fmt.Sprintf("%d", app.ID), nil
		}
	}
	return "", nil
}

func (c *Configurator) deleteOAuthApp(ctx context.Context, forgejoURL, appID string) error {
	_, err := c.forgejoAPI(ctx, "DELETE", forgejoURL+"/api/v1/user/applications/oauth2/"+appID, nil)
	return err
}

func (c *Configurator) createOAuthApp(ctx context.Context, forgejoURL string) (clientID, clientSecret string, err error) {
	payload := map[string]any{
		"name":          appName,
		"redirect_uris": []string{c.RedirectURI},
	}
	body, _ := json.Marshal(payload)
	out, err := c.forgejoAPI(ctx, "POST", forgejoURL+"/api/v1/user/applications/oauth2", body)
	if err != nil {
		return "", "", fmt.Errorf("creating OAuth app: %w", err)
	}

	var result struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return "", "", fmt.Errorf("parsing create response: %w", err)
	}
	if result.ClientID == "" || result.ClientSecret == "" {
		return "", "", fmt.Errorf("empty client_id or client_secret in response")
	}
	return result.ClientID, result.ClientSecret, nil
}

func (c *Configurator) forgejoAPI(ctx context.Context, method, url string, body []byte) ([]byte, error) {
	args := []string{"-sf", "-X", method, "-u", c.AdminUsername + ":" + c.AdminPass}
	if body != nil {
		args = append(args, "-H", "Content-Type: application/json", "-d", string(body))
	}
	args = append(args, url)

	return c.Docker.Exec(ctx, c.Container, append([]string{"curl"}, args...)...)
}
