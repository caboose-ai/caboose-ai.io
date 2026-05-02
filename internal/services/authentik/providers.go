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
