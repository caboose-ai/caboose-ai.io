package docker

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/caboose-ai/caboose-ai.io/internal/runner"
)

type ComposeClient struct {
	Runner     runner.CommandRunner
	ComposeDir string
}

func NewComposeClient(r runner.CommandRunner, composeDir string) *ComposeClient {
	return &ComposeClient{Runner: r, ComposeDir: composeDir}
}

func (c *ComposeClient) Up(ctx context.Context) ([]byte, error) {
	return c.run(ctx, "up", "-d")
}

func (c *ComposeClient) Down(ctx context.Context) ([]byte, error) {
	return c.run(ctx, "down")
}

func (c *ComposeClient) DownWithVolumes(ctx context.Context) ([]byte, error) {
	return c.run(ctx, "down", "-v")
}

func (c *ComposeClient) UpService(ctx context.Context, services ...string) ([]byte, error) {
	args := append([]string{"up", "-d"}, services...)
	return c.run(ctx, args...)
}

func (c *ComposeClient) Restart(ctx context.Context, services ...string) ([]byte, error) {
	args := append([]string{"restart"}, services...)
	return c.run(ctx, args...)
}

func (c *ComposeClient) PS(ctx context.Context) ([]byte, error) {
	return c.run(ctx, "ps", "--format=json")
}

func (c *ComposeClient) run(ctx context.Context, args ...string) ([]byte, error) {
	fullArgs := append([]string{"compose", "-f", c.ComposeDir + "/docker-compose.yml"}, args...)
	return c.Runner.Run(ctx, "docker", fullArgs...)
}

func (c *ComposeClient) Logs(ctx context.Context, service string, tail int) ([]byte, error) {
	return c.run(ctx, "logs", "--no-color", "--tail", strconv.Itoa(tail), service)
}

func (c *ComposeClient) IsRunning(ctx context.Context, service string) (bool, error) {
	out, err := c.PS(ctx)
	if err != nil {
		return false, fmt.Errorf("checking service %s: %w", service, err)
	}
	return strings.Contains(string(out), service), nil
}
