package mcp

import (
	"context"
	"fmt"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type dockerRestartInput struct {
	Services []string `json:"services" jsonschema:"the Docker Compose services to restart"`
}

type dockerLogsInput struct {
	Service string `json:"service" jsonschema:"the Docker Compose service to get logs from"`
	Tail    int    `json:"tail,omitempty" jsonschema:"number of log lines to return (default 100)"`
}

func (s *Server) registerDockerTools() {
	sdkmcp.AddTool(s.mcpServer, &sdkmcp.Tool{
		Name:        "docker_ps",
		Description: "List all Docker Compose containers and their status",
	}, s.handleDockerPS)

	sdkmcp.AddTool(s.mcpServer, &sdkmcp.Tool{
		Name:        "docker_up",
		Description: "Start all Docker Compose services",
	}, s.handleDockerUp)

	sdkmcp.AddTool(s.mcpServer, &sdkmcp.Tool{
		Name:        "docker_down",
		Description: "Stop all Docker Compose services",
	}, s.handleDockerDown)

	sdkmcp.AddTool(s.mcpServer, &sdkmcp.Tool{
		Name:        "docker_restart",
		Description: "Restart specific Docker Compose services",
	}, s.handleDockerRestart)

	sdkmcp.AddTool(s.mcpServer, &sdkmcp.Tool{
		Name:        "docker_logs",
		Description: "Get recent logs from a Docker Compose service",
	}, s.handleDockerLogs)
}

func (s *Server) handleDockerPS(ctx context.Context, _ *sdkmcp.CallToolRequest, _ any) (*sdkmcp.CallToolResult, any, error) {
	if err := s.requireCompose(); err != nil {
		return nil, nil, err
	}
	out, err := s.compose.PS(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("docker_ps: %w", err)
	}
	return textResult(string(out)), nil, nil
}

func (s *Server) handleDockerUp(ctx context.Context, _ *sdkmcp.CallToolRequest, _ any) (*sdkmcp.CallToolResult, any, error) {
	if err := s.requireCompose(); err != nil {
		return nil, nil, err
	}
	out, err := s.compose.Up(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("docker_up: %w", err)
	}
	return textResult(fmt.Sprintf("Services started successfully.\n%s", string(out))), nil, nil
}

func (s *Server) handleDockerDown(ctx context.Context, _ *sdkmcp.CallToolRequest, _ any) (*sdkmcp.CallToolResult, any, error) {
	if err := s.requireCompose(); err != nil {
		return nil, nil, err
	}
	out, err := s.compose.Down(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("docker_down: %w", err)
	}
	return textResult(fmt.Sprintf("Services stopped successfully.\n%s", string(out))), nil, nil
}

func (s *Server) handleDockerRestart(ctx context.Context, _ *sdkmcp.CallToolRequest, input dockerRestartInput) (*sdkmcp.CallToolResult, any, error) {
	if err := s.requireCompose(); err != nil {
		return nil, nil, err
	}
	if len(input.Services) == 0 {
		return nil, nil, fmt.Errorf("docker_restart: at least one service name is required")
	}
	out, err := s.compose.Restart(ctx, input.Services...)
	if err != nil {
		return nil, nil, fmt.Errorf("docker_restart: %w", err)
	}
	return textResult(fmt.Sprintf("Restarted %v successfully.\n%s", input.Services, string(out))), nil, nil
}

func (s *Server) handleDockerLogs(ctx context.Context, _ *sdkmcp.CallToolRequest, input dockerLogsInput) (*sdkmcp.CallToolResult, any, error) {
	if err := s.requireCompose(); err != nil {
		return nil, nil, err
	}
	if input.Service == "" {
		return nil, nil, fmt.Errorf("docker_logs: service name is required")
	}
	tail := input.Tail
	if tail <= 0 {
		tail = 100
	}
	out, err := s.compose.Logs(ctx, input.Service, tail)
	if err != nil {
		return nil, nil, fmt.Errorf("docker_logs: %w", err)
	}
	return textResult(string(out)), nil, nil
}
