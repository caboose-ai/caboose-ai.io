package telegrambot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/caboose-ai/caboose-ai.io/internal/runner"
)

const (
	defaultTelegramAPIBase = "https://api.telegram.org"
	defaultPollTimeout     = 30
	openClawInvokeTimeout  = 2 * time.Minute
	maxTelegramMessage     = 3900
)

var defaultAllowedModels = []string{
	"github-copilot/claude-opus-4.6",
	"github-copilot/gpt-5.3-codex",
	"ollama/qwen3:14b",
	"ollama/qwen3:8b",
}

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type Config struct {
	BotToken            string
	AllowedUserIDs      map[int64]bool
	DefaultModel        string
	AllowedModels       []string
	TelegramAPIBase     string
	RequireConfirmation bool
}

func ConfigFromEnv() (Config, error) {
	requireConfirmation, err := envBool("TELEGRAM_REQUIRE_CONFIRMATION", true)
	if err != nil {
		return Config{}, err
	}
	cfg := Config{
		BotToken:            strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN")),
		DefaultModel:        strings.TrimSpace(os.Getenv("TELEGRAM_DEFAULT_MODEL")),
		TelegramAPIBase:     strings.TrimSpace(os.Getenv("TELEGRAM_API_BASE")),
		RequireConfirmation: requireConfirmation,
	}
	if cfg.BotToken == "" {
		return Config{}, errors.New("TELEGRAM_BOT_TOKEN is required")
	}
	if cfg.TelegramAPIBase == "" {
		cfg.TelegramAPIBase = defaultTelegramAPIBase
	}
	if cfg.DefaultModel == "" {
		cfg.DefaultModel = defaultAllowedModels[0]
	}
	cfg.AllowedModels = splitCSV(os.Getenv("TELEGRAM_ALLOWED_MODELS"))
	if len(cfg.AllowedModels) == 0 {
		cfg.AllowedModels = append([]string{}, defaultAllowedModels...)
	}
	cfg.AllowedUserIDs = map[int64]bool{}
	for _, raw := range splitCSV(os.Getenv("TELEGRAM_ALLOWED_USER_IDS")) {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return Config{}, fmt.Errorf("invalid TELEGRAM_ALLOWED_USER_IDS value %q", raw)
		}
		cfg.AllowedUserIDs[id] = true
	}
	if len(cfg.AllowedUserIDs) == 0 {
		return Config{}, errors.New("TELEGRAM_ALLOWED_USER_IDS is required")
	}
	if !contains(cfg.AllowedModels, cfg.DefaultModel) {
		return Config{}, fmt.Errorf("TELEGRAM_DEFAULT_MODEL %q is not in TELEGRAM_ALLOWED_MODELS", cfg.DefaultModel)
	}
	return cfg, nil
}

type Bot struct {
	cfg       Config
	runner    runner.CommandRunner
	http      HTTPClient
	userModel map[int64]string
	mu        sync.Mutex
}

func New(cfg Config, commandRunner runner.CommandRunner, httpClient HTTPClient) (*Bot, error) {
	if cfg.TelegramAPIBase == "" {
		cfg.TelegramAPIBase = defaultTelegramAPIBase
	}
	if cfg.DefaultModel == "" {
		cfg.DefaultModel = defaultAllowedModels[0]
	}
	if len(cfg.AllowedModels) == 0 {
		cfg.AllowedModels = append([]string{}, defaultAllowedModels...)
	}
	if cfg.AllowedUserIDs == nil {
		cfg.AllowedUserIDs = map[int64]bool{}
	}
	if len(cfg.AllowedUserIDs) == 0 {
		return nil, errors.New("at least one allowed Telegram user ID is required")
	}
	if !contains(cfg.AllowedModels, cfg.DefaultModel) {
		return nil, fmt.Errorf("default model %q is not in the allowed model list", cfg.DefaultModel)
	}
	if cfg.BotToken == "" {
		return nil, errors.New("telegram bot token is required")
	}
	if commandRunner == nil {
		return nil, errors.New("command runner is required")
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 35 * time.Second}
	}
	return &Bot{
		cfg:       cfg,
		runner:    commandRunner,
		http:      httpClient,
		userModel: map[int64]string{},
	}, nil
}

func (b *Bot) AllowedChatIDs() []int64 {
	ids := make([]int64, 0, len(b.cfg.AllowedUserIDs))
	for id := range b.cfg.AllowedUserIDs {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

type User struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
}

type Chat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
}

type Message struct {
	MessageID int    `json:"message_id"`
	Text      string `json:"text"`
	From      User   `json:"from"`
	Chat      Chat   `json:"chat"`
}

type Update struct {
	UpdateID int64    `json:"update_id"`
	Message  *Message `json:"message"`
}

func (b *Bot) Run(ctx context.Context) error {
	offset, err := b.discardPendingUpdates(ctx)
	for err != nil {
		if sleepErr := sleepContext(ctx, 2*time.Second); sleepErr != nil {
			return sleepErr
		}
		offset, err = b.discardPendingUpdates(ctx)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		updates, err := b.getUpdates(ctx, offset)
		if err != nil {
			if sleepErr := sleepContext(ctx, 2*time.Second); sleepErr != nil {
				return sleepErr
			}
			continue
		}
		for _, update := range updates {
			if update.UpdateID >= offset {
				offset = update.UpdateID + 1
			}
			if update.Message == nil {
				continue
			}
			replies := b.HandleMessage(ctx, *update.Message)
			for _, reply := range replies {
				if err := b.sendMessage(ctx, update.Message.Chat.ID, reply); err != nil {
					continue
				}
			}
		}
	}
}

func (b *Bot) SendText(ctx context.Context, chatID int64, text string) error {
	for _, chunk := range splitTelegram(text) {
		if err := b.sendMessage(ctx, chatID, chunk); err != nil {
			return err
		}
	}
	return nil
}

func (b *Bot) HandleMessage(ctx context.Context, msg Message) []string {
	if !b.authorized(msg.From.ID) {
		return []string{"Unauthorized Telegram user."}
	}
	if msg.Chat.Type != "private" {
		return []string{"Telegram Agent Bridge only accepts private chat messages."}
	}
	if msg.Chat.ID != msg.From.ID || !b.authorized(msg.Chat.ID) {
		return []string{"Unauthorized Telegram chat."}
	}

	text := strings.TrimSpace(msg.Text)
	if text == "" {
		return []string{helpText()}
	}

	command, rest := splitCommand(text)
	switch command {
	case "/start", "/help":
		return []string{helpText()}
	case "/status":
		return b.status(ctx)
	case "/model":
		return b.modelCommand(msg.From.ID, rest)
	case "/ask":
		return b.invoke(ctx, msg.From.ID, rest)
	case "/agent":
		return b.agent(ctx, msg.From.ID, rest)
	default:
		return []string{"Unknown command.\n\n" + helpText()}
	}
}

func (b *Bot) authorized(userID int64) bool {
	return b.cfg.AllowedUserIDs[userID]
}

func (b *Bot) status(ctx context.Context) []string {
	out, err := b.runner.Run(ctx, "openclaw", "models", "status", "--json")
	if err != nil {
		return []string{"OpenClaw status failed:\n" + err.Error()}
	}
	return splitTelegram("OpenClaw models:\n" + strings.TrimSpace(string(out)))
}

func (b *Bot) modelCommand(userID int64, rest string) []string {
	if strings.TrimSpace(rest) == "" {
		return []string{fmt.Sprintf("Current model: %s\nAllowed models:\n- %s", b.modelForUser(userID), strings.Join(b.cfg.AllowedModels, "\n- "))}
	}
	model := strings.Fields(rest)[0]
	if !contains(b.cfg.AllowedModels, model) {
		return []string{fmt.Sprintf("Model %q is not allowed.\nAllowed models:\n- %s", model, strings.Join(b.cfg.AllowedModels, "\n- "))}
	}
	b.mu.Lock()
	b.userModel[userID] = model
	b.mu.Unlock()
	return []string{"Model set to " + model}
}

func (b *Bot) invoke(ctx context.Context, userID int64, prompt string) []string {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return []string{"Usage: /ask <prompt>"}
	}
	return b.runOpenClaw(ctx, b.modelForUser(userID), prompt)
}

func (b *Bot) agent(ctx context.Context, userID int64, rest string) []string {
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return []string{"Usage: /agent <role> <task>\nExample: /agent qa run the SSO smoke checks"}
	}
	confirmed := false
	if strings.HasPrefix(strings.ToLower(rest), "confirm ") {
		confirmed = true
		rest = strings.TrimSpace(rest[len("confirm "):])
	}
	if rest == "" {
		return []string{"Usage: /agent <role> <task>\nExample: /agent qa run the SSO smoke checks"}
	}
	if b.cfg.RequireConfirmation && !confirmed && risky(rest) {
		return []string{"Confirmation required for write/deploy actions. Re-run as:\n/agent confirm " + rest}
	}
	fields := strings.Fields(rest)
	role := fields[0]
	task := strings.TrimSpace(strings.TrimPrefix(rest, role))
	if task == "" {
		return []string{"Usage: /agent <role> <task>"}
	}
	prompt := fmt.Sprintf("You are the %s agent for the caboose-ai homelab. Be concise, operational, and explicit about commands and risks.\n\nTask: %s", role, task)
	return b.runOpenClaw(ctx, b.modelForUser(userID), prompt)
}

func (b *Bot) runOpenClaw(ctx context.Context, model, prompt string) []string {
	invokeCtx, cancel := context.WithTimeout(ctx, openClawInvokeTimeout)
	defer cancel()
	out, err := b.runner.Run(invokeCtx, "openclaw", "infer", "model", "run", "--gateway", "--json", "--model", model, "--prompt", prompt)
	if err != nil {
		return []string{"Agent invocation failed:\n" + err.Error()}
	}
	text := extractModelText(out)
	if strings.TrimSpace(text) == "" {
		text = strings.TrimSpace(string(out))
	}
	return splitTelegram(text)
}

func (b *Bot) modelForUser(userID int64) string {
	b.mu.Lock()
	defer b.mu.Unlock()
	if model := b.userModel[userID]; model != "" {
		return model
	}
	return b.cfg.DefaultModel
}

func (b *Bot) getUpdates(ctx context.Context, offset int64) ([]Update, error) {
	return b.getUpdatesWithTimeout(ctx, offset, defaultPollTimeout)
}

func (b *Bot) discardPendingUpdates(ctx context.Context) (int64, error) {
	var offset int64
	for {
		updates, err := b.getUpdatesWithTimeout(ctx, offset, 0)
		if err != nil {
			return 0, err
		}
		if len(updates) == 0 {
			return offset, nil
		}
		for _, update := range updates {
			if update.UpdateID >= offset {
				offset = update.UpdateID + 1
			}
		}
	}
}

func (b *Bot) getUpdatesWithTimeout(ctx context.Context, offset int64, timeout int) ([]Update, error) {
	values := url.Values{}
	values.Set("timeout", strconv.Itoa(timeout))
	if offset > 0 {
		values.Set("offset", strconv.FormatInt(offset, 10))
	}
	endpoint := fmt.Sprintf("%s/bot%s/getUpdates?%s", strings.TrimRight(b.cfg.TelegramAPIBase, "/"), b.cfg.BotToken, values.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := b.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("telegram getUpdates returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var payload struct {
		OK     bool     `json:"ok"`
		Result []Update `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	if !payload.OK {
		return nil, errors.New("telegram getUpdates returned ok=false")
	}
	return payload.Result, nil
}

func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (b *Bot) sendMessage(ctx context.Context, chatID int64, text string) error {
	values := url.Values{}
	values.Set("chat_id", strconv.FormatInt(chatID, 10))
	values.Set("text", text)
	endpoint := fmt.Sprintf("%s/bot%s/sendMessage", strings.TrimRight(b.cfg.TelegramAPIBase, "/"), b.cfg.BotToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := b.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("telegram sendMessage returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func helpText() string {
	return strings.Join([]string{
		"Telegram Agent Bridge",
		"",
		"/ask <prompt> - ask the current model",
		"/agent <role> <task> - run a role-scoped agent prompt",
		"/model - show current and allowed models",
		"/model <provider/model> - switch model",
		"/status - show OpenClaw model status",
	}, "\n")
}

func splitCommand(text string) (string, string) {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return "", ""
	}
	command := fields[0]
	if at := strings.Index(command, "@"); at >= 0 {
		command = command[:at]
	}
	return strings.ToLower(command), strings.TrimSpace(strings.TrimPrefix(text, fields[0]))
}

func splitCSV(raw string) []string {
	var values []string
	for _, value := range strings.Split(raw, ",") {
		value = strings.TrimSpace(value)
		if value != "" {
			values = append(values, value)
		}
	}
	return values
}

func envBool(key string, fallback bool) (bool, error) {
	raw := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if raw == "" {
		return fallback, nil
	}
	switch raw {
	case "1", "true", "yes", "on":
		return true, nil
	case "0", "false", "no", "off":
		return false, nil
	default:
		return false, fmt.Errorf("%s must be a boolean value", key)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func risky(text string) bool {
	lower := strings.ToLower(text)
	for _, word := range []string{"deploy", "restart", "delete", "remove", "write", "edit", "update", "create", "replace", "commit", "push", "merge", "apply", "shutdown", "reset", "rollback", "roll back", "rollout"} {
		if strings.Contains(lower, word) {
			return true
		}
	}
	return false
}

func extractModelText(out []byte) string {
	var payload map[string]any
	if err := json.NewDecoder(bytes.NewReader(out)).Decode(&payload); err != nil {
		return strings.TrimSpace(string(out))
	}
	for _, key := range []string{"text", "response", "output", "content", "message"} {
		if value, ok := payload[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	if choices, ok := payload["choices"].([]any); ok && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]any); ok {
			if value, ok := choice["text"].(string); ok {
				return strings.TrimSpace(value)
			}
			if msg, ok := choice["message"].(map[string]any); ok {
				if value, ok := msg["content"].(string); ok {
					return strings.TrimSpace(value)
				}
			}
		}
	}
	return strings.TrimSpace(string(out))
}

func splitTelegram(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return []string{""}
	}
	runes := []rune(text)
	var chunks []string
	for len(runes) > maxTelegramMessage {
		cut := -1
		prefix := runes[:maxTelegramMessage]
		for i := len(prefix) - 1; i >= 0; i-- {
			if prefix[i] == '\n' {
				cut = i
				break
			}
		}
		if cut < maxTelegramMessage/2 {
			cut = maxTelegramMessage
		}
		chunks = append(chunks, strings.TrimSpace(string(runes[:cut])))
		runes = []rune(strings.TrimSpace(string(runes[cut:])))
	}
	chunks = append(chunks, strings.TrimSpace(string(runes)))
	return chunks
}
