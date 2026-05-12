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

	"github.com/caboose-ai/caboose-ai.io/internal/runner"
	"github.com/caboose-ai/caboose-ai.io/internal/telegrambot"
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
	bot, err := telegrambot.New(cfg, runner.NewLocalRunner(), nil)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "notify":
			return notify(ctx, bot, os.Args[2:])
		default:
			return fmt.Errorf("unknown command %q; use no arguments to run the bot or `notify <message>` to send a completion message", os.Args[1])
		}
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
	for _, chatID := range chatIDs {
		if err := bot.SendText(ctx, chatID, message); err != nil {
			return fmt.Errorf("sending Telegram message to %d: %w", chatID, err)
		}
		fmt.Fprintf(os.Stderr, "telegram-agent: sent notification to %d\n", chatID)
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
