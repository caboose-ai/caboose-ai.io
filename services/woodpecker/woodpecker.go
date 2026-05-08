package woodpecker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/caboose-ai/caboose-ai.io/internal/docker"
	"github.com/caboose-ai/caboose-ai.io/internal/runner"
	"github.com/caboose-ai/caboose-ai.io/internal/secrets"
	"github.com/caboose-ai/caboose-ai.io/internal/service"
)

const appName = "Woodpecker CI"

type Configurator struct {
	Docker        *docker.ExecClient
	HTTP          runner.HTTPClient
	Secrets       secrets.SecretStore
	Container     string
	AdminUsername string
	AdminPass     string
	RedirectURI   string
}

func New(d *docker.ExecClient, httpClient runner.HTTPClient, s secrets.SecretStore, container, adminUser, adminPass, redirectURI string) *Configurator {
	return &Configurator{
		Docker:        d,
		HTTP:          httpClient,
		Secrets:       s,
		Container:     container,
		AdminUsername: adminUser,
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

func (c *Configurator) Configure(ctx context.Context, opts service.ConfigureOpts) (*service.ConfigureResult, error) {
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
		return &service.ConfigureResult{Status: service.StatusDryRun, Message: fmt.Sprintf("would %s Woodpecker OAuth app", action)}, nil
	}

	if appID != "" {
		if opts.Force {
			if err := c.deleteOAuthApp(ctx, forgejoURL, appID); err != nil {
				return nil, err
			}
		} else {
			existingSecret, _ := c.Secrets.Get(ctx, "WOODPECKER_GITEA_SECRET")
			if existingSecret != "" {
				return &service.ConfigureResult{Status: service.StatusAlreadyConfigured, Message: "Woodpecker OAuth already configured"}, nil
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

	return &service.ConfigureResult{
		Status:          service.StatusCreated,
		Message:         "Woodpecker OAuth configured",
		RestartRequired: true,
		Services:        []string{"woodpecker-server"},
	}, nil
}

func (c *Configurator) forgejoInternalURL(_ context.Context) (string, error) {
	// Forgejo maps its port to 127.0.0.1:3000 on the host, so the installer
	// (which runs on the host) can always reach it there without needing to
	// inspect container networks.
	return "http://127.0.0.1:3000", nil
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
		"name":                appName,
		"redirect_uris":       []string{c.RedirectURI},
		"confidential_client": true,
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
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("building Forgejo %s request: %w", method, err)
	}
	req.SetBasicAuth(c.AdminUsername, c.AdminPass)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Forgejo API %s %s: %w", method, url, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading Forgejo response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("Forgejo API %s returned HTTP %d: %s", method, resp.StatusCode, string(data))
	}
	return data, nil
}
