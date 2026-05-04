package portainer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

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

	alreadyInit, err := c.initAdmin(ctx)
	if err != nil {
		return nil, fmt.Errorf("Portainer admin init: %w", err)
	}

	jwt, err := c.getJWT(ctx)
	if err != nil {
		if alreadyInit {
			return nil, fmt.Errorf("Portainer admin login failed — Portainer was already initialized with a different password; "+
				"update PORTAINER_ADMIN_PASSWORD in your 1Password vault to match the existing admin password, "+
				"then re-run install: %w", err)
		}
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

// initAdmin posts to Portainer's admin init endpoint. It returns (true, nil)
// when Portainer was already initialized (HTTP 409), (false, nil) on fresh
// initialization, and (false, err) on any other failure.
func (c *Configurator) initAdmin(ctx context.Context) (alreadyInit bool, err error) {
	body, _ := json.Marshal(map[string]string{
		"Username": "admin",
		"Password": c.AdminPass,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.PortainerURL+"/api/users/admin/init", bytes.NewReader(body))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return false, fmt.Errorf("initializing Portainer admin: %w", err)
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusConflict {
		return true, nil
	}
	if resp.StatusCode >= 400 {
		return false, fmt.Errorf("Portainer admin init returned HTTP %d", resp.StatusCode)
	}
	return false, nil
}

// noRedirectDo creates an HTTP client that does not follow redirects and
// executes the given request.
func (c *Configurator) noRedirectDo(req *http.Request) (*http.Response, error) {
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Timeout: 30 * time.Second,
	}
	return client.Do(req)
}

func (c *Configurator) getJWT(ctx context.Context) (string, error) {
	body, err := json.Marshal(map[string]string{
		"Username": "admin",
		"Password": c.AdminPass,
	})
	if err != nil {
		return "", fmt.Errorf("marshalling Portainer auth request: %w", err)
	}

	const maxRetries = 5
	for attempt := 0; attempt < maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.PortainerURL+"/api/auth", bytes.NewReader(body))
		if err != nil {
			return "", fmt.Errorf("creating Portainer auth request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.noRedirectDo(req)
		if err != nil {
			return "", fmt.Errorf("authenticating with Portainer: %w", err)
		}

		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return "", fmt.Errorf("reading Portainer auth response: %w", err)
		}

		// If we get a 3xx redirect, Portainer is not ready yet (likely still showing setup wizard)
		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			if attempt < maxRetries-1 {
				select {
				case <-ctx.Done():
					return "", fmt.Errorf("waiting for Portainer auth endpoint to be ready: %w", ctx.Err())
				case <-time.After(2 * time.Second):
					continue
				}
			}
			return "", fmt.Errorf("Portainer auth endpoint still returning redirects (HTTP %d) after %d attempts", resp.StatusCode, maxRetries)
		}

		// Non-200 non-3xx response is an error
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

	return "", fmt.Errorf("Portainer auth failed after %d attempts", maxRetries)
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
