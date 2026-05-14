package main

import (
	"context"
	"errors"
	"io"
	"reflect"
	"testing"
	"time"

	"github.com/caboose-ai/caboose-ai.io/internal/runner"
)

func TestPRViewArgsUsesExplicitRepoAndPR(t *testing.T) {
	got := prViewArgs(options{Repo: "caboose-ai/ai-skills", PRNumber: 6})

	want := []string{
		"pr",
		"view",
		"6",
		"--json",
		"number,title,url,state,isDraft,mergeStateStatus,reviewDecision,statusCheckRollup,comments,reviews,latestReviews,reviewRequests,headRefName,headRefOid,baseRefName,headRepository,headRepositoryOwner,commits",
		"--repo",
		"caboose-ai/ai-skills",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("prViewArgs() = %#v, want %#v", got, want)
	}
}

func TestPRViewArgsCanUseCurrentBranchPR(t *testing.T) {
	got := prViewArgs(options{})

	if got[0] != "pr" || got[1] != "view" {
		t.Fatalf("prViewArgs() = %#v, want gh pr view", got)
	}
	for _, arg := range got {
		if arg == "0" {
			t.Fatalf("prViewArgs() = %#v, should not include zero PR number", got)
		}
	}
}

func TestTelegramConfigReadsBotTokenFromOnePassword(t *testing.T) {
	t.Setenv("TELEGRAM_BOT_TOKEN", "")
	t.Setenv("TELEGRAM_ALLOWED_USER_IDS", "123")
	t.Setenv("TELEGRAM_NOTIFY_CHAT_ID", "123")
	mock := runner.NewMockRunner()
	mock.On("op item get Telegram Homelab Bot Token --vault Personal --fields password --reveal", []byte("secret-token\n"), nil)

	cfg, err := telegramConfig(context.Background(), mock, options{
		TokenOPItem:  "Telegram Homelab Bot Token",
		TokenOPVault: "Personal",
	})

	if err != nil {
		t.Fatalf("telegramConfig() error = %v", err)
	}
	if cfg.BotToken != "secret-token" {
		t.Fatalf("BotToken = %q, want secret-token", cfg.BotToken)
	}
}

func TestFetchPRLoadsReviewComments(t *testing.T) {
	mock := runner.NewMockRunner()
	mock.On("gh pr view 6 --json", []byte(`{"number":6,"headRefOid":"abc123"}`), nil)
	mock.On("gh api --paginate repos/caboose-ai/ai-skills/pulls/6/comments --jq .[]", []byte(`{"user":{"login":"chatgpt-codex-connector"},"body":"Fix it","commit_id":"abc123"}
{"user":{"login":"reviewer"},"body":"Other","commit_id":"abc123"}`), nil)

	pr, err := fetchPR(context.Background(), mock, options{Repo: "caboose-ai/ai-skills", PRNumber: 6})

	if err != nil {
		t.Fatalf("fetchPR() error = %v", err)
	}
	if len(pr.ReviewComments) != 2 {
		t.Fatalf("ReviewComments = %v, want two review comments", pr.ReviewComments)
	}
}

func TestFetchReviewCommentsAcceptsEmptyPaginatedOutput(t *testing.T) {
	mock := runner.NewMockRunner()
	mock.On("gh api --paginate repos/caboose-ai/ai-skills/pulls/6/comments --jq .[]", []byte("\n"), nil)

	comments, err := fetchReviewComments(context.Background(), mock, "caboose-ai/ai-skills", 6)

	if err != nil {
		t.Fatalf("fetchReviewComments() error = %v", err)
	}
	if len(comments) != 0 {
		t.Fatalf("comments = %#v, want empty", comments)
	}
}

func TestWatchReportsTimeoutWhenCheckCommandExceedsDeadline(t *testing.T) {
	err := watch(context.Background(), contextDeadlineRunner{}, options{
		Repo:     "caboose-ai/ai-skills",
		PRNumber: 6,
		Poll:     time.Minute,
		Timeout:  time.Millisecond,
	}, io.Discard, io.Discard)

	if !errors.Is(err, errTimedOut) {
		t.Fatalf("watch() error = %v, want errTimedOut", err)
	}
}

func TestParseOptionsDefaultsToTenMinuteWatcher(t *testing.T) {
	opts, err := parseOptions(nil)
	if err != nil {
		t.Fatalf("parseOptions() error = %v", err)
	}
	if opts.Poll != time.Minute {
		t.Fatalf("Poll = %s, want 1m", opts.Poll)
	}
	if opts.Timeout != 10*time.Minute {
		t.Fatalf("Timeout = %s, want 10m", opts.Timeout)
	}
}

type contextDeadlineRunner struct{}

func (contextDeadlineRunner) Run(ctx context.Context, _ string, _ ...string) ([]byte, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func (r contextDeadlineRunner) RunWithStdin(ctx context.Context, _ io.Reader, name string, args ...string) ([]byte, error) {
	return r.Run(ctx, name, args...)
}
