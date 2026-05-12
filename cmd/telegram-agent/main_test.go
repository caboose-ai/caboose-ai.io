package main

import (
	"context"
	"io"
	"strconv"
	"strings"
	"testing"

	"github.com/caboose-ai/caboose-ai.io/internal/telegrambot"
)

type stubRunner struct{}

func (r stubRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return nil, nil
}

func (r stubRunner) RunWithStdin(ctx context.Context, stdin io.Reader, name string, args ...string) ([]byte, error) {
	return r.Run(ctx, name, args...)
}

func TestNotificationChatIDsRejectsNonAllowedOverride(t *testing.T) {
	t.Setenv("TELEGRAM_NOTIFY_CHAT_ID", "999")
	bot := newNotifyTestBot(t)

	_, err := notificationChatIDs(bot)

	if err == nil || !strings.Contains(err.Error(), "not allowlisted") {
		t.Fatalf("notificationChatIDs() error = %v, want not allowlisted", err)
	}
}

func TestNotificationChatIDsAcceptsAllowedOverride(t *testing.T) {
	t.Setenv("TELEGRAM_NOTIFY_CHAT_ID", "456,123")
	bot := newNotifyTestBot(t)

	ids, err := notificationChatIDs(bot)

	if err != nil {
		t.Fatalf("notificationChatIDs() error = %v", err)
	}
	if got := strings.Join([]string{strconv.FormatInt(ids[0], 10), strconv.FormatInt(ids[1], 10)}, ","); got != "123,456" {
		t.Fatalf("ids = %v, want [123 456]", ids)
	}
}

func newNotifyTestBot(t *testing.T) *telegrambot.Bot {
	t.Helper()
	bot, err := telegrambot.New(telegrambot.Config{
		BotToken:       "token",
		AllowedUserIDs: map[int64]bool{123: true, 456: true},
	}, stubRunner{}, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return bot
}
