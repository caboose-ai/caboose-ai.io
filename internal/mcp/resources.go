package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/caboose-ai/caboose-ai.io/internal/config"
	"github.com/caboose-ai/caboose-ai.io/internal/service"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *Server) registerResources() {
	s.registerStaticResource("homelab://config/docker-compose", "Docker Compose Config", "Full docker-compose.yml service definitions", "docker-compose.yml")
	s.registerStaticResource("homelab://config/env-example", "Environment Template", ".env.example with all configurable variables", ".env.example")
	s.registerStaticResource("homelab://config/prometheus", "Prometheus Config", "Prometheus scrape configuration", "prometheus.yml")

	s.mcpServer.AddResource(&sdkmcp.Resource{
		URI:         "homelab://config/urls",
		Name:        "Service URLs",
		Description: "All service URLs derived from domain",
		MIMEType:    "application/json",
	}, func(_ context.Context, req *sdkmcp.ReadResourceRequest) (*sdkmcp.ReadResourceResult, error) {
		urls := config.DeriveURLs(s.cfg.Domain)
		data, _ := json.MarshalIndent(urls, "", "  ")
		return &sdkmcp.ReadResourceResult{
			Contents: []*sdkmcp.ResourceContents{{URI: req.Params.URI, Text: string(data)}},
		}, nil
	})

	s.mcpServer.AddResourceTemplate(&sdkmcp.ResourceTemplate{
		URITemplate: "homelab://services/{service}/status",
		Name:        "Service Status",
		Description: "Combined status for a service: container running, health, SSO configured",
	}, s.handleServiceStatusResource)

	s.mcpServer.AddResourceTemplate(&sdkmcp.ResourceTemplate{
		URITemplate: "homelab://services/{service}/manifest",
		Name:        "Service Manifest",
		Description: "Service workspace manifest from services/<slug>/service.yaml",
	}, s.handleServiceManifestResource)
}

func (s *Server) registerStaticResource(uri, name, description, filename string) {
	s.mcpServer.AddResource(&sdkmcp.Resource{
		URI:         uri,
		Name:        name,
		Description: description,
		MIMEType:    "text/plain",
	}, func(_ context.Context, req *sdkmcp.ReadResourceRequest) (*sdkmcp.ReadResourceResult, error) {
		path := filepath.Join(s.cfg.ComposeDir, filename)
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", filename, err)
		}
		return &sdkmcp.ReadResourceResult{
			Contents: []*sdkmcp.ResourceContents{{URI: req.Params.URI, Text: string(data)}},
		}, nil
	})
}

type serviceStatus struct {
	Service    string `json:"service"`
	Running    bool   `json:"running"`
	Healthy    *bool  `json:"healthy,omitempty"`
	Configured *bool  `json:"configured,omitempty"`
	Error      string `json:"error,omitempty"`
}

func (s *Server) handleServiceStatusResource(ctx context.Context, req *sdkmcp.ReadResourceRequest) (*sdkmcp.ReadResourceResult, error) {
	service := extractServiceFromURI(req.Params.URI)
	if service == "" {
		return nil, fmt.Errorf("could not extract service name from URI %q", req.Params.URI)
	}

	status := serviceStatus{Service: service}

	composeService := service
	if s.registry != nil {
		if manifest, ok := s.registry.Manifest(service); ok && len(manifest.ComposeServices) > 0 {
			composeService = manifest.ComposeServices[0]
		}
	}

	running, err := s.compose.IsRunning(ctx, composeService)
	if err != nil {
		status.Error = err.Error()
	} else {
		status.Running = running
	}

	for _, c := range s.checkers {
		if strings.EqualFold(c.Name(), service) {
			healthy := c.Check(ctx) == nil
			status.Healthy = &healthy
			break
		}
	}

	for _, svc := range s.configurators() {
		if strings.EqualFold(svc.Slug(), service) || strings.EqualFold(svc.Name(), service) {
			configured, _ := svc.CheckConfigured(ctx)
			status.Configured = &configured
			break
		}
	}

	data, _ := json.MarshalIndent(status, "", "  ")
	return &sdkmcp.ReadResourceResult{
		Contents: []*sdkmcp.ResourceContents{{URI: req.Params.URI, Text: string(data)}},
	}, nil
}

func (s *Server) handleServiceManifestResource(_ context.Context, req *sdkmcp.ReadResourceRequest) (*sdkmcp.ReadResourceResult, error) {
	service := extractServiceManifestFromURI(req.Params.URI)
	if service == "" {
		return nil, fmt.Errorf("could not extract service name from URI %q", req.Params.URI)
	}
	if s.registry == nil {
		return nil, fmt.Errorf("service registry is not initialized")
	}
	manifest, ok := s.registry.Manifest(service)
	if !ok {
		return nil, fmt.Errorf("unknown service %q", service)
	}
	data, _ := json.MarshalIndent(manifest, "", "  ")
	return &sdkmcp.ReadResourceResult{
		Contents: []*sdkmcp.ResourceContents{{URI: req.Params.URI, Text: string(data)}},
	}, nil
}

func extractServiceFromURI(uri string) string {
	const prefix = "homelab://services/"
	const suffix = "/status"
	if !strings.HasPrefix(uri, prefix) || !strings.HasSuffix(uri, suffix) {
		return ""
	}
	return strings.TrimSuffix(strings.TrimPrefix(uri, prefix), suffix)
}

func extractServiceManifestFromURI(uri string) string {
	const prefix = "homelab://services/"
	const suffix = "/manifest"
	if !strings.HasPrefix(uri, prefix) || !strings.HasSuffix(uri, suffix) {
		return ""
	}
	return strings.TrimSuffix(strings.TrimPrefix(uri, prefix), suffix)
}

func (s *Server) registerPrompts() {
	s.mcpServer.AddPrompt(&sdkmcp.Prompt{
		Name:        "diagnose-service",
		Description: "Gather all diagnostic information for a service (status, health, logs, SSO config)",
		Arguments: []*sdkmcp.PromptArgument{
			{Name: "service", Description: "Service name to diagnose", Required: true},
		},
	}, s.handleDiagnoseServicePrompt)

	s.mcpServer.AddPrompt(&sdkmcp.Prompt{
		Name:        "full-stack-report",
		Description: "Generate a comprehensive status report for all homelab services",
	}, s.handleFullStackReportPrompt)
}

func (s *Server) handleDiagnoseServicePrompt(ctx context.Context, req *sdkmcp.GetPromptRequest) (*sdkmcp.GetPromptResult, error) {
	service := req.Params.Arguments["service"]
	if service == "" {
		return nil, fmt.Errorf("service argument is required")
	}

	var parts []string
	parts = append(parts, fmt.Sprintf("# Diagnostic Report: %s\n", service))

	running, err := s.compose.IsRunning(ctx, service)
	if err != nil {
		parts = append(parts, fmt.Sprintf("## Container Status\nError checking: %v\n", err))
	} else {
		parts = append(parts, fmt.Sprintf("## Container Status\nRunning: %v\n", running))
	}

	for _, c := range s.checkers {
		if strings.EqualFold(c.Name(), service) {
			if err := c.Check(ctx); err != nil {
				parts = append(parts, fmt.Sprintf("## Health Check\nUnhealthy: %v\n", err))
			} else {
				parts = append(parts, "## Health Check\nHealthy\n")
			}
			break
		}
	}

	logs, err := s.compose.Logs(ctx, service, 50)
	if err != nil {
		parts = append(parts, fmt.Sprintf("## Recent Logs\nError fetching: %v\n", err))
	} else {
		parts = append(parts, fmt.Sprintf("## Recent Logs (last 50 lines)\n```\n%s\n```\n", string(logs)))
	}

	for _, svc := range s.configurators() {
		if strings.EqualFold(svc.Slug(), service) || strings.EqualFold(svc.Name(), service) {
			configured, err := svc.CheckConfigured(ctx)
			if err != nil {
				parts = append(parts, fmt.Sprintf("## SSO Configuration\nError checking: %v\n", err))
			} else {
				parts = append(parts, fmt.Sprintf("## SSO Configuration\nConfigured: %v\n", configured))
			}
			break
		}
	}

	return &sdkmcp.GetPromptResult{
		Description: fmt.Sprintf("Diagnostic report for %s", service),
		Messages: []*sdkmcp.PromptMessage{
			{
				Role:    "user",
				Content: &sdkmcp.TextContent{Text: strings.Join(parts, "\n")},
			},
		},
	}, nil
}

func (s *Server) handleFullStackReportPrompt(ctx context.Context, _ *sdkmcp.GetPromptRequest) (*sdkmcp.GetPromptResult, error) {
	var parts []string
	parts = append(parts, "# Full Stack Report\n")

	psOut, err := s.compose.PS(ctx)
	if err != nil {
		parts = append(parts, fmt.Sprintf("## Container Status\nError: %v\n", err))
	} else {
		parts = append(parts, fmt.Sprintf("## Container Status\n```json\n%s\n```\n", string(psOut)))
	}

	if len(s.checkers) > 0 {
		parts = append(parts, "## Health Checks")
		for _, c := range s.checkers {
			if err := c.Check(ctx); err != nil {
				parts = append(parts, fmt.Sprintf("- %s: UNHEALTHY (%v)", c.Name(), err))
			} else {
				parts = append(parts, fmt.Sprintf("- %s: healthy", c.Name()))
			}
		}
		parts = append(parts, "")
	}

	if len(s.configurators()) > 0 {
		parts = append(parts, "## SSO Configuration")
		for _, svc := range s.configurators() {
			configured, err := svc.CheckConfigured(ctx)
			if err != nil {
				parts = append(parts, fmt.Sprintf("- %s: error (%v)", svc.Name(), err))
			} else if configured {
				parts = append(parts, fmt.Sprintf("- %s: configured", svc.Name()))
			} else {
				parts = append(parts, fmt.Sprintf("- %s: NOT configured", svc.Name()))
			}
		}
		parts = append(parts, "")
	}

	return &sdkmcp.GetPromptResult{
		Description: "Full homelab stack status report",
		Messages: []*sdkmcp.PromptMessage{
			{
				Role:    "user",
				Content: &sdkmcp.TextContent{Text: strings.Join(parts, "\n")},
			},
		},
	}, nil
}

func (s *Server) configurators() []service.ServiceConfigurator {
	if s.registry == nil {
		return s.services
	}
	configurators := make([]service.ServiceConfigurator, 0, len(s.registry.ConfigurableSlugs()))
	for _, slug := range s.registry.ConfigurableSlugs() {
		if configurator, ok := s.registry.Configurator(slug); ok {
			configurators = append(configurators, configurator)
		}
	}
	return configurators
}
