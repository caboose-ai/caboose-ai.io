package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/caboose-ai/caboose-ai.io/internal/prwatch"
	"github.com/caboose-ai/caboose-ai.io/internal/runner"
	"github.com/caboose-ai/caboose-ai.io/internal/telegrambot"
)

const githubFields = "number,title,url,state,isDraft,mergeStateStatus,reviewDecision,statusCheckRollup,comments,reviews,latestReviews,reviewRequests,headRefName,headRefOid,baseRefName,headRepository,headRepositoryOwner,commits"

var (
	errWaiting  = errors.New("PR is still waiting")
	errBlocked  = errors.New("PR is blocked")
	errTimedOut = errors.New("timed out waiting for PR readiness")
)

type options struct {
	Repo           string
	PRNumber       int
	Poll           time.Duration
	Timeout        time.Duration
	Once           bool
	DryRun         bool
	TokenOPItem    string
	TokenOPVault   string
	AllowedUserIDs string
	NotifyChatID   string
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr, runner.NewLocalRunner()); err != nil {
		fmt.Fprintf(os.Stderr, "pr-ready-watch: %v\n", err)
		os.Exit(exitCode(err))
	}
}

func run(args []string, stdout, stderr io.Writer, commandRunner runner.CommandRunner) error {
	opts, err := parseOptions(args)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return watch(ctx, commandRunner, opts, stdout, stderr)
}

func parseOptions(args []string) (options, error) {
	opts := options{
		Repo:         strings.TrimSpace(os.Getenv("GITHUB_REPOSITORY")),
		Poll:         time.Minute,
		Timeout:      10 * time.Minute,
		TokenOPItem:  strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN_OP_ITEM")),
		TokenOPVault: strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN_OP_VAULT")),
	}
	if opts.TokenOPVault == "" {
		opts.TokenOPVault = "Personal"
	}

	fs := flag.NewFlagSet("pr-ready-watch", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&opts.Repo, "repo", opts.Repo, "GitHub repository in owner/name form. Defaults to GITHUB_REPOSITORY or the current checkout.")
	fs.IntVar(&opts.PRNumber, "pr", 0, "Pull request number. Defaults to the current branch PR resolved by gh.")
	fs.DurationVar(&opts.Poll, "poll", opts.Poll, "Polling interval.")
	fs.DurationVar(&opts.Timeout, "timeout", opts.Timeout, "Maximum watch duration. Use 0 to watch until ready or blocked.")
	fs.BoolVar(&opts.Once, "once", false, "Check once and exit.")
	fs.BoolVar(&opts.DryRun, "dry-run", false, "Print the readiness message instead of sending Telegram.")
	fs.StringVar(&opts.TokenOPItem, "token-op-item", opts.TokenOPItem, "1Password item containing the Telegram bot token when TELEGRAM_BOT_TOKEN is unset.")
	fs.StringVar(&opts.TokenOPVault, "token-op-vault", opts.TokenOPVault, "1Password vault for --token-op-item.")
	fs.StringVar(&opts.AllowedUserIDs, "telegram-allowed-user-ids", "", "Override TELEGRAM_ALLOWED_USER_IDS for this run.")
	fs.StringVar(&opts.NotifyChatID, "telegram-notify-chat-id", "", "Override TELEGRAM_NOTIFY_CHAT_ID for this run.")
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	if opts.Poll <= 0 {
		return options{}, errors.New("--poll must be greater than zero")
	}
	if opts.Timeout < 0 {
		return options{}, errors.New("--timeout must be zero or greater")
	}
	return opts, nil
}

func watch(ctx context.Context, commandRunner runner.CommandRunner, opts options, stdout, stderr io.Writer) error {
	start := time.Now()
	var deadline time.Time
	if opts.Timeout > 0 {
		deadline = start.Add(opts.Timeout)
	}

	for {
		checkCtx := ctx
		cancel := func() {}
		if !deadline.IsZero() {
			remaining := time.Until(deadline)
			if remaining <= 0 {
				return fmt.Errorf("%w after %s", errTimedOut, opts.Timeout)
			}
			checkCtx, cancel = context.WithTimeout(ctx, remaining)
		}
		pr, assessment, err := checkOnce(checkCtx, commandRunner, opts)
		checkErr := checkCtx.Err()
		cancel()
		if err != nil {
			if errors.Is(checkErr, context.DeadlineExceeded) || (!deadline.IsZero() && !time.Now().Before(deadline)) {
				return fmt.Errorf("%w after %s", errTimedOut, opts.Timeout)
			}
			return err
		}
		fmt.Fprintf(stderr, "pr-ready-watch: PR #%d %s (%s)\n", pr.Number, assessment.State, assessment.Summary)

		switch assessment.State {
		case prwatch.Ready:
			return deliver(ctx, commandRunner, opts, stdout, prwatch.Message(pr, assessment))
		case prwatch.Blocked:
			if opts.DryRun || strings.EqualFold(pr.State, "OPEN") {
				if err := deliver(ctx, commandRunner, opts, stdout, prwatch.Message(pr, assessment)); err != nil {
					return err
				}
			}
			return fmt.Errorf("%w: %s", errBlocked, assessment.Summary)
		}

		if opts.Once {
			if opts.DryRun {
				fmt.Fprintln(stdout, prwatch.Message(pr, assessment))
			}
			return fmt.Errorf("%w: %s", errWaiting, assessment.Summary)
		}
		if !deadline.IsZero() && !time.Now().Before(deadline) {
			return fmt.Errorf("%w after %s", errTimedOut, opts.Timeout)
		}
		sleepFor := opts.Poll
		if !deadline.IsZero() {
			remaining := time.Until(deadline)
			if remaining <= 0 {
				return fmt.Errorf("%w after %s", errTimedOut, opts.Timeout)
			}
			if remaining < sleepFor {
				sleepFor = remaining
			}
		}
		if err := sleepContext(ctx, sleepFor); err != nil {
			return err
		}
	}
}

func checkOnce(ctx context.Context, commandRunner runner.CommandRunner, opts options) (prwatch.PullRequest, prwatch.Assessment, error) {
	pr, err := fetchPR(ctx, commandRunner, opts)
	if err != nil {
		return prwatch.PullRequest{}, prwatch.Assessment{}, err
	}
	return pr, prwatch.Assess(pr), nil
}

func fetchPR(ctx context.Context, commandRunner runner.CommandRunner, opts options) (prwatch.PullRequest, error) {
	out, err := commandRunner.Run(ctx, "gh", prViewArgs(opts)...)
	if err != nil {
		return prwatch.PullRequest{}, err
	}
	var pr prwatch.PullRequest
	if err := json.Unmarshal(out, &pr); err != nil {
		return prwatch.PullRequest{}, fmt.Errorf("parse gh pr view JSON: %w", err)
	}
	repo, err := repoName(ctx, commandRunner, opts)
	if err != nil {
		return prwatch.PullRequest{}, err
	}
	reviewComments, err := fetchReviewComments(ctx, commandRunner, repo, pr.Number)
	if err != nil {
		return prwatch.PullRequest{}, err
	}
	pr.ReviewComments = reviewComments
	return pr, nil
}

func repoName(ctx context.Context, commandRunner runner.CommandRunner, opts options) (string, error) {
	if strings.TrimSpace(opts.Repo) != "" {
		return strings.TrimSpace(opts.Repo), nil
	}
	out, err := commandRunner.Run(ctx, "gh", "repo", "view", "--json", "nameWithOwner", "--jq", ".nameWithOwner")
	if err != nil {
		return "", err
	}
	repo := strings.TrimSpace(string(out))
	if repo == "" {
		return "", errors.New("gh repo view returned an empty repository name")
	}
	return repo, nil
}

func fetchReviewComments(ctx context.Context, commandRunner runner.CommandRunner, repo string, prNumber int) ([]prwatch.ReviewComment, error) {
	if strings.TrimSpace(repo) == "" || prNumber <= 0 {
		return nil, nil
	}
	out, err := commandRunner.Run(ctx, "gh", "api", "--paginate", fmt.Sprintf("repos/%s/pulls/%d/comments", repo, prNumber), "--jq", ".[]")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(string(out)) == "" {
		return nil, nil
	}
	var comments []prwatch.ReviewComment
	decoder := json.NewDecoder(bytes.NewReader(out))
	for {
		var comment prwatch.ReviewComment
		if err := decoder.Decode(&comment); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("parse pull review comments JSON: %w", err)
		}
		comments = append(comments, comment)
	}
	return comments, nil
}

func prViewArgs(opts options) []string {
	args := []string{"pr", "view"}
	if opts.PRNumber > 0 {
		args = append(args, strconv.Itoa(opts.PRNumber))
	}
	args = append(args, "--json", githubFields)
	if strings.TrimSpace(opts.Repo) != "" {
		args = append(args, "--repo", strings.TrimSpace(opts.Repo))
	}
	return args
}

func deliver(ctx context.Context, commandRunner runner.CommandRunner, opts options, stdout io.Writer, message string) error {
	if opts.DryRun {
		fmt.Fprintln(stdout, message)
		return nil
	}
	cfg, err := telegramConfig(ctx, commandRunner, opts)
	if err != nil {
		return err
	}
	bot, err := telegrambot.New(cfg, commandRunner, nil)
	if err != nil {
		return err
	}
	chatIDs, err := notificationChatIDs(bot.AllowedChatIDs(), strings.TrimSpace(os.Getenv("TELEGRAM_NOTIFY_CHAT_ID")))
	if err != nil {
		return err
	}
	var failures []string
	for _, chatID := range chatIDs {
		if err := bot.SendText(ctx, chatID, message); err != nil {
			failures = append(failures, fmt.Sprintf("%d: %v", chatID, err))
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("sending Telegram message failed for %d chat(s): %s", len(failures), strings.Join(failures, "; "))
	}
	return nil
}

func telegramConfig(ctx context.Context, commandRunner runner.CommandRunner, opts options) (telegrambot.Config, error) {
	if strings.TrimSpace(opts.AllowedUserIDs) != "" {
		if err := os.Setenv("TELEGRAM_ALLOWED_USER_IDS", strings.TrimSpace(opts.AllowedUserIDs)); err != nil {
			return telegrambot.Config{}, err
		}
	}
	if strings.TrimSpace(opts.NotifyChatID) != "" {
		if err := os.Setenv("TELEGRAM_NOTIFY_CHAT_ID", strings.TrimSpace(opts.NotifyChatID)); err != nil {
			return telegrambot.Config{}, err
		}
	}
	if strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN")) == "" && strings.TrimSpace(opts.TokenOPItem) != "" {
		args := []string{"item", "get", strings.TrimSpace(opts.TokenOPItem)}
		if strings.TrimSpace(opts.TokenOPVault) != "" {
			args = append(args, "--vault", strings.TrimSpace(opts.TokenOPVault))
		}
		args = append(args, "--fields", "password", "--reveal")
		out, err := commandRunner.Run(ctx, "op", args...)
		if err != nil {
			return telegrambot.Config{}, fmt.Errorf("read Telegram token from 1Password: %w", err)
		}
		token := strings.TrimSpace(string(out))
		if token == "" {
			return telegrambot.Config{}, errors.New("1Password Telegram token field was empty")
		}
		if err := os.Setenv("TELEGRAM_BOT_TOKEN", token); err != nil {
			return telegrambot.Config{}, err
		}
	}
	return telegrambot.ConfigFromEnv()
}

func notificationChatIDs(allowed []int64, raw string) ([]int64, error) {
	if strings.TrimSpace(raw) == "" {
		return allowed, nil
	}
	allowlist := map[int64]bool{}
	for _, id := range allowed {
		allowlist[id] = true
	}
	var ids []int64
	for _, field := range strings.Split(raw, ",") {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		id, err := strconv.ParseInt(field, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid TELEGRAM_NOTIFY_CHAT_ID value %q", field)
		}
		if !allowlist[id] {
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

func sleepContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func exitCode(err error) int {
	switch {
	case errors.Is(err, errBlocked):
		return 2
	case errors.Is(err, errWaiting), errors.Is(err, errTimedOut):
		return 1
	default:
		return 1
	}
}
