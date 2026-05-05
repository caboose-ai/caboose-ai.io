package authentik

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

type OAuthSource struct {
	PK           string `json:"pk"`
	Name         string `json:"name"`
	Slug         string `json:"slug"`
	Enabled      bool   `json:"enabled"`
	ProviderType string `json:"provider_type"`
}

type sourceList struct {
	Results []OAuthSource `json:"results"`
}

func (c *Client) ListSources(ctx context.Context) ([]OAuthSource, error) {
	data, err := c.Get(ctx, "/api/v3/sources/oauth/?page_size=50")
	if err != nil {
		return nil, err
	}
	var list sourceList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("parsing sources: %w", err)
	}
	return list.Results, nil
}

func (c *Client) GetSourcePK(ctx context.Context, slug string) (string, error) {
	data, err := c.Get(ctx, fmt.Sprintf("/api/v3/sources/oauth/?slug=%s", url.QueryEscape(slug)))
	if err != nil {
		return "", err
	}

	var list sourceList
	if err := json.Unmarshal(data, &list); err != nil {
		return "", fmt.Errorf("parsing source response: %w", err)
	}

	if len(list.Results) > 0 {
		return list.Results[0].PK, nil
	}
	return "", nil
}

type UpsertSourceParams struct {
	Name               string `json:"name"`
	Slug               string `json:"slug"`
	Enabled            bool   `json:"enabled"`
	Promoted           bool   `json:"promoted"`
	ProviderType       string `json:"provider_type"`
	ConsumerKey        string `json:"consumer_key"`
	ConsumerSecret     string `json:"consumer_secret"`
	AuthenticationFlow string `json:"authentication_flow,omitempty"`
	EnrollmentFlow     string `json:"enrollment_flow,omitempty"`
}

func (c *Client) UpsertSource(ctx context.Context, params UpsertSourceParams) error {
	pk, err := c.GetSourcePK(ctx, params.Slug)
	if err != nil {
		return err
	}

	if pk != "" {
		_, err = c.Patch(ctx, fmt.Sprintf("/api/v3/sources/oauth/%s/", url.PathEscape(params.Slug)), params)
	} else {
		_, err = c.Post(ctx, "/api/v3/sources/oauth/", params)
	}
	return err
}
