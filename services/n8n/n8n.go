package n8n

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/caboose-ai/caboose-ai.io/internal/runner"
	"github.com/caboose-ai/caboose-ai.io/internal/service"
)

type Configurator struct {
	URL       string
	HTTP      runner.HTTPClient
	Email     string
	FirstName string
	LastName  string
	Password  string
}

func New(url string, httpClient runner.HTTPClient, email, password string) *Configurator {
	return &Configurator{
		URL:       url,
		HTTP:      httpClient,
		Email:     email,
		FirstName: "Caboose",
		LastName:  "Admin",
		Password:  password,
	}
}

func (c *Configurator) Name() string { return "n8n owner account" }
func (c *Configurator) Slug() string { return "n8n" }

func (c *Configurator) CheckConfigured(ctx context.Context) (bool, error) {
	setup, err := c.showSetupOnFirstLoad(ctx)
	if err != nil {
		return false, err
	}
	return !setup, nil
}

func (c *Configurator) Configure(ctx context.Context, opts service.ConfigureOpts) (*service.ConfigureResult, error) {
	setup, err := c.showSetupOnFirstLoad(ctx)
	if err != nil {
		return nil, err
	}
	if !setup {
		return &service.ConfigureResult{Status: service.StatusAlreadyConfigured, Message: "n8n owner account already configured"}, nil
	}
	if opts.DryRun {
		return &service.ConfigureResult{Status: service.StatusDryRun, Message: "would create n8n owner account"}, nil
	}

	payload := map[string]string{
		"email":     c.Email,
		"firstName": c.FirstName,
		"lastName":  c.LastName,
		"password":  c.Password,
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL+"/rest/owner/setup", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("building n8n owner setup request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("n8n owner setup request: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("n8n owner setup returned HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	return &service.ConfigureResult{Status: service.StatusCreated, Message: "n8n owner account created"}, nil
}

func (c *Configurator) showSetupOnFirstLoad(ctx context.Context) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.URL+"/rest/settings", nil)
	if err != nil {
		return false, fmt.Errorf("building n8n settings request: %w", err)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return false, fmt.Errorf("n8n settings request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("n8n settings returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	var parsed struct {
		Data struct {
			UserManagement struct {
				ShowSetupOnFirstLoad bool `json:"showSetupOnFirstLoad"`
			} `json:"userManagement"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return false, fmt.Errorf("parsing n8n settings: %w", err)
	}
	return parsed.Data.UserManagement.ShowSetupOnFirstLoad, nil
}
