package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/caboose-ai/caboose-ai.io/internal/config"
	"github.com/caboose-ai/caboose-ai.io/internal/runner"
	"github.com/caboose-ai/caboose-ai.io/internal/telegrambot"
	"gopkg.in/yaml.v3"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "telegram-agent: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := telegrambot.ConfigFromEnv()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "notify":
			bot, err := telegrambot.New(cfg, runner.NewLocalRunner(), nil)
			if err != nil {
				return err
			}
			return notify(ctx, bot, os.Args[2:])
		default:
			return fmt.Errorf("unknown command %q; use no arguments to run the bot or `notify <message>` to send a completion message", os.Args[1])
		}
	}

	lab, cleanupLab, err := labFromEnv()
	if err != nil {
		return err
	}
	defer cleanupLab()
	cfg.Lab = lab

	bot, err := telegrambot.New(cfg, runner.NewLocalRunner(), nil)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "telegram-agent: starting long poll with %d allowed user(s); default model %s\n", len(cfg.AllowedUserIDs), cfg.DefaultModel)
	err = bot.Run(ctx)
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

func notify(ctx context.Context, bot *telegrambot.Bot, args []string) error {
	message := strings.TrimSpace(strings.Join(args, " "))
	if message == "" {
		return errors.New("notify requires a message")
	}
	chatIDs, err := notificationChatIDs(bot)
	if err != nil {
		return err
	}
	var failures []string
	for _, chatID := range chatIDs {
		if err := bot.SendText(ctx, chatID, message); err != nil {
			failures = append(failures, fmt.Sprintf("%d: %v", chatID, err))
			continue
		}
		fmt.Fprintf(os.Stderr, "telegram-agent: sent notification to %d\n", chatID)
	}
	if len(failures) > 0 {
		return fmt.Errorf("sending Telegram message failed for %d chat(s): %s", len(failures), strings.Join(failures, "; "))
	}
	return nil
}

func notificationChatIDs(bot *telegrambot.Bot) ([]int64, error) {
	raw := strings.TrimSpace(os.Getenv("TELEGRAM_NOTIFY_CHAT_ID"))
	if raw == "" {
		return bot.AllowedChatIDs(), nil
	}
	allowed := map[int64]bool{}
	for _, id := range bot.AllowedChatIDs() {
		allowed[id] = true
	}
	ids := make([]int64, 0)
	for _, field := range strings.Split(raw, ",") {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		id, err := strconv.ParseInt(field, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid TELEGRAM_NOTIFY_CHAT_ID value %q", field)
		}
		if !allowed[id] {
			return nil, fmt.Errorf("TELEGRAM_NOTIFY_CHAT_ID value %d is not allowlisted", id)
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil, errors.New("TELEGRAM_NOTIFY_CHAT_ID did not contain any chat IDs")
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids, nil
}

func labFromEnv() (telegrambot.LabService, func(), error) {
	mcpTimeout, err := envDuration("HOMELAB_MCP_TIMEOUT", 2*time.Minute)
	if err != nil {
		return nil, nil, err
	}
	configPath, cleanup, err := resolveMCPConfigPath(strings.TrimSpace(os.Getenv("HOMELAB_MCP_CONFIG")))
	if err != nil {
		return nil, nil, err
	}
	command, args := mcpServerCommand(strings.TrimSpace(os.Getenv("HOMELAB_MCP_BINARY")), configPath)
	workDir := strings.TrimSpace(os.Getenv("HOMELAB_MCP_WORKDIR"))
	connector := telegrambot.NewCommandMCPConnector(command, args, workDir, mcpTimeout)
	return telegrambot.NewLabMCP(connector), cleanup, nil
}

func envDuration(key string, fallback time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be a duration: %w", key, err)
	}
	return value, nil
}

func resolveMCPConfigPath(path string) (string, func(), error) {
	if path != "" {
		return path, func() {}, nil
	}

	cfg := config.DefaultConfig()
	cfg.Domain = envDefault("HOMELAB_DOMAIN", "caboose-ai.io")
	cfg.ComposeDir = envDefault("HOMELAB_COMPOSE_DIR", cfg.ComposeDir)
	cfg.ServeMode = envDefault("HOMELAB_SERVE_MODE", cfg.ServeMode)
	cfg.Email = envDefault("HOMELAB_EMAIL", cfg.Email)
	cfg.OPVault = envDefault("HOMELAB_OP_VAULT", cfg.OPVault)
	cfg.OPStaticVault = envDefault("HOMELAB_OP_STATIC_VAULT", cfg.OPStaticVault)

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return "", nil, fmt.Errorf("generating homelab MCP config: %w", err)
	}
	file, err := os.CreateTemp("", "telegram-homelab-mcp-*.yml")
	if err != nil {
		return "", nil, fmt.Errorf("creating temporary homelab MCP config: %w", err)
	}
	cleanup := func() {
		_ = os.Remove(file.Name())
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		cleanup()
		return "", nil, fmt.Errorf("writing temporary homelab MCP config: %w", err)
	}
	if err := file.Close(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("closing temporary homelab MCP config: %w", err)
	}
	return file.Name(), cleanup, nil
}

func mcpServerCommand(binary, configPath string) (string, []string) {
	if binary != "" {
		return binary, []string{"--config", configPath}
	}
	return "go", []string{"run", "-buildvcs=false", "./cmd/mcp", "--config", configPath}
}

func envDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}
