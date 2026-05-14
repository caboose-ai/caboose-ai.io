package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/caboose-ai/caboose-ai.io/internal/config"
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

type notifyHTTPClient struct {
	seen []int64
}

func (c *notifyHTTPClient) Do(req *http.Request) (*http.Response, error) {
	if !strings.Contains(req.URL.Path, "/sendMessage") {
		return nil, errors.New("unexpected endpoint")
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	values, err := url.ParseQuery(string(body))
	if err != nil {
		return nil, err
	}
	chatID, err := strconv.ParseInt(values.Get("chat_id"), 10, 64)
	if err != nil {
		return nil, err
	}
	c.seen = append(c.seen, chatID)
	if chatID == 123 {
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Body:       io.NopCloser(strings.NewReader(`{"ok":false}`)),
		}, nil
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
	}, nil
}

func TestNotifyContinuesAfterSingleChatFailure(t *testing.T) {
	t.Setenv("TELEGRAM_NOTIFY_CHAT_ID", "123,456")
	client := &notifyHTTPClient{}
	bot, err := telegrambot.New(telegrambot.Config{
		BotToken:            "token",
		AllowedUserIDs:      map[int64]bool{123: true, 456: true},
		TelegramAPIBase:     "https://telegram.test",
		RequireConfirmation: true,
	}, stubRunner{}, client)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	err = notify(context.Background(), bot, []string{"build complete"})
	if err == nil || !strings.Contains(err.Error(), "123") {
		t.Fatalf("notify() error = %v, want failure mentioning chat 123", err)
	}
	if len(client.seen) != 2 {
		t.Fatalf("send attempts = %v, want both chats attempted", client.seen)
	}
}

func TestResolveMCPConfigPathUsesRunnableDefaults(t *testing.T) {
	t.Setenv("HOMELAB_DOMAIN", "")

	path, cleanup, err := resolveMCPConfigPath("")
	if err != nil {
		t.Fatalf("resolveMCPConfigPath() error = %v", err)
	}
	defer cleanup()

	cfg, err := config.LoadFromFile(path)
	if err != nil {
		t.Fatalf("LoadFromFile(%q): %v", path, err)
	}
	if cfg.Domain != "caboose-ai.io" {
		t.Fatalf("Domain = %q, want caboose-ai.io", cfg.Domain)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("generated MCP config does not validate: %v", err)
	}
}

func TestEnvDurationRejectsInvalidValue(t *testing.T) {
	t.Setenv("HOMELAB_MCP_TIMEOUT", "soon")

	_, err := envDuration("HOMELAB_MCP_TIMEOUT", 2*time.Minute)

	if err == nil || !strings.Contains(err.Error(), "HOMELAB_MCP_TIMEOUT") {
		t.Fatalf("envDuration() error = %v, want HOMELAB_MCP_TIMEOUT error", err)
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
