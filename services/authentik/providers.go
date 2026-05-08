package authentik

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

type Provider struct {
	PK           int           `json:"pk"`
	Name         string        `json:"name"`
	ClientID     string        `json:"client_id"`
	ClientSecret string        `json:"client_secret"`
	RedirectURIs []RedirectURI `json:"redirect_uris"`
}

type providerList struct {
	Results []Provider `json:"results"`
}

func (c *Client) GetProvider(ctx context.Context, name string) (*Provider, error) {
	slug := strings.ToLower(strings.ReplaceAll(name, " ", "-"))
	data, err := c.Get(ctx, fmt.Sprintf("/api/v3/providers/oauth2/?search=%s", url.QueryEscape(slug)))
	if err != nil {
		return nil, err
	}

	var list providerList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("parsing provider response: %w", err)
	}

	for _, p := range list.Results {
		if strings.EqualFold(strings.ReplaceAll(p.Name, " ", "-"), slug) {
			return &p, nil
		}
	}
	return nil, nil
}

func (c *Client) ListProviders(ctx context.Context) ([]Provider, error) {
	data, err := c.Get(ctx, "/api/v3/providers/oauth2/?page_size=50")
	if err != nil {
		return nil, err
	}

	var list providerList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("parsing providers: %w", err)
	}
	return list.Results, nil
}

type RedirectURI struct {
	MatchingMode string `json:"matching_mode"`
	URL          string `json:"url"`
}

type CreateProviderParams struct {
	Name              string        `json:"name"`
	AuthorizationFlow string        `json:"authorization_flow"`
	InvalidationFlow  string        `json:"invalidation_flow"`
	ClientType        string        `json:"client_type"`
	RedirectURIs      []RedirectURI `json:"redirect_uris"`
	SigningKey        string        `json:"signing_key,omitempty"`
	PropertyMappings  []string      `json:"property_mappings,omitempty"`
}

func (c *Client) CreateProvider(ctx context.Context, params CreateProviderParams) (*Provider, error) {
	data, err := c.Post(ctx, "/api/v3/providers/oauth2/", params)
	if err != nil {
		return nil, fmt.Errorf("creating provider %q: %w", params.Name, err)
	}

	var p Provider
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parsing created provider: %w", err)
	}
	return &p, nil
}

func (c *Client) UpdateProviderOIDCSettings(ctx context.Context, pk int, redirectURIs []RedirectURI, signingKey string) error {
	body := map[string]any{"redirect_uris": redirectURIs}
	if signingKey != "" {
		body["signing_key"] = signingKey
	}
	_, err := c.Patch(ctx, fmt.Sprintf("/api/v3/providers/oauth2/%d/", pk), body)
	if err != nil {
		return fmt.Errorf("updating provider %d OIDC settings: %w", pk, err)
	}
	return nil
}

type Flow struct {
	PK          string `json:"pk"`
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Designation string `json:"designation"`
}

type flowList struct {
	Results []Flow `json:"results"`
}

func (c *Client) GetFlow(ctx context.Context, slug string) (*Flow, error) {
	data, err := c.Get(ctx, fmt.Sprintf("/api/v3/flows/instances/?slug=%s", url.QueryEscape(slug)))
	if err != nil {
		return nil, err
	}

	var list flowList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("parsing flow response: %w", err)
	}
	if len(list.Results) == 0 {
		return nil, fmt.Errorf("flow %q not found", slug)
	}
	return &list.Results[0], nil
}

func (c *Client) GetFlowByDesignation(ctx context.Context, designation string) (*Flow, error) {
	data, err := c.Get(ctx, fmt.Sprintf("/api/v3/flows/instances/?designation=%s", url.QueryEscape(designation)))
	if err != nil {
		return nil, err
	}

	var list flowList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("parsing flow response: %w", err)
	}
	if len(list.Results) == 0 {
		return nil, fmt.Errorf("no flow with designation %q found", designation)
	}
	return &list.Results[0], nil
}

type CreateFlowParams struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	Designation string `json:"designation"`
}

func (c *Client) CreateFlow(ctx context.Context, params CreateFlowParams) (*Flow, error) {
	data, err := c.Post(ctx, "/api/v3/flows/instances/", params)
	if err != nil {
		return nil, fmt.Errorf("creating flow %q: %w", params.Slug, err)
	}

	var flow Flow
	if err := json.Unmarshal(data, &flow); err != nil {
		return nil, fmt.Errorf("parsing created flow: %w", err)
	}
	return &flow, nil
}

type PropertyMapping struct {
	PK      string `json:"pk"`
	Name    string `json:"name"`
	Managed string `json:"managed"`
}

type propertyMappingList struct {
	Results []PropertyMapping `json:"results"`
}

func (c *Client) ListOAuthScopeMappings(ctx context.Context) ([]PropertyMapping, error) {
	data, err := c.Get(ctx, "/api/v3/propertymappings/provider/scope/?page_size=100")
	if err != nil {
		return nil, err
	}
	var list propertyMappingList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("parsing scope mappings: %w", err)
	}
	return list.Results, nil
}

type Application struct {
	PK               string `json:"pk"`
	Slug             string `json:"slug"`
	Name             string `json:"name"`
	PolicyEngineMode string `json:"policy_engine_mode"`
}

type applicationList struct {
	Results []Application `json:"results"`
}

func (c *Client) GetApplication(ctx context.Context, slug string) (*Application, error) {
	data, err := c.Get(ctx, fmt.Sprintf("/api/v3/core/applications/?search=%s&page_size=100", url.QueryEscape(slug)))
	if err != nil {
		return nil, err
	}
	var list applicationList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("parsing application response: %w", err)
	}
	for _, app := range list.Results {
		if app.Slug == slug {
			return &app, nil
		}
	}
	return nil, nil
}

func (c *Client) EnsureApplicationOpen(ctx context.Context, slug string) error {
	app, err := c.GetApplication(ctx, slug)
	if err != nil {
		return err
	}
	if app == nil {
		return fmt.Errorf("application %q not found", slug)
	}
	if app.PolicyEngineMode == "all" {
		return nil
	}
	payload := map[string]any{"policy_engine_mode": "all"}
	_, err = c.Patch(ctx, fmt.Sprintf("/api/v3/core/applications/%s/", url.PathEscape(slug)), payload)
	return err
}

func (c *Client) CreateApplication(ctx context.Context, name, slug string, providerPK int) (*Application, error) {
	payload := map[string]any{
		"name":               name,
		"slug":               slug,
		"provider":           providerPK,
		"policy_engine_mode": "all",
	}
	data, err := c.Post(ctx, "/api/v3/core/applications/", payload)
	if err != nil {
		return nil, fmt.Errorf("creating application %q: %w", slug, err)
	}
	var app Application
	if err := json.Unmarshal(data, &app); err != nil {
		return nil, fmt.Errorf("parsing created application: %w", err)
	}
	return &app, nil
}
