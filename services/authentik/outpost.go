package authentik

import (
	"context"
	"encoding/json"
	"fmt"
)

type ProxyProvider struct {
	PK           int    `json:"pk"`
	Name         string `json:"name"`
	Mode         string `json:"mode"`
	ExternalHost string `json:"external_host"`
	InternalHost string `json:"internal_host"`
}

type proxyProviderList struct {
	Results []ProxyProvider `json:"results"`
}

type CreateProxyProviderParams struct {
	Name              string `json:"name"`
	AuthorizationFlow string `json:"authorization_flow"`
	InvalidationFlow  string `json:"invalidation_flow"`
	Mode              string `json:"mode"`
	ExternalHost      string `json:"external_host"`
	InternalHost      string `json:"internal_host,omitempty"`
}

type UpdateProxyProviderParams struct {
	Mode         string `json:"mode,omitempty"`
	ExternalHost string `json:"external_host,omitempty"`
	InternalHost string `json:"internal_host,omitempty"`
}

func (c *Client) GetProxyProvider(ctx context.Context, name string) (*ProxyProvider, error) {
	data, err := c.Get(ctx, "/api/v3/providers/proxy/?search="+name)
	if err != nil {
		return nil, err
	}
	var list proxyProviderList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("parsing proxy provider response: %w", err)
	}
	for _, p := range list.Results {
		if p.Name == name {
			return &p, nil
		}
	}
	return nil, nil
}

func (c *Client) CreateProxyProvider(ctx context.Context, params CreateProxyProviderParams) (*ProxyProvider, error) {
	data, err := c.Post(ctx, "/api/v3/providers/proxy/", params)
	if err != nil {
		return nil, fmt.Errorf("creating proxy provider %q: %w", params.Name, err)
	}
	var p ProxyProvider
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parsing created proxy provider: %w", err)
	}
	return &p, nil
}

func (c *Client) UpdateProxyProvider(ctx context.Context, pk int, params UpdateProxyProviderParams) error {
	_, err := c.Patch(ctx, fmt.Sprintf("/api/v3/providers/proxy/%d/", pk), params)
	return err
}

type Outpost struct {
	PK        string `json:"pk"`
	Name      string `json:"name"`
	Providers []int  `json:"providers"`
}

type outpostList struct {
	Results []Outpost `json:"results"`
}

func (c *Client) GetEmbeddedOutpost(ctx context.Context) (*Outpost, error) {
	data, err := c.Get(ctx, "/api/v3/outposts/instances/?managed=goauthentik.io/outposts/embedded")
	if err != nil {
		return nil, err
	}
	var list outpostList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("parsing outpost response: %w", err)
	}
	if len(list.Results) == 0 {
		return nil, fmt.Errorf("embedded outpost not found")
	}
	return &list.Results[0], nil
}

func (c *Client) UpdateOutpost(ctx context.Context, outpostPK string, providerPKs []int, authentikHost string) error {
	payload := map[string]any{
		"providers": providerPKs,
		"config": map[string]any{
			"authentik_host": authentikHost,
		},
	}
	_, err := c.Patch(ctx, fmt.Sprintf("/api/v3/outposts/instances/%s/", outpostPK), payload)
	return err
}
