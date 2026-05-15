package telegrambot

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

type fakeLabService struct {
	status string
	docker string
	ask    string

	serviceSlug string
	askPrompt   string
}

func (f *fakeLabService) Status(context.Context) (string, error) {
	return f.status, nil
}

func (f *fakeLabService) Docker(context.Context) (string, error) {
	return f.docker, nil
}

func (f *fakeLabService) ServiceStatus(_ context.Context, slug string) (string, error) {
	f.serviceSlug = slug
	return "service " + slug, nil
}

func (f *fakeLabService) Ask(_ context.Context, prompt string) (string, error) {
	f.askPrompt = prompt
	return f.ask, nil
}

func TestBotLabCommandsUseConfiguredLabService(t *testing.T) {
	lab := &fakeLabService{status: "lab healthy", docker: "containers", ask: "agent answer"}
	bot := newTestBotWithLab(t, lab)

	got := []string{
		onlyReply(t, bot.HandleMessage(context.Background(), allowedMessage("/lab status"))),
		onlyReply(t, bot.HandleMessage(context.Background(), allowedMessage("/lab docker"))),
		onlyReply(t, bot.HandleMessage(context.Background(), allowedMessage("/lab service forgejo"))),
		onlyReply(t, bot.HandleMessage(context.Background(), allowedMessage("/lab ask summarize the lab"))),
	}
	want := []string{"lab healthy", "containers", "service forgejo", "agent answer"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("replies = %#v, want %#v", got, want)
	}
	if lab.serviceSlug != "forgejo" {
		t.Fatalf("service slug = %q, want forgejo", lab.serviceSlug)
	}
	if lab.askPrompt != "summarize the lab" {
		t.Fatalf("ask prompt = %q, want summarize the lab", lab.askPrompt)
	}
}

func TestBotLabCommandRequiresConfiguredLabService(t *testing.T) {
	bot := newTestBot(t, &fakeRunner{})

	reply := onlyReply(t, bot.HandleMessage(context.Background(), allowedMessage("/lab status")))

	if !strings.Contains(reply, "Lab MCP is not configured") {
		t.Fatalf("reply = %q, want MCP configuration error", reply)
	}
}

type fakeMCPConnector struct {
	toolName string
	args     map[string]any
	uri      string
	result   string
}

func (f *fakeMCPConnector) CallTool(_ context.Context, name string, args map[string]any) (string, error) {
	f.toolName = name
	f.args = args
	return f.result, nil
}

func (f *fakeMCPConnector) ReadResource(_ context.Context, uri string) (string, error) {
	f.uri = uri
	return f.result, nil
}

func TestLabMCPUsesAllowedMCPCalls(t *testing.T) {
	tests := []struct {
		name     string
		run      func(context.Context, *LabMCP) (string, error)
		toolName string
		toolArgs map[string]any
		resource string
	}{
		{
			name:     "status",
			run:      func(ctx context.Context, lab *LabMCP) (string, error) { return lab.Status(ctx) },
			toolName: "health_check_all",
		},
		{
			name:     "docker",
			run:      func(ctx context.Context, lab *LabMCP) (string, error) { return lab.Docker(ctx) },
			toolName: "docker_ps",
		},
		{
			name:     "service",
			run:      func(ctx context.Context, lab *LabMCP) (string, error) { return lab.ServiceStatus(ctx, "forgejo") },
			resource: "homelab://services/forgejo/status",
		},
		{
			name:     "ask",
			run:      func(ctx context.Context, lab *LabMCP) (string, error) { return lab.Ask(ctx, "what is down?") },
			toolName: "agent_invoke",
			toolArgs: map[string]any{"prompt": "what is down?"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			connector := &fakeMCPConnector{result: "ok"}
			lab := NewLabMCP(connector)
			got, err := tt.run(context.Background(), lab)
			if err != nil {
				t.Fatalf("run returned error: %v", err)
			}
			if got != "ok" {
				t.Fatalf("result = %q, want ok", got)
			}
			if connector.toolName != tt.toolName {
				t.Fatalf("tool = %q, want %q", connector.toolName, tt.toolName)
			}
			if !reflect.DeepEqual(connector.args, tt.toolArgs) {
				t.Fatalf("args = %#v, want %#v", connector.args, tt.toolArgs)
			}
			if connector.uri != tt.resource {
				t.Fatalf("resource = %q, want %q", connector.uri, tt.resource)
			}
		})
	}
}

func TestLabMCPRejectsInvalidServiceSlug(t *testing.T) {
	lab := NewLabMCP(&fakeMCPConnector{result: "ok"})

	_, err := lab.ServiceStatus(context.Background(), "../secrets")

	if err == nil || !strings.Contains(err.Error(), "invalid service slug") {
		t.Fatalf("ServiceStatus() error = %v, want invalid service slug", err)
	}
}

func newTestBotWithLab(t *testing.T, lab LabService) *Bot {
	t.Helper()
	bot, err := New(Config{
		BotToken:            "token",
		AllowedUserIDs:      map[int64]bool{123: true},
		DefaultModel:        "github-copilot/claude-opus-4.6",
		RequireConfirmation: true,
		AllowedModels: []string{
			"github-copilot/claude-opus-4.6",
			"github-copilot/gpt-5.3-codex",
			"ollama/qwen3:14b",
			"ollama/qwen3:8b",
		},
		Lab: lab,
	}, &fakeRunner{}, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return bot
}

func onlyReply(t *testing.T, replies []string) string {
	t.Helper()
	if len(replies) != 1 {
		t.Fatalf("replies = %#v, want one reply", replies)
	}
	return replies[0]
}
