package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/caboose-ai/caboose-ai.io/internal/docker"
)

type RecoveryOptions struct {
	Domain     string
	Username   string
	Minutes    int
	ComposeDir string
}

func RunRecovery(ctx context.Context, exec *docker.ExecClient, opts RecoveryOptions) (string, error) {
	const container = "authentik-server"

	if opts.Username == "" {
		opts.Username = "akadmin"
	}
	if opts.Minutes <= 0 {
		opts.Minutes = 10
	}
	if opts.Domain == "" {
		opts.Domain = "caboose-ai.io"
	}

	running, err := exec.ContainerRunning(ctx, container)
	if err != nil {
		return "", fmt.Errorf("checking authentik container %q: %w", container, err)
	}
	if !running {
		return "", fmt.Errorf("authentik container %q is not running; start it with: %s", container, recoveryStartHint(opts.ComposeDir))
	}

	out, err := exec.Exec(ctx, container, "ak", "create_recovery_key", fmt.Sprintf("%d", opts.Minutes), opts.Username)
	if err != nil {
		return "", fmt.Errorf("creating recovery key: %w", err)
	}

	token := parseRecoveryToken(strings.TrimSpace(string(out)))
	if token == "" {
		return "", fmt.Errorf("could not parse recovery token from output: %s", string(out))
	}

	return fmt.Sprintf("https://auth.%s/recovery/use-token/%s/", opts.Domain, token), nil
}

func recoveryStartHint(composeDir string) string {
	if composeDir == "" {
		return "mise run homelab:install"
	}
	return fmt.Sprintf("docker compose -f %s/docker-compose.yml up -d authentik-server", composeDir)
}

func parseRecoveryToken(output string) string {
	prefix := "/recovery/use-token/"
	idx := strings.Index(output, prefix)
	if idx < 0 {
		return ""
	}
	rest := output[idx+len(prefix):]
	end := strings.Index(rest, "/")
	if end < 0 {
		return strings.TrimSpace(rest)
	}
	return rest[:end]
}
