package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/caboose-ai/caboose-ai.io/internal/health"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type healthCheckInput struct {
	Services []string `json:"services,omitempty" jsonschema:"specific services to check (default: all)"`
}

type prometheusQueryInput struct {
	Query string `json:"query" jsonschema:"PromQL query to execute against Prometheus"`
}

type containerStatsInput struct {
	Service string `json:"service,omitempty" jsonschema:"specific container to get stats for (default: all)"`
}

func (s *Server) registerMonitoringTools() {
	sdkmcp.AddTool(s.mcpServer, &sdkmcp.Tool{
		Name:        "health_check",
		Description: "Check health of specific homelab services",
	}, s.handleHealthCheck)

	sdkmcp.AddTool(s.mcpServer, &sdkmcp.Tool{
		Name:        "health_check_all",
		Description: "Check health of all homelab services and return a full stack report",
	}, s.handleHealthCheckAll)

	sdkmcp.AddTool(s.mcpServer, &sdkmcp.Tool{
		Name:        "prometheus_query",
		Description: "Execute a PromQL query against the Prometheus instance",
	}, s.handlePrometheusQuery)

	sdkmcp.AddTool(s.mcpServer, &sdkmcp.Tool{
		Name:        "container_stats",
		Description: "Get CPU, memory, and network stats for Docker containers",
	}, s.handleContainerStats)
}

type healthResult struct {
	Name    string `json:"name"`
	Healthy bool   `json:"healthy"`
	Error   string `json:"error,omitempty"`
	Latency string `json:"latency"`
}

func (s *Server) handleHealthCheck(ctx context.Context, _ *sdkmcp.CallToolRequest, input healthCheckInput) (*sdkmcp.CallToolResult, any, error) {
	checkers := s.checkers
	if len(input.Services) > 0 {
		checkers = filterCheckers(s.checkers, input.Services)
		if len(checkers) == 0 {
			return nil, nil, fmt.Errorf("health_check: no matching services found for %v", input.Services)
		}
	}
	if len(checkers) == 0 {
		return nil, nil, fmt.Errorf("health_check: no health checkers configured — is Authentik bootstrapped?")
	}

	results := runHealthChecks(ctx, checkers)
	data, _ := json.MarshalIndent(results, "", "  ")
	return textResult(string(data)), nil, nil
}

func (s *Server) handleHealthCheckAll(ctx context.Context, _ *sdkmcp.CallToolRequest, _ any) (*sdkmcp.CallToolResult, any, error) {
	if len(s.checkers) == 0 {
		return nil, nil, fmt.Errorf("health_check_all: no health checkers configured — is Authentik bootstrapped?")
	}

	results := runHealthChecks(ctx, s.checkers)
	data, _ := json.MarshalIndent(results, "", "  ")
	return textResult(string(data)), nil, nil
}

func (s *Server) handlePrometheusQuery(ctx context.Context, _ *sdkmcp.CallToolRequest, input prometheusQueryInput) (*sdkmcp.CallToolResult, any, error) {
	if input.Query == "" {
		return nil, nil, fmt.Errorf("prometheus_query: query is required")
	}

	reqURL := fmt.Sprintf("http://localhost:9090/api/v1/query?query=%s", url.QueryEscape(input.Query))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("prometheus_query: %w", err)
	}

	resp, err := s.http.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("prometheus_query: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("prometheus_query: reading response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, nil, fmt.Errorf("prometheus_query: HTTP %d: %s", resp.StatusCode, string(body))
	}
	return textResult(string(body)), nil, nil
}

func (s *Server) handleContainerStats(ctx context.Context, _ *sdkmcp.CallToolRequest, input containerStatsInput) (*sdkmcp.CallToolResult, any, error) {
	args := []string{"stats", "--no-stream", "--format", "json"}
	if input.Service != "" {
		args = append(args, input.Service)
	}

	out, err := s.runner.Run(ctx, "docker", args...)
	if err != nil {
		return nil, nil, fmt.Errorf("container_stats: %w", err)
	}
	return textResult(string(out)), nil, nil
}

func filterCheckers(checkers []health.Checker, names []string) []health.Checker {
	nameSet := make(map[string]bool)
	for _, n := range names {
		nameSet[strings.ToLower(n)] = true
	}
	var filtered []health.Checker
	for _, c := range checkers {
		if nameSet[strings.ToLower(c.Name())] {
			filtered = append(filtered, c)
		}
	}
	return filtered
}

func runHealthChecks(ctx context.Context, checkers []health.Checker) []healthResult {
	results := make([]healthResult, 0, len(checkers))
	for _, c := range checkers {
		start := time.Now()
		err := c.Check(ctx)
		elapsed := time.Since(start)

		r := healthResult{
			Name:    c.Name(),
			Healthy: err == nil,
			Latency: elapsed.Round(time.Millisecond).String(),
		}
		if err != nil {
			r.Error = err.Error()
		}
		results = append(results, r)
	}
	return results
}
