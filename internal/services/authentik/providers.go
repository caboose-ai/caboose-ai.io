package authentik

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

type Provider struct {
	PK           int    `json:"pk"`
	Name         string `json:"name"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
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
	return nil, fmt.Errorf("provider %q not found in Authentik", name)
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

type Flow struct {
	PK   string `json:"pk"`
	Slug string `json:"slug"`
	Name string `json:"name"`
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
	PK   string `json:"pk"`
	Slug string `json:"slug"`
	Name string `json:"name"`
}

type applicationList struct {
	Results []Application `json:"results"`
}

func (c *Client) GetApplication(ctx context.Context, slug string) (*Application, error) {
	data, err := c.Get(ctx, fmt.Sprintf("/api/v3/core/applications/?slug=%s", url.QueryEscape(slug)))
	if err != nil {
		return nil, err
	}
	var list applicationList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("parsing application response: %w", err)
	}
	if len(list.Results) == 0 {
		return nil, nil
	}
	return &list.Results[0], nil
}

func (c *Client) CreateApplication(ctx context.Context, name, slug string, providerPK int) (*Application, error) {
	payload := map[string]any{
		"name":     name,
		"slug":     slug,
		"provider": providerPK,
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
