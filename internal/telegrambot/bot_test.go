package telegrambot

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"unicode/utf8"
)

type runCall struct {
	name string
	args []string
}

type fakeRunner struct {
	out     []byte
	err     error
	calls   []runCall
	runHook func(ctx context.Context, name string, args ...string) ([]byte, error)
}

func (r *fakeRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, runCall{name: name, args: append([]string{}, args...)})
	if r.runHook != nil {
		return r.runHook(ctx, name, args...)
	}
	return r.out, r.err
}

func (r *fakeRunner) RunWithStdin(ctx context.Context, stdin io.Reader, name string, args ...string) ([]byte, error) {
	return r.Run(ctx, name, args...)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) Do(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestConfigFromEnvLoadsDefaultsAndRejectsInvalidDefault(t *testing.T) {
	t.Setenv("TELEGRAM_BOT_TOKEN", "token")
	t.Setenv("TELEGRAM_ALLOWED_USER_IDS", "123, 456")
	t.Setenv("TELEGRAM_DEFAULT_MODEL", "")
	t.Setenv("TELEGRAM_ALLOWED_MODELS", "")
	t.Setenv("TELEGRAM_API_BASE", "")
	t.Setenv("TELEGRAM_REQUIRE_CONFIRMATION", "")

	cfg, err := ConfigFromEnv()
	if err != nil {
		t.Fatalf("ConfigFromEnv() error = %v", err)
	}
	if cfg.DefaultModel != "github-copilot/claude-opus-4.6" {
		t.Fatalf("DefaultModel = %q", cfg.DefaultModel)
	}
	if !cfg.AllowedUserIDs[123] || !cfg.AllowedUserIDs[456] {
		t.Fatalf("AllowedUserIDs = %#v", cfg.AllowedUserIDs)
	}
	if !cfg.RequireConfirmation {
		t.Fatal("RequireConfirmation = false, want true")
	}

	t.Setenv("TELEGRAM_DEFAULT_MODEL", "not-allowed")
	t.Setenv("TELEGRAM_ALLOWED_MODELS", "ollama/qwen3:8b")
	if _, err := ConfigFromEnv(); err == nil {
		t.Fatal("ConfigFromEnv() error = nil, want invalid default model error")
	}

	t.Setenv("TELEGRAM_DEFAULT_MODEL", "")
	t.Setenv("TELEGRAM_ALLOWED_MODELS", "")
	t.Setenv("TELEGRAM_REQUIRE_CONFIRMATION", "ture")
	if _, err := ConfigFromEnv(); err == nil {
		t.Fatal("ConfigFromEnv() error = nil, want invalid TELEGRAM_REQUIRE_CONFIRMATION error")
	}
}

func TestNewRejectsDefaultModelOutsideAllowlist(t *testing.T) {
	_, err := New(Config{
		BotToken:       "token",
		AllowedUserIDs: map[int64]bool{123: true},
		DefaultModel:   "not-allowed",
		AllowedModels:  []string{"ollama/qwen3:8b"},
	}, &fakeRunner{}, nil)
	if err == nil {
		t.Fatal("New() error = nil, want invalid default model error")
	}
}

func TestUnauthorizedUserCannotInvokeRunner(t *testing.T) {
	runner := &fakeRunner{out: []byte(`{"text":"ok"}`)}
	bot := newTestBot(t, runner)

	replies := bot.HandleMessage(context.Background(), Message{
		Text: "/ask hello",
		From: User{ID: 999},
		Chat: Chat{ID: 999},
	})

	if len(replies) != 1 || replies[0] != "Unauthorized Telegram user." {
		t.Fatalf("replies = %#v", replies)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("runner calls = %#v, want none", runner.calls)
	}
}

func TestGroupChatCannotInvokeRunner(t *testing.T) {
	runner := &fakeRunner{out: []byte(`{"text":"ok"}`)}
	bot := newTestBot(t, runner)

	replies := bot.HandleMessage(context.Background(), Message{
		Text: "/ask hello",
		From: User{ID: 123},
		Chat: Chat{ID: -100, Type: "group"},
	})

	if len(replies) != 1 || !strings.Contains(replies[0], "private chat") {
		t.Fatalf("replies = %#v", replies)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("runner calls = %#v, want none", runner.calls)
	}
}

func TestAskUsesSelectedModelAndExtractsResponseText(t *testing.T) {
	runner := &fakeRunner{out: []byte(`{"response":"model answer"}`)}
	bot := newTestBot(t, runner)

	if replies := bot.HandleMessage(context.Background(), allowedMessage("/model ollama/qwen3:14b")); len(replies) != 1 || !strings.Contains(replies[0], "ollama/qwen3:14b") {
		t.Fatalf("/model replies = %#v", replies)
	}
	replies := bot.HandleMessage(context.Background(), allowedMessage("/ask summarize status"))

	if len(replies) != 1 || replies[0] != "model answer" {
		t.Fatalf("/ask replies = %#v", replies)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("runner calls = %#v", runner.calls)
	}
	call := runner.calls[0]
	if call.name != "openclaw" {
		t.Fatalf("runner name = %q", call.name)
	}
	want := []string{"infer", "model", "run", "--gateway", "--json", "--model", "ollama/qwen3:14b", "--prompt", "summarize status"}
	if strings.Join(call.args, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("runner args = %#v, want %#v", call.args, want)
	}
}

func TestAgentDraftsPlanByDefaultForRiskyActions(t *testing.T) {
	runner := &fakeRunner{out: []byte(`{"text":"plan draft"}`)}
	bot := newTestBot(t, runner)

	replies := bot.HandleMessage(context.Background(), allowedMessage("/agent ops restart authentik"))
	if len(replies) != 1 || replies[0] != "plan draft" {
		t.Fatalf("replies = %#v", replies)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("runner calls = %#v", runner.calls)
	}

	prompt := runner.calls[0].args[len(runner.calls[0].args)-1]
	for _, want := range []string{
		"Draft a plan only",
		"Do not execute",
		"Task: restart authentik",
		"intended commands/tools",
		"affected files/services",
		"rollback notes",
		"verification",
		"required confirmation phrase",
		"Forgejo/Gitea branch or PR",
		"Woodpecker CI",
		"Portainer, MCP, or CLI",
		"human approval before restarts, deploys, or destructive operations",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
}

func TestAgentConfirmIsBoundedRemoteSignal(t *testing.T) {
	runner := &fakeRunner{out: []byte(`{"text":"confirmed plan update"}`)}
	bot := newTestBot(t, runner)

	replies := bot.HandleMessage(context.Background(), allowedMessage("/agent confirm ops restart authentik"))
	if len(replies) != 1 || replies[0] != "confirmed plan update" {
		t.Fatalf("confirmed replies = %#v", replies)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("runner calls = %#v", runner.calls)
	}
	prompt := runner.calls[0].args[len(runner.calls[0].args)-1]
	for _, want := range []string{
		"Remote confirmation signal received",
		"not durable approval",
		"Do not treat this as approval for broad Paperclip execution",
		"Task: restart authentik",
		"required confirmation phrase",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
}

func TestRunDropsPendingUpdatesBeforeProcessing(t *testing.T) {
	runner := &fakeRunner{out: []byte(`{"text":"fresh"}`)}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var requests []string
	client := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.URL.Path+"?"+req.URL.RawQuery)
		switch {
		case strings.Contains(req.URL.Path, "getUpdates"):
			timeout := req.URL.Query().Get("timeout")
			offset := req.URL.Query().Get("offset")
			switch {
			case timeout == "0" && offset == "":
				return telegramResponse(`{"ok":true,"result":[{"update_id":10,"message":{"text":"/agent confirm ops restart authentik","from":{"id":123},"chat":{"id":123,"type":"private"}}}]}`), nil
			case timeout == "0" && offset == "11":
				return telegramResponse(`{"ok":true,"result":[]}`), nil
			case timeout == "30" && offset == "11":
				return telegramResponse(`{"ok":true,"result":[{"update_id":11,"message":{"text":"/ask fresh","from":{"id":123},"chat":{"id":123,"type":"private"}}}]}`), nil
			default:
				t.Fatalf("unexpected getUpdates query: %s", req.URL.RawQuery)
			}
		case strings.Contains(req.URL.Path, "sendMessage"):
			cancel()
			return telegramResponse(`{"ok":true,"result":{}}`), nil
		default:
			t.Fatalf("unexpected request path: %s", req.URL.Path)
		}
		return nil, nil
	})
	bot, err := New(Config{
		BotToken:        "token",
		AllowedUserIDs:  map[int64]bool{123: true},
		TelegramAPIBase: "https://telegram.test",
	}, runner, client)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := bot.Run(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v, want context.Canceled", err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("runner calls = %#v, want one fresh update call; requests=%#v", runner.calls, requests)
	}
	if gotPrompt := runner.calls[0].args[len(runner.calls[0].args)-1]; gotPrompt != "fresh" {
		t.Fatalf("prompt = %q, want fresh", gotPrompt)
	}
}

func TestRunContinuesAfterSendMessageFailure(t *testing.T) {
	runner := &fakeRunner{out: []byte(`{"text":"ok"}`)}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sendCount := 0
	client := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case strings.Contains(req.URL.Path, "getUpdates"):
			timeout := req.URL.Query().Get("timeout")
			offset := req.URL.Query().Get("offset")
			switch {
			case timeout == "0":
				return telegramResponse(`{"ok":true,"result":[]}`), nil
			case timeout == "30" && offset == "":
				return telegramResponse(`{"ok":true,"result":[{"update_id":1,"message":{"text":"/ask first","from":{"id":123},"chat":{"id":123,"type":"private"}}}]}`), nil
			case timeout == "30" && offset == "2":
				return telegramResponse(`{"ok":true,"result":[{"update_id":2,"message":{"text":"/ask second","from":{"id":123},"chat":{"id":123,"type":"private"}}}]}`), nil
			default:
				t.Fatalf("unexpected getUpdates query: %s", req.URL.RawQuery)
			}
		case strings.Contains(req.URL.Path, "sendMessage"):
			sendCount++
			if sendCount == 1 {
				return &http.Response{
					StatusCode: http.StatusBadRequest,
					Body:       io.NopCloser(strings.NewReader(`{"ok":false}`)),
				}, nil
			}
			cancel()
			return telegramResponse(`{"ok":true,"result":{}}`), nil
		default:
			t.Fatalf("unexpected request path: %s", req.URL.Path)
		}
		return nil, nil
	})

	bot, err := New(Config{
		BotToken:        "token",
		AllowedUserIDs:  map[int64]bool{123: true},
		TelegramAPIBase: "https://telegram.test",
	}, runner, client)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := bot.Run(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v, want context.Canceled", err)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("runner calls = %#v, want two command invocations", runner.calls)
	}
}

func TestStatusRunsOpenClawModelStatus(t *testing.T) {
	runner := &fakeRunner{out: []byte(`{"ready":true}`)}
	bot := newTestBot(t, runner)

	replies := bot.HandleMessage(context.Background(), allowedMessage("/status"))

	if len(replies) != 1 || !strings.Contains(replies[0], `{"ready":true}`) {
		t.Fatalf("/status replies = %#v", replies)
	}
	want := []string{"models", "status", "--json"}
	if len(runner.calls) != 1 || strings.Join(runner.calls[0].args, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("runner calls = %#v, want args %#v", runner.calls, want)
	}
}

func telegramResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestSendTextSplitsAndPostsToTelegram(t *testing.T) {
	var bodies []string
	client := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		data, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("reading request body: %v", err)
		}
		bodies = append(bodies, string(data))
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
		}, nil
	})
	bot, err := New(Config{
		BotToken:        "token",
		AllowedUserIDs:  map[int64]bool{123: true},
		TelegramAPIBase: "https://telegram.test",
	}, &fakeRunner{}, client)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := bot.SendText(context.Background(), 123, strings.Repeat("x", maxTelegramMessage+10)); err != nil {
		t.Fatalf("SendText() error = %v", err)
	}

	if len(bodies) != 2 {
		t.Fatalf("sent message count = %d, want 2", len(bodies))
	}
	if !strings.Contains(bodies[0], "chat_id=123") || !strings.Contains(bodies[1], "chat_id=123") {
		t.Fatalf("request bodies = %#v", bodies)
	}
}

func TestSplitTelegramKeepsUTF8Boundaries(t *testing.T) {
	input := strings.Repeat("界", maxTelegramMessage+1)
	chunks := splitTelegram(input)
	if len(chunks) != 2 {
		t.Fatalf("chunk count = %d, want 2", len(chunks))
	}
	for i, chunk := range chunks {
		if !utf8.ValidString(chunk) {
			t.Fatalf("chunk %d is invalid UTF-8", i)
		}
	}
}

func TestAgentConfirmWithoutArgumentsReturnsUsage(t *testing.T) {
	runner := &fakeRunner{out: []byte(`{"text":"ok"}`)}
	bot := newTestBot(t, runner)
	replies := bot.HandleMessage(context.Background(), allowedMessage("/agent confirm"))
	if len(replies) != 1 || !strings.Contains(replies[0], "Usage: /agent <role> <task>") {
		t.Fatalf("replies = %#v", replies)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("runner calls = %#v, want none", runner.calls)
	}
}

func TestInvokeUsesTimeoutContext(t *testing.T) {
	runner := &fakeRunner{
		runHook: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			if _, ok := ctx.Deadline(); !ok {
				t.Fatal("runner context has no deadline")
			}
			return []byte(`{"text":"ok"}`), nil
		},
	}
	bot := newTestBot(t, runner)
	replies := bot.HandleMessage(context.Background(), allowedMessage("/ask hello"))
	if len(replies) != 1 || replies[0] != "ok" {
		t.Fatalf("replies = %#v", replies)
	}
}

func newTestBot(t *testing.T, runner *fakeRunner) *Bot {
	t.Helper()
	bot, err := New(Config{
		BotToken:            "token",
		AllowedUserIDs:      map[int64]bool{123: true},
		DefaultModel:        "github-copilot/claude-opus-4.6",
		RequireConfirmation: true,
		AllowedModels: []string{
			"github-copilot/claude-opus-4.6",
			"github-copilot/gpt-5.3-codex",
			"ollama/qwen3:14b",
			"ollama/qwen3:8b",
		},
	}, runner, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return bot
}

func allowedMessage(text string) Message {
	return Message{
		Text: text,
		From: User{ID: 123},
		Chat: Chat{ID: 123, Type: "private"},
	}
}
