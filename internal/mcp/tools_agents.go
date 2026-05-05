package mcp

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type agentInvokeInput struct {
	Prompt       string   `json:"prompt" jsonschema:"prompt to send to the selected agent"`
	Preferred    []string `json:"preferred,omitempty" jsonschema:"ordered providers to try: ollama, claude, copilot, emberfall"`
	Model        string   `json:"model,omitempty" jsonschema:"model name for ollama (default llama3.1)"`
	OllamaPrompt string   `json:"ollama_prompt,omitempty" jsonschema:"optional override prompt for ollama"`
}

func (s *Server) registerAgentTools() {
	sdkmcp.AddTool(s.mcpServer, &sdkmcp.Tool{
		Name:        "agent_invoke",
		Description: "Invoke local agent providers with fallback (Ollama, Claude Code, Copilot CLI)",
	}, s.handleAgentInvoke)
}

func (s *Server) handleAgentInvoke(ctx context.Context, _ *sdkmcp.CallToolRequest, input agentInvokeInput) (*sdkmcp.CallToolResult, any, error) {
	if strings.TrimSpace(input.Prompt) == "" {
		return nil, nil, fmt.Errorf("agent_invoke: prompt is required")
	}

	providers := input.Preferred
	if len(providers) == 0 {
		providers = []string{"ollama", "claude", "copilot", "emberfall"}
	}

	model := strings.TrimSpace(input.Model)
	if model == "" {
		model = "llama3.1"
	}

	var errs []string
	for _, provider := range providers {
		provider = strings.ToLower(strings.TrimSpace(provider))
		if provider == "" {
			continue
		}

		out, err := s.invokeProvider(ctx, provider, model, input.Prompt)
		if err == nil {
			return textResult(fmt.Sprintf("provider: %s\n\n%s", provider, strings.TrimSpace(string(out)))), nil, nil
		}
		errs = append(errs, fmt.Sprintf("%s: %v", provider, err))
	}

	return nil, nil, fmt.Errorf("agent_invoke: all providers failed (%s)", strings.Join(errs, "; "))
}

func (s *Server) invokeProvider(ctx context.Context, provider, model, prompt string) ([]byte, error) {
	switch provider {
	case "ollama":
		return s.runner.Run(ctx, "ollama", "run", model, prompt)
	case "claude":
		return s.runner.Run(ctx, "claude", "-p", prompt)
	case "copilot":
		return s.runner.Run(ctx, "copilot", "chat", "--prompt", prompt)
	case "emberfall":
		return s.runner.RunWithStdin(ctx, bytes.NewBufferString(prompt), "emberfall", "--tests", "-")
	default:
		return nil, fmt.Errorf("unsupported provider %q", provider)
	}
}
