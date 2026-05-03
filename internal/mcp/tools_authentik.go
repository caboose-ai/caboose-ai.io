package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/caboose-ai/caboose-ai.io/internal/services"
	"github.com/caboose-ai/caboose-ai.io/internal/services/authentik"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type authentikGetProviderInput struct {
	Name string `json:"name" jsonschema:"name of the OAuth2 provider to look up"`
}

type authentikUpsertSourceInput struct {
	Name           string `json:"name" jsonschema:"display name for the OAuth source"`
	Slug           string `json:"slug" jsonschema:"URL-safe identifier (e.g. github, google)"`
	ProviderType   string `json:"provider_type" jsonschema:"OAuth provider type (e.g. github, google-oauth2)"`
	ConsumerKey    string `json:"consumer_key" jsonschema:"OAuth client ID"`
	ConsumerSecret string `json:"consumer_secret" jsonschema:"OAuth client secret"`
}

type authentikFindUserInput struct {
	Username string `json:"username" jsonschema:"username to search for in Authentik"`
}

type authentikRenameUserInput struct {
	CurrentUsername string `json:"current_username" jsonschema:"current username"`
	NewUsername     string `json:"new_username" jsonschema:"desired new username"`
}

type ssoConfigureServiceInput struct {
	Service string `json:"service" jsonschema:"service slug: forgejo, woodpecker, portainer, grafana, open-webui, mattermost, social"`
	DryRun  bool   `json:"dry_run,omitempty" jsonschema:"if true, show what would happen without making changes"`
	Force   bool   `json:"force,omitempty" jsonschema:"if true, re-create existing configuration"`
}

func (s *Server) registerAuthentikTools() {
	sdkmcp.AddTool(s.mcpServer, &sdkmcp.Tool{
		Name:        "authentik_list_providers",
		Description: "List all OAuth2 providers configured in Authentik",
	}, s.handleAuthentikListProviders)

	sdkmcp.AddTool(s.mcpServer, &sdkmcp.Tool{
		Name:        "authentik_get_provider",
		Description: "Get details of a specific OAuth2 provider by name",
	}, s.handleAuthentikGetProvider)

	sdkmcp.AddTool(s.mcpServer, &sdkmcp.Tool{
		Name:        "authentik_list_sources",
		Description: "List all OAuth sources (e.g. GitHub, Google) configured in Authentik",
	}, s.handleAuthentikListSources)

	sdkmcp.AddTool(s.mcpServer, &sdkmcp.Tool{
		Name:        "authentik_upsert_source",
		Description: "Create or update an OAuth source in Authentik (e.g. GitHub or Google login)",
	}, s.handleAuthentikUpsertSource)

	sdkmcp.AddTool(s.mcpServer, &sdkmcp.Tool{
		Name:        "authentik_find_user",
		Description: "Search for a user in Authentik by username",
	}, s.handleAuthentikFindUser)

	sdkmcp.AddTool(s.mcpServer, &sdkmcp.Tool{
		Name:        "authentik_rename_user",
		Description: "Rename a user in Authentik",
	}, s.handleAuthentikRenameUser)

	sdkmcp.AddTool(s.mcpServer, &sdkmcp.Tool{
		Name:        "sso_configure_service",
		Description: "Run the full SSO/OAuth configuration for a specific homelab service",
	}, s.handleSSOConfigureService)
}

func (s *Server) handleAuthentikListProviders(ctx context.Context, _ *sdkmcp.CallToolRequest, _ any) (*sdkmcp.CallToolResult, any, error) {
	if err := s.requireAuthentik(); err != nil {
		return nil, nil, err
	}
	providers, err := s.ak.ListProviders(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("authentik_list_providers: %w", err)
	}
	data, _ := json.MarshalIndent(providers, "", "  ")
	return textResult(string(data)), nil, nil
}

func (s *Server) handleAuthentikGetProvider(ctx context.Context, _ *sdkmcp.CallToolRequest, input authentikGetProviderInput) (*sdkmcp.CallToolResult, any, error) {
	if err := s.requireAuthentik(); err != nil {
		return nil, nil, err
	}
	if input.Name == "" {
		return nil, nil, fmt.Errorf("authentik_get_provider: name is required")
	}
	provider, err := s.ak.GetProvider(ctx, input.Name)
	if err != nil {
		return nil, nil, fmt.Errorf("authentik_get_provider: %w", err)
	}
	data, _ := json.MarshalIndent(provider, "", "  ")
	return textResult(string(data)), nil, nil
}

func (s *Server) handleAuthentikListSources(ctx context.Context, _ *sdkmcp.CallToolRequest, _ any) (*sdkmcp.CallToolResult, any, error) {
	if err := s.requireAuthentik(); err != nil {
		return nil, nil, err
	}
	sources, err := s.ak.ListSources(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("authentik_list_sources: %w", err)
	}
	data, _ := json.MarshalIndent(sources, "", "  ")
	return textResult(string(data)), nil, nil
}

func (s *Server) handleAuthentikUpsertSource(ctx context.Context, _ *sdkmcp.CallToolRequest, input authentikUpsertSourceInput) (*sdkmcp.CallToolResult, any, error) {
	if err := s.requireAuthentik(); err != nil {
		return nil, nil, err
	}
	if input.Slug == "" || input.ConsumerKey == "" || input.ConsumerSecret == "" {
		return nil, nil, fmt.Errorf("authentik_upsert_source: slug, consumer_key, and consumer_secret are required")
	}
	params := authentik.UpsertSourceParams{
		Name:           input.Name,
		Slug:           input.Slug,
		Enabled:        true,
		ProviderType:   input.ProviderType,
		ConsumerKey:    input.ConsumerKey,
		ConsumerSecret: input.ConsumerSecret,
	}
	if err := s.ak.UpsertSource(ctx, params); err != nil {
		return nil, nil, fmt.Errorf("authentik_upsert_source: %w", err)
	}
	return textResult(fmt.Sprintf("OAuth source %q upserted successfully", input.Slug)), nil, nil
}

func (s *Server) handleAuthentikFindUser(ctx context.Context, _ *sdkmcp.CallToolRequest, input authentikFindUserInput) (*sdkmcp.CallToolResult, any, error) {
	if err := s.requireAuthentik(); err != nil {
		return nil, nil, err
	}
	if input.Username == "" {
		return nil, nil, fmt.Errorf("authentik_find_user: username is required")
	}
	user, err := s.ak.FindUser(ctx, input.Username)
	if err != nil {
		return nil, nil, fmt.Errorf("authentik_find_user: %w", err)
	}
	if user == nil {
		return textResult(fmt.Sprintf("User %q not found", input.Username)), nil, nil
	}
	data, _ := json.MarshalIndent(user, "", "  ")
	return textResult(string(data)), nil, nil
}

func (s *Server) handleAuthentikRenameUser(ctx context.Context, _ *sdkmcp.CallToolRequest, input authentikRenameUserInput) (*sdkmcp.CallToolResult, any, error) {
	if err := s.requireAuthentik(); err != nil {
		return nil, nil, err
	}
	if input.CurrentUsername == "" || input.NewUsername == "" {
		return nil, nil, fmt.Errorf("authentik_rename_user: both current_username and new_username are required")
	}
	if err := s.ak.RenameAdminUser(ctx, input.CurrentUsername, input.NewUsername); err != nil {
		return nil, nil, fmt.Errorf("authentik_rename_user: %w", err)
	}
	return textResult(fmt.Sprintf("User %q renamed to %q", input.CurrentUsername, input.NewUsername)), nil, nil
}

func (s *Server) handleSSOConfigureService(ctx context.Context, _ *sdkmcp.CallToolRequest, input ssoConfigureServiceInput) (*sdkmcp.CallToolResult, any, error) {
	if err := s.requireAuthentik(); err != nil {
		return nil, nil, err
	}
	if input.Service == "" {
		return nil, nil, fmt.Errorf("sso_configure_service: service is required")
	}

	var svc services.ServiceConfigurator
	for _, sc := range s.services {
		if strings.EqualFold(sc.Slug(), input.Service) {
			svc = sc
			break
		}
	}
	if svc == nil {
		slugs := make([]string, len(s.services))
		for i, sc := range s.services {
			slugs[i] = sc.Slug()
		}
		return nil, nil, fmt.Errorf("sso_configure_service: unknown service %q — valid services: %s", input.Service, strings.Join(slugs, ", "))
	}

	opts := services.ConfigureOpts{DryRun: input.DryRun, Force: input.Force}
	result, err := svc.Configure(ctx, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("sso_configure_service(%s): %w", input.Service, err)
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	return textResult(string(data)), nil, nil
}
