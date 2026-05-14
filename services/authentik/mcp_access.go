package authentik

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type CreateOAuthScopeMappingParams struct {
	Name        string `json:"name"`
	ScopeName   string `json:"scope_name"`
	Description string `json:"description"`
	Expression  string `json:"expression"`
}

func (c *Client) CreateOAuthScopeMapping(ctx context.Context, params CreateOAuthScopeMappingParams) (*PropertyMapping, error) {
	data, err := c.Post(ctx, "/api/v3/propertymappings/provider/scope/", params)
	if err != nil {
		return nil, fmt.Errorf("creating OAuth scope mapping %q: %w", params.ScopeName, err)
	}
	var mapping PropertyMapping
	if err := json.Unmarshal(data, &mapping); err != nil {
		return nil, fmt.Errorf("parsing OAuth scope mapping: %w", err)
	}
	return &mapping, nil
}

func (c *Client) GetOAuthScopeMappingByScopeName(ctx context.Context, scopeName string) (*PropertyMapping, error) {
	mappings, err := c.ListOAuthScopeMappings(ctx)
	if err != nil {
		return nil, err
	}
	for _, mapping := range mappings {
		if mapping.ScopeName == scopeName || strings.EqualFold(mapping.Name, scopeName) {
			return &mapping, nil
		}
	}
	return nil, nil
}

func (c *Client) UpdateProviderMCPSettings(ctx context.Context, pk int, propertyMappings []string, signingKey string) error {
	body := map[string]any{
		"property_mappings": propertyMappings,
		"redirect_uris":     []RedirectURI{},
	}
	if signingKey != "" {
		body["signing_key"] = signingKey
	}
	_, err := c.Patch(ctx, fmt.Sprintf("/api/v3/providers/oauth2/%d/", pk), body)
	if err != nil {
		return fmt.Errorf("updating provider %d MCP settings: %w", pk, err)
	}
	return nil
}

type CreateServiceAccountParams struct {
	Name        string
	CreateGroup bool
	Expiring    bool
	Expires     time.Time
}

type ServiceAccount struct {
	Username string `json:"username"`
	Token    string `json:"token"`
	UserUID  string `json:"user_uid"`
	UserPK   int    `json:"user_pk"`
	GroupPK  string `json:"group_pk"`
}

func (c *Client) CreateServiceAccount(ctx context.Context, params CreateServiceAccountParams) (*ServiceAccount, error) {
	body := map[string]any{
		"name":         params.Name,
		"create_group": params.CreateGroup,
		"expiring":     params.Expiring,
	}
	if !params.Expires.IsZero() {
		body["expires"] = params.Expires.Format(time.RFC3339)
	}
	data, err := c.Post(ctx, "/api/v3/core/users/service_account/", body)
	if err != nil {
		return nil, fmt.Errorf("creating service account %q: %w", params.Name, err)
	}
	var account ServiceAccount
	if err := json.Unmarshal(data, &account); err != nil {
		return nil, fmt.Errorf("parsing service account: %w", err)
	}
	return &account, nil
}
