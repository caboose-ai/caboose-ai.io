package mcp

import (
	"context"
	"testing"

	"github.com/caboose-ai/caboose-ai.io/internal/config"
	"github.com/caboose-ai/caboose-ai.io/internal/docker"
	"github.com/caboose-ai/caboose-ai.io/internal/health"
	"github.com/caboose-ai/caboose-ai.io/internal/service"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestServerIntegration(t *testing.T) {
	r := &mockRunner{output: []byte(`[{"Name":"forgejo","State":"running"}]`)}
	store := &mockSecretStore{data: map[string]string{
		"AUTHENTIK_SECRET_KEY": "test-key",
	}}

	s := &Server{
		cfg:        &config.Config{Domain: "test.local", ComposeDir: "/tmp/test"},
		runner:     r,
		http:       nil,
		compose:    docker.NewComposeClient(r, "/tmp/test"),
		dockerExec: docker.NewExecClient(r),
		secrets:    store,
		checkers: []health.Checker{
			&mockChecker{name: "Forgejo", err: nil},
		},
		services: []service.ServiceConfigurator{
			&mockServiceConfigurator{slug: "forgejo", name: "Forgejo", result: &service.ConfigureResult{Status: service.StatusAlreadyConfigured}},
		},
		mcpServer: sdkmcp.NewServer(
			&sdkmcp.Implementation{Name: "homelab-test", Version: "v0.0.1"},
			&sdkmcp.ServerOptions{
				Capabilities: &sdkmcp.ServerCapabilities{
					Tools:     &sdkmcp.ToolCapabilities{},
					Resources: &sdkmcp.ResourceCapabilities{},
					Prompts:   &sdkmcp.PromptCapabilities{},
				},
			},
		),
	}

	s.registerDockerTools()
	s.registerSecretsTools()
	s.registerMonitoringTools()
	s.registerAuthentikTools()
	s.registerResources()
	s.registerPrompts()

	ctx := context.Background()
	clientTransport, serverTransport := sdkmcp.NewInMemoryTransports()

	go func() {
		_, _ = s.mcpServer.Connect(ctx, serverTransport, nil)
	}()

	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer session.Close()

	t.Run("ListTools", func(t *testing.T) {
		count := 0
		for _, err := range session.Tools(ctx, nil) {
			if err != nil {
				t.Fatalf("listing tools: %v", err)
			}
			count++
		}
		if count != 20 {
			t.Errorf("expected 20 tools, got %d", count)
		}
	})

	t.Run("CallDockerPS", func(t *testing.T) {
		result, err := session.CallTool(ctx, &sdkmcp.CallToolParams{Name: "docker_ps"})
		if err != nil {
			t.Fatalf("call docker_ps: %v", err)
		}
		if len(result.Content) == 0 {
			t.Fatal("expected content in result")
		}
		tc, ok := result.Content[0].(*sdkmcp.TextContent)
		if !ok {
			t.Fatal("expected TextContent")
		}
		if tc.Text == "" {
			t.Error("expected non-empty text")
		}
	})

	t.Run("CallSecretsList", func(t *testing.T) {
		result, err := session.CallTool(ctx, &sdkmcp.CallToolParams{Name: "secrets_list"})
		if err != nil {
			t.Fatalf("call secrets_list: %v", err)
		}
		if len(result.Content) == 0 {
			t.Fatal("expected content in result")
		}
		tc := result.Content[0].(*sdkmcp.TextContent)
		if tc.Text == "" {
			t.Error("expected non-empty text")
		}
	})

	t.Run("CallHealthCheckAll", func(t *testing.T) {
		result, err := session.CallTool(ctx, &sdkmcp.CallToolParams{Name: "health_check_all"})
		if err != nil {
			t.Fatalf("call health_check_all: %v", err)
		}
		if len(result.Content) == 0 {
			t.Fatal("expected content in result")
		}
	})

	t.Run("ListResources", func(t *testing.T) {
		count := 0
		for _, err := range session.Resources(ctx, nil) {
			if err != nil {
				t.Fatalf("listing resources: %v", err)
			}
			count++
		}
		if count != 4 {
			t.Errorf("expected 4 resources, got %d", count)
		}
	})

	t.Run("ListPrompts", func(t *testing.T) {
		count := 0
		for _, err := range session.Prompts(ctx, nil) {
			if err != nil {
				t.Fatalf("listing prompts: %v", err)
			}
			count++
		}
		if count != 2 {
			t.Errorf("expected 2 prompts, got %d", count)
		}
	})
}
