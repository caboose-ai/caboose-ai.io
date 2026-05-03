package portainer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/caboose-ai/caboose-ai.io/internal/runner"
	"github.com/caboose-ai/caboose-ai.io/internal/services"
	"github.com/caboose-ai/caboose-ai.io/internal/services/authentik"
)

type Configurator struct {
	AK           *authentik.Client
	HTTP         runner.HTTPClient
	PortainerURL string
	AdminPass    string
	AuthentikURL string
	RedirectURI  string
}

func New(ak *authentik.Client, httpClient runner.HTTPClient, portainerURL, adminPass, authentikURL, redirectURI string) *Configurator {
	return &Configurator{
		AK:           ak,
		HTTP:         httpClient,
		PortainerURL: portainerURL,
		AdminPass:    adminPass,
		AuthentikURL: authentikURL,
		RedirectURI:  redirectURI,
	}
}

func (c *Configurator) Name() string { return "Portainer OAuth" }
func (c *Configurator) Slug() string { return "portainer" }

func (c *Configurator) CheckConfigured(ctx context.Context) (bool, error) {
	jwt, err := c.getJWT(ctx)
	if err != nil {
		return false, err
	}
	clientID, err := c.currentOAuthClient(ctx, jwt)
	if err != nil {
		return false, err
	}
	return clientID != "", nil
}

func (c *Configurator) Configure(ctx context.Context, opts services.ConfigureOpts) (*services.ConfigureResult, error) {
	provider, err := c.AK.GetProvider(ctx, "portainer")
	if err != nil {
		return nil, fmt.Errorf("fetching Portainer provider from Authentik: %w", err)
	}

	if opts.DryRun {
		return &services.ConfigureResult{Status: services.StatusDryRun, Message: "would configure Portainer OAuth"}, nil
	}

	if err := c.initAdmin(ctx); err != nil {
		return nil, fmt.Errorf("Portainer admin init: %w", err)
	}

	jwt, err := c.getJWT(ctx)
	if err != nil {
		return nil, err
	}

	currentClient, _ := c.currentOAuthClient(ctx, jwt)
	if currentClient == provider.ClientID && !opts.Force {
		return &services.ConfigureResult{Status: services.StatusAlreadyConfigured, Message: "Portainer OAuth already configured"}, nil
	}

	if err := c.applyOAuth(ctx, jwt, provider.ClientID, provider.ClientSecret); err != nil {
		return nil, err
	}

	return &services.ConfigureResult{Status: services.StatusCreated, Message: "Portainer OAuth configured"}, nil
}

func (c *Configurator) initAdmin(ctx context.Context) error {
	body, _ := json.Marshal(map[string]string{
		"Username": "admin",
		"Password": c.AdminPass,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.PortainerURL+"/api/users/admin/init", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("initializing Portainer admin: %w", err)
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusConflict {
		return nil
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("Portainer admin init returned HTTP %d", resp.StatusCode)
	}
	return nil
}

func (c *Configurator) getJWT(ctx context.Context) (string, error) {
	body, err := json.Marshal(map[string]string{
		"Username": "admin",
		"Password": c.AdminPass,
	})
	if err != nil {
		return "", fmt.Errorf("marshalling Portainer auth request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.PortainerURL+"/api/auth", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("creating Portainer auth request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("authenticating with Portainer: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading Portainer auth response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Portainer auth returned HTTP %d", resp.StatusCode)
	}

	var result struct {
		JWT string `json:"jwt"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("parsing Portainer auth response: %w", err)
	}
	if result.JWT == "" {
		return "", fmt.Errorf("Portainer auth returned empty JWT")
	}
	return result.JWT, nil
}

func (c *Configurator) currentOAuthClient(ctx context.Context, jwt string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.PortainerURL+"/api/settings", nil)
	if err != nil {
		return "", fmt.Errorf("creating Portainer settings request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+jwt)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading Portainer settings response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Portainer settings GET returned HTTP %d", resp.StatusCode)
	}

	var settings struct {
		OAuthSettings struct {
			ClientID string `json:"ClientID"`
		} `json:"OAuthSettings"`
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		return "", fmt.Errorf("parsing Portainer settings response: %w", err)
	}
	return settings.OAuthSettings.ClientID, nil
}

func (c *Configurator) applyOAuth(ctx context.Context, jwt, clientID, clientSecret string) error {
	payload := map[string]any{
		"AuthenticationMethod": 3,
		"OAuthSettings": map[string]any{
			"ClientID":             clientID,
			"ClientSecret":         clientSecret,
			"AuthorizationURI":     c.AuthentikURL + "/application/o/authorize/",
			"AccessTokenURI":       c.AuthentikURL + "/application/o/token/",
			"ResourceURI":          c.AuthentikURL + "/application/o/userinfo/",
			"RedirectURI":          c.RedirectURI,
			"UserIdentifier":       "preferred_username",
			"Scopes":               "openid email profile",
			"OAuthAutoCreateUsers": true,
		},
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPut, c.PortainerURL+"/api/settings", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("applying Portainer OAuth: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Portainer settings PUT returned HTTP %d: %s", resp.StatusCode, string(data))
	}
	return nil
}
