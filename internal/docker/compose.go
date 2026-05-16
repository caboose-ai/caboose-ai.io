package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/caboose-ai/caboose-ai.io/internal/runner"
	"gopkg.in/yaml.v3"
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
	profiles, err := c.profiles()
	if err != nil {
		return nil, err
	}
	return c.runWithProfiles(ctx, profiles, "down", "-v", "--remove-orphans")
}

func (c *ComposeClient) UpService(ctx context.Context, services ...string) ([]byte, error) {
	args := append([]string{"up", "-d"}, services...)
	return c.run(ctx, args...)
}

func (c *ComposeClient) UpProfileServices(ctx context.Context, profile string, services ...string) ([]byte, error) {
	args := append([]string{"up", "-d"}, services...)
	return c.runWithProfiles(ctx, []string{profile}, args...)
}

func (c *ComposeClient) Restart(ctx context.Context, services ...string) ([]byte, error) {
	args := append([]string{"restart"}, services...)
	return c.run(ctx, args...)
}

func (c *ComposeClient) PS(ctx context.Context) ([]byte, error) {
	return c.run(ctx, "ps", "--format=json")
}

func (c *ComposeClient) run(ctx context.Context, args ...string) ([]byte, error) {
	return c.runWithProfiles(ctx, nil, args...)
}

func (c *ComposeClient) runWithProfiles(ctx context.Context, profiles []string, args ...string) ([]byte, error) {
	fullArgs := []string{"compose", "-f", c.composePath()}
	for _, profile := range profiles {
		fullArgs = append(fullArgs, "--profile", profile)
	}
	fullArgs = append(fullArgs, args...)
	return c.Runner.Run(ctx, "docker", fullArgs...)
}

func (c *ComposeClient) composePath() string {
	return c.ComposeDir + "/docker-compose.yml"
}

func (c *ComposeClient) profiles() ([]string, error) {
	data, err := os.ReadFile(c.composePath())
	if err != nil {
		return nil, fmt.Errorf("reading compose profiles: %w", err)
	}

	var compose struct {
		Services map[string]struct {
			Profiles []string `yaml:"profiles"`
		} `yaml:"services"`
	}
	if err := yaml.Unmarshal(data, &compose); err != nil {
		return nil, fmt.Errorf("parsing compose profiles: %w", err)
	}

	seen := map[string]bool{}
	for _, service := range compose.Services {
		for _, profile := range service.Profiles {
			if profile != "" {
				seen[profile] = true
			}
		}
	}

	profiles := make([]string, 0, len(seen))
	for profile := range seen {
		profiles = append(profiles, profile)
	}
	sort.Strings(profiles)
	return profiles, nil
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
