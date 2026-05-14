package main

import (
	"context"
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
		"number,title,url,state,isDraft,mergeStateStatus,reviewDecision,statusCheckRollup,comments,reviews,latestReviews,reviewRequests,headRefName,headRefOid,baseRefName,headRepository,headRepositoryOwner",
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
	mock.On("gh api repos/caboose-ai/ai-skills/pulls/6/comments", []byte(`[{"user":{"login":"chatgpt-codex-connector"},"body":"Fix it","commit_id":"abc123"}]`), nil)

	pr, err := fetchPR(context.Background(), mock, options{Repo: "caboose-ai/ai-skills", PRNumber: 6})

	if err != nil {
		t.Fatalf("fetchPR() error = %v", err)
	}
	if len(pr.ReviewComments) != 1 {
		t.Fatalf("ReviewComments = %v, want one Codex review comment", pr.ReviewComments)
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
