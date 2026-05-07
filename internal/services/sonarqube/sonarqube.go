package sonarqube

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/caboose-ai/caboose-ai.io/internal/runner"
	"github.com/caboose-ai/caboose-ai.io/internal/services"
)

const defaultPassword = "admin"

type Configurator struct {
	URL           string
	HTTP          runner.HTTPClient
	AdminPassword string
}

func New(url string, httpClient runner.HTTPClient, adminPassword string) *Configurator {
	return &Configurator{URL: url, HTTP: httpClient, AdminPassword: adminPassword}
}

func (c *Configurator) Name() string { return "SonarQube admin account" }
func (c *Configurator) Slug() string { return "sonarqube" }

func (c *Configurator) CheckConfigured(ctx context.Context) (bool, error) {
	ok, err := c.validate(ctx, c.AdminPassword)
	if err == nil && ok {
		return true, nil
	}
	defaultOK, defaultErr := c.validate(ctx, defaultPassword)
	if defaultErr != nil {
		return false, defaultErr
	}
	return !defaultOK, err
}

func (c *Configurator) Configure(ctx context.Context, opts services.ConfigureOpts) (*services.ConfigureResult, error) {
	if opts.DryRun {
		return &services.ConfigureResult{Status: services.StatusDryRun, Message: "would rotate SonarQube default admin password"}, nil
	}
	if ok, _ := c.validate(ctx, c.AdminPassword); ok {
		return &services.ConfigureResult{Status: services.StatusAlreadyConfigured, Message: "SonarQube admin password already configured"}, nil
	}
	if ok, err := c.validate(ctx, defaultPassword); err != nil {
		return nil, err
	} else if !ok {
		return nil, fmt.Errorf("SonarQube admin is not using the generated password and default admin/admin is no longer valid")
	}

	form := url.Values{}
	form.Set("login", "admin")
	form.Set("previousPassword", defaultPassword)
	form.Set("password", c.AdminPassword)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL+"/api/users/change_password", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("building SonarQube password change request: %w", err)
	}
	req.SetBasicAuth("admin", defaultPassword)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("SonarQube password change request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("SonarQube password change returned HTTP %d: %s", resp.StatusCode, string(body))
	}
	return &services.ConfigureResult{Status: services.StatusUpdated, Message: "SonarQube default admin password rotated"}, nil
}

func (c *Configurator) validate(ctx context.Context, password string) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.URL+"/api/authentication/validate", nil)
	if err != nil {
		return false, fmt.Errorf("building SonarQube auth validation request: %w", err)
	}
	req.SetBasicAuth("admin", password)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return false, fmt.Errorf("SonarQube auth validation request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("SonarQube auth validation returned HTTP %d: %s", resp.StatusCode, string(body))
	}
	return strings.Contains(string(body), `"valid":true`), nil
}
