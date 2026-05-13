package prwatch

import (
	"strings"
	"testing"
)

func TestAssessReadyAfterCodexReviewWithNoChecks(t *testing.T) {
	pr := PullRequest{
		Number:           6,
		Title:            "feat: add session and PR workflow skills",
		URL:              "https://github.com/caboose-ai/ai-skills/pull/6",
		State:            "OPEN",
		IsDraft:          true,
		MergeStateStatus: "CLEAN",
		Comments: []Comment{
			{Author: User{Login: "cxm6467"}, Body: "@codex review"},
			{Author: User{Login: "chatgpt-codex-connector"}, Body: "Codex Review: Didn't find any major issues."},
		},
		StatusCheckRollup: []map[string]any{},
	}

	got := Assess(pr)

	if got.State != Ready {
		t.Fatalf("Assess().State = %q, want %q; findings=%v", got.State, Ready, got.Findings)
	}
	if !strings.Contains(got.Summary, "final human review") {
		t.Fatalf("summary = %q, want final human review handoff", got.Summary)
	}
	if !containsFinding(got.Findings, "draft") {
		t.Fatalf("findings = %v, want draft status noted", got.Findings)
	}
}

func TestAssessWaitsForCodexReview(t *testing.T) {
	pr := PullRequest{
		Number:           7,
		Title:            "feat: test",
		URL:              "https://github.com/caboose-ai/caboose-ai.io/pull/7",
		State:            "OPEN",
		MergeStateStatus: "CLEAN",
		Comments: []Comment{
			{Author: User{Login: "cxm6467"}, Body: "@codex review"},
		},
	}

	got := Assess(pr)

	if got.State != Waiting {
		t.Fatalf("Assess().State = %q, want %q; findings=%v", got.State, Waiting, got.Findings)
	}
	if !containsFinding(got.Findings, "Codex review has not completed") {
		t.Fatalf("findings = %v, want missing Codex review finding", got.Findings)
	}
}

func TestAssessBlocksFailingChecks(t *testing.T) {
	pr := readyPR()
	pr.StatusCheckRollup = []map[string]any{
		{"__typename": "CheckRun", "name": "test", "status": "COMPLETED", "conclusion": "FAILURE"},
	}

	got := Assess(pr)

	if got.State != Blocked {
		t.Fatalf("Assess().State = %q, want %q; findings=%v", got.State, Blocked, got.Findings)
	}
	if !containsFinding(got.Findings, "test failed") {
		t.Fatalf("findings = %v, want failing check finding", got.Findings)
	}
}

func TestAssessWaitsForPendingChecks(t *testing.T) {
	pr := readyPR()
	pr.StatusCheckRollup = []map[string]any{
		{"__typename": "CheckRun", "name": "build", "status": "IN_PROGRESS", "conclusion": ""},
	}

	got := Assess(pr)

	if got.State != Waiting {
		t.Fatalf("Assess().State = %q, want %q; findings=%v", got.State, Waiting, got.Findings)
	}
	if !containsFinding(got.Findings, "build is still IN_PROGRESS") {
		t.Fatalf("findings = %v, want pending check finding", got.Findings)
	}
}

func TestAssessBlocksChangesRequested(t *testing.T) {
	pr := readyPR()
	pr.LatestReviews = []Review{
		{Author: User{Login: "reviewer"}, State: "CHANGES_REQUESTED"},
	}

	got := Assess(pr)

	if got.State != Blocked {
		t.Fatalf("Assess().State = %q, want %q; findings=%v", got.State, Blocked, got.Findings)
	}
	if !containsFinding(got.Findings, "changes requested") {
		t.Fatalf("findings = %v, want changes requested finding", got.Findings)
	}
}

func readyPR() PullRequest {
	return PullRequest{
		Number:           8,
		Title:            "feat: ready",
		URL:              "https://github.com/caboose-ai/caboose-ai.io/pull/8",
		State:            "OPEN",
		MergeStateStatus: "CLEAN",
		Comments: []Comment{
			{Author: User{Login: "chatgpt-codex-connector"}, Body: "Codex Review: Didn't find any major issues."},
		},
	}
}

func containsFinding(findings []string, want string) bool {
	for _, finding := range findings {
		if strings.Contains(finding, want) {
			return true
		}
	}
	return false
}
