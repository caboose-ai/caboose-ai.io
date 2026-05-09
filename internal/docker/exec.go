package docker

import (
	"context"
	"fmt"
	"io"
	"strings"

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

func (c *ExecClient) ContainerRunning(ctx context.Context, container string) (bool, error) {
	out, err := c.Runner.Run(ctx, "docker", "inspect", "--format", "{{.State.Running}}", container)
	if err != nil {
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "no such object") || strings.Contains(msg, "no such container") {
			return false, nil
		}
		return false, err
	}
	return strings.TrimSpace(string(out)) == "true", nil
}

func (c *ExecClient) ExecAs(ctx context.Context, container, user string, cmd ...string) ([]byte, error) {
	args := append([]string{"exec", "-u", user, container}, cmd...)
	return c.Runner.Run(ctx, "docker", args...)
}

func (c *ExecClient) ExecWithStdin(ctx context.Context, container string, stdin io.Reader, cmd ...string) ([]byte, error) {
	args := append([]string{"exec", "-i", container}, cmd...)
	return c.Runner.RunWithStdin(ctx, stdin, "docker", args...)
}

func (c *ExecClient) CopyTo(ctx context.Context, src, container, dest string) ([]byte, error) {
	return c.Runner.Run(ctx, "docker", "cp", src, container+":"+dest)
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
