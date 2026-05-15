package telegrambot

import (
	"context"
	"fmt"
	"strings"
)

type MCPConnector interface {
	CallTool(ctx context.Context, name string, args map[string]any) (string, error)
	ReadResource(ctx context.Context, uri string) (string, error)
}

type LabMCP struct {
	connector MCPConnector
}

func NewLabMCP(connector MCPConnector) *LabMCP {
	return &LabMCP{connector: connector}
}

func (l *LabMCP) Status(ctx context.Context) (string, error) {
	return l.call(ctx, "health_check_all", nil)
}

func (l *LabMCP) Docker(ctx context.Context) (string, error) {
	return l.call(ctx, "docker_ps", nil)
}

func (l *LabMCP) ServiceStatus(ctx context.Context, slug string) (string, error) {
	slug = strings.TrimSpace(slug)
	if !isServiceSlug(slug) {
		return "", fmt.Errorf("lab service: invalid service slug %q", slug)
	}
	return l.read(ctx, "homelab://services/"+slug+"/status")
}

func (l *LabMCP) Ask(ctx context.Context, prompt string) (string, error) {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return "", fmt.Errorf("lab ask: prompt is required")
	}
	return l.call(ctx, "agent_invoke", map[string]any{"prompt": prompt})
}

func (l *LabMCP) call(ctx context.Context, name string, args map[string]any) (string, error) {
	if l.connector == nil {
		return "", fmt.Errorf("lab MCP connector is not configured")
	}
	return l.connector.CallTool(ctx, name, args)
}

func (l *LabMCP) read(ctx context.Context, uri string) (string, error) {
	if l.connector == nil {
		return "", fmt.Errorf("lab MCP connector is not configured")
	}
	return l.connector.ReadResource(ctx, uri)
}

func isServiceSlug(slug string) bool {
	if slug == "" {
		return false
	}
	for _, r := range slug {
		if r >= 'a' && r <= 'z' {
			continue
		}
		if r >= '0' && r <= '9' {
			continue
		}
		if r == '-' {
			continue
		}
		return false
	}
	return true
}
