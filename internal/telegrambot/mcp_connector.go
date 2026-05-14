package telegrambot

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type CommandMCPConnector struct {
	command string
	args    []string
	workDir string
	timeout time.Duration
}

func NewCommandMCPConnector(command string, args []string, workDir string, timeout time.Duration) *CommandMCPConnector {
	return &CommandMCPConnector{
		command: command,
		args:    append([]string(nil), args...),
		workDir: workDir,
		timeout: timeout,
	}
}

func (c *CommandMCPConnector) CallTool(ctx context.Context, name string, args map[string]any) (string, error) {
	return c.withSession(ctx, func(ctx context.Context, session *sdkmcp.ClientSession) (string, error) {
		result, err := session.CallTool(ctx, &sdkmcp.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			return "", err
		}
		text := toolText(result)
		if result.IsError {
			return text, fmt.Errorf("mcp tool %s returned an error: %s", name, text)
		}
		return text, nil
	})
}

func (c *CommandMCPConnector) ReadResource(ctx context.Context, uri string) (string, error) {
	return c.withSession(ctx, func(ctx context.Context, session *sdkmcp.ClientSession) (string, error) {
		result, err := session.ReadResource(ctx, &sdkmcp.ReadResourceParams{URI: uri})
		if err != nil {
			return "", err
		}
		var parts []string
		for _, content := range result.Contents {
			if strings.TrimSpace(content.Text) != "" {
				parts = append(parts, strings.TrimSpace(content.Text))
			}
		}
		return strings.Join(parts, "\n\n"), nil
	})
}

func (c *CommandMCPConnector) withSession(ctx context.Context, fn func(context.Context, *sdkmcp.ClientSession) (string, error)) (string, error) {
	if strings.TrimSpace(c.command) == "" {
		return "", fmt.Errorf("mcp command is required")
	}
	if c.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, c.command, c.args...)
	if c.workDir != "" {
		cmd.Dir = c.workDir
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "telegram-agent", Version: "v0.0.0"}, nil)
	transport := &sdkmcp.CommandTransport{
		Command:           cmd,
		TerminateDuration: 2 * time.Second,
	}
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return "", formatMCPError("connect", err, stderr.String())
	}
	defer session.Close()

	text, err := fn(ctx, session)
	if err != nil {
		return text, formatMCPError("request", err, stderr.String())
	}
	return text, nil
}

func toolText(result *sdkmcp.CallToolResult) string {
	if result == nil {
		return ""
	}
	var parts []string
	for _, content := range result.Content {
		if text, ok := content.(*sdkmcp.TextContent); ok && strings.TrimSpace(text.Text) != "" {
			parts = append(parts, strings.TrimSpace(text.Text))
		}
	}
	return strings.Join(parts, "\n\n")
}

func formatMCPError(stage string, err error, stderr string) error {
	stderr = strings.TrimSpace(stderr)
	if stderr == "" {
		return fmt.Errorf("mcp %s: %w", stage, err)
	}
	return fmt.Errorf("mcp %s: %w\n%s", stage, err, stderr)
}
