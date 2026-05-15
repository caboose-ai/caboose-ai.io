package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	running, err := composeServiceIsRunning(out, service)
	if err != nil {
		return false, fmt.Errorf("parsing compose ps for service %s: %w", service, err)
	}
	return running, nil
}

type composePSRow struct {
	Name    string `json:"Name"`
	Service string `json:"Service"`
	State   string `json:"State"`
}

func composeServiceIsRunning(out []byte, service string) (bool, error) {
	rows, err := parseComposePSRows(out)
	if err != nil {
		return false, err
	}
	for _, row := range rows {
		if row.Service != service && row.Name != service {
			continue
		}
		return row.State == "" || strings.EqualFold(row.State, "running"), nil
	}
	return false, nil
}

func parseComposePSRows(out []byte) ([]composePSRow, error) {
	trimmed := bytes.TrimSpace(out)
	if len(trimmed) == 0 {
		return nil, nil
	}
	if trimmed[0] == '[' {
		var rows []composePSRow
		if err := json.Unmarshal(trimmed, &rows); err != nil {
			return nil, err
		}
		return rows, nil
	}

	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	var rows []composePSRow
	for {
		var row composePSRow
		if err := decoder.Decode(&row); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		rows = append(rows, row)
	}
	return rows, nil
}
