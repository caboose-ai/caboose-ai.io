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

func TestAssessIgnoresSupersededChangesRequestedReviews(t *testing.T) {
	pr := readyPR()
	pr.Reviews = []Review{
		{Author: User{Login: "reviewer"}, State: "CHANGES_REQUESTED"},
	}
	pr.LatestReviews = []Review{
		{Author: User{Login: "reviewer"}, State: "APPROVED"},
	}

	got := Assess(pr)

	if got.State != Ready {
		t.Fatalf("Assess().State = %q, want %q; findings=%v", got.State, Ready, got.Findings)
	}
}

func TestAssessRequiresCodexCompletionAfterLatestRequest(t *testing.T) {
	pr := readyPR()
	pr.Comments = []Comment{
		{Author: User{Login: "chatgpt-codex-connector"}, Body: "Codex Review: done", CreatedAt: "2026-05-13T03:00:00Z"},
		{Author: User{Login: "cxm6467"}, Body: "@codex review", CreatedAt: "2026-05-13T03:10:00Z"},
	}
	pr.Reviews = nil

	got := Assess(pr)

	if got.State != Waiting {
		t.Fatalf("Assess().State = %q, want %q; findings=%v", got.State, Waiting, got.Findings)
	}
	if !containsFinding(got.Findings, "Codex review has not completed") {
		t.Fatalf("findings = %v, want missing Codex review finding", got.Findings)
	}
}

func TestAssessRequiresCommentCodexCompletionAfterCurrentHeadSignal(t *testing.T) {
	pr := readyPR()
	pr.HeadRefOID = "new-head"
	pr.StatusCheckRollup = []map[string]any{
		{"__typename": "CheckRun", "name": "test", "status": "COMPLETED", "conclusion": "SUCCESS", "startedAt": "2026-05-13T03:20:00Z"},
	}
	pr.Comments = []Comment{
		{Author: User{Login: "chatgpt-codex-connector"}, Body: "Codex Review: done", CreatedAt: "2026-05-13T03:10:00Z"},
	}
	pr.Reviews = nil

	got := Assess(pr)

	if got.State != Waiting {
		t.Fatalf("Assess().State = %q, want %q; findings=%v", got.State, Waiting, got.Findings)
	}
}

func TestAssessAcceptsCommentCodexCompletionAfterCurrentHeadSignal(t *testing.T) {
	pr := readyPR()
	pr.HeadRefOID = "new-head"
	pr.StatusCheckRollup = []map[string]any{
		{"__typename": "CheckRun", "name": "test", "status": "COMPLETED", "conclusion": "SUCCESS", "startedAt": "2026-05-13T03:20:00Z"},
	}
	pr.Comments = []Comment{
		{Author: User{Login: "chatgpt-codex-connector"}, Body: "Codex Review: done", CreatedAt: "2026-05-13T03:21:00Z"},
	}
	pr.Reviews = nil

	got := Assess(pr)

	if got.State != Ready {
		t.Fatalf("Assess().State = %q, want %q; findings=%v", got.State, Ready, got.Findings)
	}
}

func TestAssessDoesNotUseTruncatedCommitListAsCurrentHeadSignal(t *testing.T) {
	pr := readyPR()
	pr.HeadRefOID = "new-head"
	pr.Commits = []Commit{
		{OID: "older-head", CommittedDate: "2026-05-13T04:20:00Z", AuthoredDate: "2026-05-13T04:20:00Z"},
	}
	pr.StatusCheckRollup = []map[string]any{
		{"__typename": "CheckRun", "name": "test", "status": "COMPLETED", "conclusion": "SUCCESS", "startedAt": "2026-05-13T05:00:00Z"},
	}
	pr.Comments = []Comment{
		{Author: User{Login: "chatgpt-codex-connector"}, Body: "Codex Review: done", CreatedAt: "2026-05-13T04:30:00Z"},
	}
	pr.Reviews = nil

	got := Assess(pr)

	if got.State != Waiting {
		t.Fatalf("Assess().State = %q, want %q; findings=%v", got.State, Waiting, got.Findings)
	}
}

func TestAssessRequiresCodexReviewForCurrentHead(t *testing.T) {
	pr := readyPR()
	pr.HeadRefOID = "new-head"
	pr.Comments = nil
	pr.LatestReviews = []Review{
		{Author: User{Login: "chatgpt-codex-connector"}, State: "COMMENTED", SubmittedAt: "2026-05-13T03:10:00Z"},
	}
	pr.Reviews = []Review{
		{Author: User{Login: "chatgpt-codex-connector"}, State: "COMMENTED", SubmittedAt: "2026-05-13T03:10:00Z", Commit: Commit{OID: "old-head"}},
	}

	got := Assess(pr)

	if got.State != Waiting {
		t.Fatalf("Assess().State = %q, want %q; findings=%v", got.State, Waiting, got.Findings)
	}
}

func TestAssessAcceptsReviewCodexCompletionForCurrentHead(t *testing.T) {
	pr := readyPR()
	pr.HeadRefOID = "new-head"
	pr.Comments = nil
	pr.LatestReviews = []Review{
		{Author: User{Login: "chatgpt-codex-connector"}, State: "COMMENTED", SubmittedAt: "2026-05-13T03:10:00Z", Commit: Commit{OID: "new-head"}},
	}
	pr.Reviews = nil

	got := Assess(pr)

	if got.State != Ready {
		t.Fatalf("Assess().State = %q, want %q; findings=%v", got.State, Ready, got.Findings)
	}
}

func TestAssessBlocksStartupFailureAndStaleChecks(t *testing.T) {
	pr := readyPR()
	pr.StatusCheckRollup = []map[string]any{
		{"__typename": "CheckRun", "name": "test", "status": "COMPLETED", "conclusion": "STARTUP_FAILURE"},
		{"__typename": "CheckRun", "name": "docs", "status": "COMPLETED", "conclusion": "STALE"},
	}

	got := Assess(pr)

	if got.State != Blocked {
		t.Fatalf("Assess().State = %q, want %q; findings=%v", got.State, Blocked, got.Findings)
	}
	if !containsFinding(got.Findings, "test failed") || !containsFinding(got.Findings, "docs failed") {
		t.Fatalf("findings = %v, want failed startup/stale findings", got.Findings)
	}
}

func TestAssessAllowsHumanReviewRequiredMergeBlock(t *testing.T) {
	pr := readyPR()
	pr.MergeStateStatus = "BLOCKED"
	pr.ReviewDecision = "REVIEW_REQUIRED"

	got := Assess(pr)

	if got.State != Ready {
		t.Fatalf("Assess().State = %q, want %q; findings=%v", got.State, Ready, got.Findings)
	}
	if !containsFinding(got.Findings, "human review required") {
		t.Fatalf("findings = %v, want human review required note", got.Findings)
	}
}

func TestAssessAllowsDraftMergeState(t *testing.T) {
	pr := readyPR()
	pr.IsDraft = true
	pr.MergeStateStatus = "DRAFT"

	got := Assess(pr)

	if got.State != Ready {
		t.Fatalf("Assess().State = %q, want %q; findings=%v", got.State, Ready, got.Findings)
	}
	if !containsFinding(got.Findings, "draft PR") {
		t.Fatalf("findings = %v, want draft note", got.Findings)
	}
}

func TestAssessBlocksUnstableMergeStateAfterChecksFinish(t *testing.T) {
	pr := readyPR()
	pr.MergeStateStatus = "UNSTABLE"

	got := Assess(pr)

	if got.State != Blocked {
		t.Fatalf("Assess().State = %q, want %q; findings=%v", got.State, Blocked, got.Findings)
	}
	if !containsFinding(got.Findings, "merge state is UNSTABLE") {
		t.Fatalf("findings = %v, want unstable merge state finding", got.Findings)
	}
}

func TestAssessBlocksCurrentHeadCodexReviewComments(t *testing.T) {
	pr := readyPR()
	pr.HeadRefOID = "current-head"
	pr.ReviewComments = []ReviewComment{
		{Author: User{Login: "chatgpt-codex-connector"}, Body: "Fix this.", CommitID: "current-head"},
	}

	got := Assess(pr)

	if got.State != Blocked {
		t.Fatalf("Assess().State = %q, want %q; findings=%v", got.State, Blocked, got.Findings)
	}
	if !containsFinding(got.Findings, "Codex review has 1 current-head comment") {
		t.Fatalf("findings = %v, want current-head Codex comment finding", got.Findings)
	}
}

func TestAssessIgnoresOutdatedCodexReviewComments(t *testing.T) {
	pr := readyPR()
	pr.HeadRefOID = "current-head"
	pr.StatusCheckRollup = []map[string]any{
		{"__typename": "CheckRun", "name": "test", "status": "COMPLETED", "conclusion": "SUCCESS", "startedAt": "2026-05-13T03:00:00Z"},
	}
	pr.Comments = []Comment{
		{Author: User{Login: "chatgpt-codex-connector"}, Body: "Codex Review: done", CreatedAt: "2026-05-13T03:01:00Z"},
	}
	pr.ReviewComments = []ReviewComment{
		{Author: User{Login: "chatgpt-codex-connector"}, Body: "Fix this.", CommitID: "old-head"},
	}

	got := Assess(pr)

	if got.State != Ready {
		t.Fatalf("Assess().State = %q, want %q; findings=%v", got.State, Ready, got.Findings)
	}
}

func TestAssessIgnoresRetargetedOutdatedCodexReviewComments(t *testing.T) {
	pr := readyPR()
	pr.HeadRefOID = "current-head"
	pr.StatusCheckRollup = []map[string]any{
		{"__typename": "CheckRun", "name": "test", "status": "COMPLETED", "conclusion": "SUCCESS", "startedAt": "2026-05-13T03:10:30Z"},
	}
	pr.Comments = []Comment{
		{Author: User{Login: "cxm6467"}, Body: "@codex review", CreatedAt: "2026-05-13T03:10:00Z"},
		{Author: User{Login: "chatgpt-codex-connector"}, Body: "Codex Review: done", CreatedAt: "2026-05-13T03:11:00Z"},
	}
	pr.ReviewComments = []ReviewComment{
		{
			Author:           User{Login: "chatgpt-codex-connector"},
			Body:             "Fix this.",
			CommitID:         "current-head",
			OriginalCommitID: "old-head",
			CreatedAt:        "2026-05-13T03:00:00Z",
		},
	}

	got := Assess(pr)

	if got.State != Ready {
		t.Fatalf("Assess().State = %q, want %q; findings=%v", got.State, Ready, got.Findings)
	}
}

func TestAssessIgnoresRetargetedCodexReviewCommentsBeforeCurrentHeadSignal(t *testing.T) {
	pr := readyPR()
	pr.HeadRefOID = "current-head"
	pr.StatusCheckRollup = []map[string]any{
		{"__typename": "CheckRun", "name": "test", "status": "COMPLETED", "conclusion": "SUCCESS", "startedAt": "2026-05-13T03:20:00Z"},
	}
	pr.Comments = []Comment{
		{Author: User{Login: "cxm6467"}, Body: "@codex review", CreatedAt: "2026-05-13T03:10:00Z"},
		{Author: User{Login: "chatgpt-codex-connector"}, Body: "Codex Review: done", CreatedAt: "2026-05-13T03:21:00Z"},
	}
	pr.ReviewComments = []ReviewComment{
		{
			Author:           User{Login: "chatgpt-codex-connector"},
			Body:             "Fix this.",
			CommitID:         "current-head",
			OriginalCommitID: "old-head",
			CreatedAt:        "2026-05-13T03:15:00Z",
		},
	}

	got := Assess(pr)

	if got.State != Ready {
		t.Fatalf("Assess().State = %q, want %q; findings=%v", got.State, Ready, got.Findings)
	}
}

func TestAssessWaitsOnUnstableMergeStateWhenChecksArePending(t *testing.T) {
	pr := readyPR()
	pr.MergeStateStatus = "UNSTABLE"
	pr.StatusCheckRollup = []map[string]any{
		{"__typename": "CheckRun", "name": "test", "status": "IN_PROGRESS", "conclusion": ""},
	}

	got := Assess(pr)

	if got.State != Waiting {
		t.Fatalf("Assess().State = %q, want %q; findings=%v", got.State, Waiting, got.Findings)
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
