package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/caboose-ai/caboose-ai.io/internal/config"
	"github.com/caboose-ai/caboose-ai.io/internal/docker"
	"github.com/caboose-ai/caboose-ai.io/internal/health"
	"github.com/caboose-ai/caboose-ai.io/internal/runner"
	"github.com/caboose-ai/caboose-ai.io/internal/secrets"
	"github.com/caboose-ai/caboose-ai.io/internal/services"
	"github.com/caboose-ai/caboose-ai.io/internal/services/authentik"
	"github.com/caboose-ai/caboose-ai.io/internal/services/forgejo"
	"github.com/caboose-ai/caboose-ai.io/internal/services/grafana"
	"github.com/caboose-ai/caboose-ai.io/internal/services/mattermost"
	"github.com/caboose-ai/caboose-ai.io/internal/services/openwebui"
	"github.com/caboose-ai/caboose-ai.io/internal/services/portainer"
	"github.com/caboose-ai/caboose-ai.io/internal/services/social"
	"github.com/caboose-ai/caboose-ai.io/internal/services/woodpecker"
	"github.com/modelcontextprotocol/go-sdk/auth"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type Server struct {
	cfg        *config.Config
	runner     runner.CommandRunner
	http       runner.HTTPClient
	compose    *docker.ComposeClient
	dockerExec *docker.ExecClient
	secrets    secrets.SecretStore
	ak         *authentik.Client
	services   []services.ServiceConfigurator
	checkers   []health.Checker
	mcpServer  *sdkmcp.Server
}

func New(cfg *config.Config) *Server {
	r := runner.NewLocalRunner()
	httpClient := runner.NewHTTPClient()

	s := &Server{
		cfg:        cfg,
		runner:     r,
		http:       httpClient,
		compose:    docker.NewComposeClient(r, cfg.ComposeDir),
		dockerExec: docker.NewExecClient(r),
		secrets:    secrets.NewOnePasswordStore(cfg.OPVault, r, cfg.EnvPath()),
		mcpServer: sdkmcp.NewServer(
			&sdkmcp.Implementation{Name: "homelab", Version: "v1.0.0"},
			&sdkmcp.ServerOptions{
				Capabilities: &sdkmcp.ServerCapabilities{
					Tools:     &sdkmcp.ToolCapabilities{},
					Resources: &sdkmcp.ResourceCapabilities{},
					Prompts:   &sdkmcp.PromptCapabilities{},
				},
			},
		),
	}

	s.initAuthentik()
	s.registerDockerTools()
	s.registerSecretsTools()
	s.registerMonitoringTools()
	s.registerAuthentikTools()
	s.registerResources()
	s.registerPrompts()

	return s
}

func (s *Server) Run(ctx context.Context) error {
	return s.mcpServer.Run(ctx, &sdkmcp.StdioTransport{})
}

func (s *Server) RunHTTP(addr string) error {
	handler := sdkmcp.NewStreamableHTTPHandler(func(_ *http.Request) *sdkmcp.Server {
		return s.mcpServer
	}, nil)

	authMiddleware := auth.RequireBearerToken(s.verifyToken, &auth.RequireBearerTokenOptions{
		Scopes: []string{"homelab"},
	})

	protected := authMiddleware(handler)

	log.Printf("Homelab MCP server listening on %s (Authentik token auth enabled)", addr)
	return http.ListenAndServe(addr, protected)
}

func (s *Server) verifyToken(ctx context.Context, token string, _ *http.Request) (*auth.TokenInfo, error) {
	urls := s.cfg.URLs()
	introspectURL := urls.Authentik + "/application/o/introspect/"

	body := strings.NewReader("token=" + token)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, introspectURL, body)
	if err != nil {
		return nil, fmt.Errorf("creating introspection request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	clientID, _ := s.secrets.Get(ctx, "MCP_OAUTH_CLIENT_ID")
	clientSecret, _ := s.secrets.Get(ctx, "MCP_OAUTH_CLIENT_SECRET")
	if clientID != "" && clientSecret != "" {
		req.SetBasicAuth(clientID, clientSecret)
	}

	resp, err := s.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("introspecting token: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading introspection response: %w", err)
	}

	var introspection struct {
		Active   bool   `json:"active"`
		Username string `json:"username"`
		Scope    string `json:"scope"`
	}
	if err := json.Unmarshal(respBody, &introspection); err != nil {
		return nil, fmt.Errorf("parsing introspection response: %w", err)
	}

	if !introspection.Active {
		return nil, auth.ErrInvalidToken
	}

	var scopes []string
	if introspection.Scope != "" {
		scopes = strings.Fields(introspection.Scope)
	}

	return &auth.TokenInfo{
		UserID: introspection.Username,
		Scopes: scopes,
	}, nil
}

func (s *Server) initAuthentik() {
	ctx := context.Background()

	token, err := s.secrets.Get(ctx, "AUTHENTIK_BOOTSTRAP_TOKEN")
	if err != nil || token == "" {
		return
	}

	urls := s.cfg.URLs()
	s.ak = authentik.NewClient(urls.Authentik, token, s.http)

	s.buildServices(ctx)
	s.buildHealthCheckers()
}

func (s *Server) buildServices(ctx context.Context) {
	if s.ak == nil {
		return
	}
	urls := s.cfg.URLs()

	giteaAdminPass, _ := s.secrets.Get(ctx, "GITEA_ADMIN_PASSWORD")
	portainerAdminPass, _ := s.secrets.Get(ctx, "PORTAINER_ADMIN_PASSWORD")
	if portainerAdminPass == "" {
		portainerAdminPass = "admin"
	}

	s.services = []services.ServiceConfigurator{
		forgejo.New(s.ak, s.dockerExec, s.secrets, "forgejo", "auth-admin", urls.Authentik),
		woodpecker.New(s.dockerExec, s.http, s.secrets, "woodpecker-server", "auth-admin", giteaAdminPass, urls.Woodpecker+"/authorize"),
		portainer.New(s.ak, s.http, urls.Portainer, portainerAdminPass, urls.Authentik, urls.Portainer+"/"),
		grafana.New(s.ak, s.secrets),
		openwebui.New(s.ak, s.secrets),
		mattermost.New(s.ak, s.dockerExec, "mattermost", urls.Authentik),
		social.New(s.ak, s.cfg.Social),
	}
}

func (s *Server) buildHealthCheckers() {
	urls := s.cfg.URLs()

	headers := map[string]string{}
	if s.ak != nil {
		if token, _ := s.secrets.Get(context.Background(), "AUTHENTIK_BOOTSTRAP_TOKEN"); token != "" {
			headers["Authorization"] = "Bearer " + token
		}
	}

	s.checkers = []health.Checker{
		&health.HTTPChecker{ServiceName: "Authentik", URL: urls.Authentik + "/api/v3/root/config/", Client: s.http, Headers: headers},
		&health.HTTPChecker{ServiceName: "Forgejo", URL: urls.Forgejo, Client: s.http},
		&health.HTTPChecker{ServiceName: "Woodpecker", URL: urls.Woodpecker, Client: s.http},
		&health.HTTPChecker{ServiceName: "Portainer", URL: urls.Portainer, Client: s.http},
		&health.HTTPChecker{ServiceName: "Grafana", URL: urls.Grafana, Client: s.http},
		&health.HTTPChecker{ServiceName: "OpenWebUI", URL: urls.OpenWebUI, Client: s.http},
		&health.HTTPChecker{ServiceName: "Mattermost", URL: urls.Mattermost, Client: s.http},
	}
}

func (s *Server) requireAuthentik() error {
	if s.ak == nil {
		return fmt.Errorf("Authentik not initialized — run 'homelab install' to bootstrap the stack first")
	}
	return nil
}

func textResult(text string) *sdkmcp.CallToolResult {
	return &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{
			&sdkmcp.TextContent{Text: text},
		},
	}
}

func errResult(err error) *sdkmcp.CallToolResult {
	return &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{
			&sdkmcp.TextContent{Text: fmt.Sprintf("Error: %v", err)},
		},
		IsError: true,
	}
}
