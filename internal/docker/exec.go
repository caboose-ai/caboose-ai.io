package docker

import (
	"context"
	"fmt"

	"github.com/caboose-ai/caboose-ai.io/internal/runner"
)

type ExecClient struct {
	Runner runner.CommandRunner
}

func NewExecClient(r runner.CommandRunner) *ExecClient {
	return &ExecClient{Runner: r}
}

func (c *ExecClient) Exec(ctx context.Context, container string, cmd ...string) ([]byte, error) {
	args := append([]string{"exec", container}, cmd...)
	return c.Runner.Run(ctx, "docker", args...)
}

func (c *ExecClient) ExecAs(ctx context.Context, container, user string, cmd ...string) ([]byte, error) {
	args := append([]string{"exec", "-u", user, container}, cmd...)
	return c.Runner.Run(ctx, "docker", args...)
}

func (c *ExecClient) ContainerIP(ctx context.Context, container, network string) (string, error) {
	tmpl := fmt.Sprintf("{{range .NetworkSettings.Networks}}{{if eq .NetworkID %q}}{{.IPAddress}}{{end}}{{end}}", network)
	out, err := c.Runner.Run(ctx, "docker", "inspect", "--format", tmpl, container)
	if err != nil {
		out, err = c.Runner.Run(ctx, "docker", "inspect",
			"--format", "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}",
			container,
		)
		if err != nil {
			return "", fmt.Errorf("getting IP for %s: %w", container, err)
		}
	}
	return string(out), nil
}
