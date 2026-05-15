package prwatch

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type State string

const (
	Ready   State = "ready"
	Waiting State = "waiting"
	Blocked State = "blocked"
)

type PullRequest struct {
	Number                 int              `json:"number"`
	Title                  string           `json:"title"`
	URL                    string           `json:"url"`
	State                  string           `json:"state"`
	IsDraft                bool             `json:"isDraft"`
	MergeStateStatus       string           `json:"mergeStateStatus"`
	ReviewDecision         string           `json:"reviewDecision"`
	Comments               []Comment        `json:"comments"`
	Reviews                []Review         `json:"reviews"`
	LatestReviews          []Review         `json:"latestReviews"`
	ReviewComments         []ReviewComment  `json:"reviewComments"`
	Commits                []Commit         `json:"commits"`
	StatusCheckRollup      []map[string]any `json:"statusCheckRollup"`
	ReviewRequests         []ReviewRequest  `json:"reviewRequests"`
	HeadRefName            string           `json:"headRefName"`
	HeadRefOID             string           `json:"headRefOid"`
	LatestRequestAfterHead bool             `json:"-"`
	BaseRefName            string           `json:"baseRefName"`
	HeadRepository         Repository       `json:"headRepository"`
	HeadRepositoryOwner    RepositoryOwner  `json:"headRepositoryOwner"`
}

type User struct {
	Login string `json:"login"`
}

type Comment struct {
	Author    User   `json:"author"`
	Body      string `json:"body"`
	URL       string `json:"url"`
	CreatedAt string `json:"createdAt"`
}

type Review struct {
	Author      User   `json:"author"`
	State       string `json:"state"`
	Body        string `json:"body"`
	URL         string `json:"url"`
	SubmittedAt string `json:"submittedAt"`
	Commit      Commit `json:"commit"`
}

type Commit struct {
	OID           string `json:"oid"`
	CommittedDate string `json:"committedDate"`
	AuthoredDate  string `json:"authoredDate"`
}

type ReviewComment struct {
	Author           User   `json:"user"`
	Body             string `json:"body"`
	URL              string `json:"html_url"`
	Path             string `json:"path"`
	CommitID         string `json:"commit_id"`
	OriginalCommitID string `json:"original_commit_id"`
	CreatedAt        string `json:"created_at"`
}

type ReviewRequest struct {
	Login string `json:"login"`
}

type Repository struct {
	Name string `json:"name"`
}

type RepositoryOwner struct {
	Login string `json:"login"`
}

type Assessment struct {
	State    State
	Summary  string
	Findings []string
}

func Assess(pr PullRequest) Assessment {
	var blockers []string
	var waiting []string
	var notes []string

	if !strings.EqualFold(pr.State, "OPEN") {
		blockers = append(blockers, fmt.Sprintf("PR is %s", valueOrUnknown(pr.State)))
	}

	if strings.EqualFold(pr.ReviewDecision, "CHANGES_REQUESTED") {
		blockers = append(blockers, "review decision has changes requested")
	}
	for _, review := range pr.LatestReviews {
		if strings.EqualFold(review.State, "CHANGES_REQUESTED") {
			reviewer := strings.TrimSpace(review.Author.Login)
			if reviewer == "" {
				reviewer = "a reviewer"
			}
			blockers = append(blockers, fmt.Sprintf("%s has changes requested", reviewer))
		}
	}
	latestRequest := latestCodexRequest(pr.Comments)
	if count := currentHeadCodexReviewCommentCount(pr, latestRequest, pr.LatestRequestAfterHead); count > 0 {
		blockers = append(blockers, pluralize(count, "Codex review has %d current-head comment", "Codex review has %d current-head comments"))
	}

	checks := summarizeChecks(pr.StatusCheckRollup)
	blockers = append(blockers, checks.Blocked...)
	waiting = append(waiting, checks.Pending...)
	if checks.Total == 0 {
		notes = append(notes, "checks: none reported")
	} else if checks.Passing == checks.Total {
		notes = append(notes, fmt.Sprintf("checks: %d passing", checks.Passing))
	}

	switch strings.ToUpper(strings.TrimSpace(pr.MergeStateStatus)) {
	case "", "CLEAN", "HAS_HOOKS", "DRAFT":
	case "UNKNOWN":
		waiting = append(waiting, "merge state is UNKNOWN")
	case "DIRTY":
		blockers = append(blockers, "merge state is DIRTY")
	case "BLOCKED":
		if strings.EqualFold(pr.ReviewDecision, "REVIEW_REQUIRED") {
			notes = append(notes, "human review required")
		} else {
			blockers = append(blockers, "merge state is BLOCKED")
		}
	case "UNSTABLE":
		if len(checks.Pending) > 0 {
			waiting = append(waiting, "merge state is UNSTABLE while checks are pending")
		} else {
			blockers = append(blockers, "merge state is UNSTABLE")
		}
	default:
		waiting = append(waiting, fmt.Sprintf("merge state is %s", pr.MergeStateStatus))
	}

	if pr.IsDraft {
		notes = append(notes, "draft PR: human should mark ready after final review")
	}

	if !hasCodexReview(pr, latestRequest) {
		waiting = append(waiting, "Codex review has not completed")
	} else {
		notes = append(notes, "Codex review completed")
	}

	findings := dedupe(append(append([]string{}, blockers...), append(waiting, notes...)...))
	switch {
	case len(blockers) > 0:
		return Assessment{
			State:    Blocked,
			Summary:  "blocked before final human review",
			Findings: findings,
		}
	case len(waiting) > 0:
		return Assessment{
			State:    Waiting,
			Summary:  "waiting before final human review",
			Findings: findings,
		}
	default:
		return Assessment{
			State:    Ready,
			Summary:  "ready for final human review",
			Findings: findings,
		}
	}
}

func Message(pr PullRequest, assessment Assessment) string {
	var b strings.Builder
	fmt.Fprintf(&b, "PR #%d is %s: %s\n", pr.Number, assessment.State, assessment.Summary)
	if pr.Title != "" {
		fmt.Fprintf(&b, "%s\n", pr.Title)
	}
	if pr.URL != "" {
		fmt.Fprintf(&b, "%s\n", pr.URL)
	}
	if len(assessment.Findings) > 0 {
		b.WriteString("\nFindings:\n")
		for _, finding := range assessment.Findings {
			fmt.Fprintf(&b, "- %s\n", finding)
		}
	}
	return strings.TrimSpace(b.String())
}

type checkSummary struct {
	Total   int
	Passing int
	Pending []string
	Blocked []string
}

func summarizeChecks(items []map[string]any) checkSummary {
	var summary checkSummary
	for _, item := range items {
		summary.Total++
		name := firstString(item, "name", "workflowName", "context", "displayName")
		if name == "" {
			name = "unnamed check"
		}
		kind := strings.ToUpper(firstString(item, "__typename"))
		conclusion := strings.ToUpper(firstString(item, "conclusion"))
		status := strings.ToUpper(firstString(item, "status"))
		state := strings.ToUpper(firstString(item, "state"))

		switch {
		case kind == "STATUSCONTEXT":
			switch state {
			case "SUCCESS":
				summary.Passing++
			case "ERROR", "FAILURE":
				summary.Blocked = append(summary.Blocked, fmt.Sprintf("%s failed", name))
			case "PENDING", "EXPECTED":
				summary.Pending = append(summary.Pending, fmt.Sprintf("%s is still %s", name, state))
			default:
				summary.Pending = append(summary.Pending, fmt.Sprintf("%s has unknown state %s", name, valueOrUnknown(state)))
			}
		case conclusion == "SUCCESS", conclusion == "NEUTRAL", conclusion == "SKIPPED":
			summary.Passing++
		case conclusion == "FAILURE", conclusion == "TIMED_OUT", conclusion == "ACTION_REQUIRED", conclusion == "CANCELLED", conclusion == "STARTUP_FAILURE", conclusion == "STALE":
			summary.Blocked = append(summary.Blocked, fmt.Sprintf("%s failed", name))
		case status == "COMPLETED" && conclusion == "":
			summary.Pending = append(summary.Pending, fmt.Sprintf("%s completed without a conclusion", name))
		case status == "":
			summary.Pending = append(summary.Pending, fmt.Sprintf("%s has unknown status", name))
		default:
			summary.Pending = append(summary.Pending, fmt.Sprintf("%s is still %s", name, status))
		}
	}
	sort.Strings(summary.Blocked)
	sort.Strings(summary.Pending)
	return summary
}

func hasCodexReview(pr PullRequest, latestRequest time.Time) bool {
	for _, comment := range pr.Comments {
		if isCodexActor(comment.Author.Login) &&
			looksLikeCompletedCodexReview(comment.Body) &&
			completedAfter(comment.CreatedAt, latestRequest) &&
			commentMatchesHead(comment, pr.HeadRefOID, pr.LatestRequestAfterHead) {
			return true
		}
	}
	for _, reviews := range [][]Review{pr.LatestReviews, pr.Reviews} {
		for _, review := range reviews {
			if isCodexActor(review.Author.Login) &&
				!strings.EqualFold(strings.TrimSpace(review.State), "PENDING") &&
				reviewMatchesHead(review, pr.HeadRefOID) &&
				completedAfter(review.SubmittedAt, latestRequest) {
				return true
			}
		}
	}
	return false
}

func latestCodexRequest(comments []Comment) time.Time {
	var latest time.Time
	for _, comment := range comments {
		if !looksLikeCodexRequest(comment.Body) {
			continue
		}
		createdAt, ok := parseGitHubTime(comment.CreatedAt)
		if ok && createdAt.After(latest) {
			latest = createdAt
		}
	}
	return latest
}

func looksLikeCodexRequest(body string) bool {
	return strings.EqualFold(strings.TrimSpace(body), "@codex review")
}

func completedAfter(completedAt string, cutoff time.Time) bool {
	if cutoff.IsZero() {
		return true
	}
	completed, ok := parseGitHubTime(completedAt)
	return ok && !completed.Before(cutoff)
}

func reviewMatchesHead(review Review, headRefOID string) bool {
	headRefOID = strings.TrimSpace(headRefOID)
	reviewOID := strings.TrimSpace(review.Commit.OID)
	return headRefOID == "" || (reviewOID != "" && strings.EqualFold(reviewOID, headRefOID))
}

func commentMatchesHead(comment Comment, headRefOID string, latestRequestAfterHead bool) bool {
	headRefOID = strings.TrimSpace(headRefOID)
	if headRefOID == "" {
		return true
	}
	if bodyMentionsHead(comment.Body, headRefOID) {
		return true
	}
	return latestRequestAfterHead
}

func bodyMentionsHead(body, headRefOID string) bool {
	headRefOID = strings.TrimSpace(headRefOID)
	if headRefOID == "" {
		return false
	}
	body = strings.ToLower(body)
	headRefOID = strings.ToLower(headRefOID)
	if strings.Contains(body, headRefOID) {
		return true
	}
	return len(headRefOID) >= 12 && strings.Contains(body, headRefOID[:12])
}

func parseGitHubTime(raw string) (time.Time, bool) {
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(raw))
	return parsed, err == nil
}

func currentHeadCodexReviewCommentCount(pr PullRequest, latestRequest time.Time, latestRequestAfterHead bool) int {
	var count int
	for _, comment := range pr.ReviewComments {
		if !isCodexActor(comment.Author.Login) {
			continue
		}
		if !reviewCommentMatchesHead(comment, pr.HeadRefOID, latestRequest, latestRequestAfterHead) {
			continue
		}
		count++
	}
	return count
}

func reviewCommentMatchesHead(comment ReviewComment, headRefOID string, latestRequest time.Time, latestRequestAfterHead bool) bool {
	headRefOID = strings.TrimSpace(headRefOID)
	if headRefOID == "" {
		return true
	}
	originalCommitID := strings.TrimSpace(comment.OriginalCommitID)
	if originalCommitID != "" {
		if strings.EqualFold(originalCommitID, headRefOID) {
			return true
		}
		return strings.EqualFold(strings.TrimSpace(comment.CommitID), headRefOID) &&
			completedAfter(comment.CreatedAt, latestRequest) &&
			latestRequestAfterHead
	}
	return strings.EqualFold(strings.TrimSpace(comment.CommitID), headRefOID)
}

func pluralize(count int, singular, plural string) string {
	if count == 1 {
		return fmt.Sprintf(singular, count)
	}
	return fmt.Sprintf(plural, count)
}

func isCodexActor(login string) bool {
	login = strings.ToLower(strings.TrimSpace(login))
	return strings.Contains(login, "codex") || strings.Contains(login, "chatgpt")
}

func looksLikeCompletedCodexReview(body string) bool {
	body = strings.TrimSpace(body)
	lower := strings.ToLower(body)
	if lower == "@codex review" {
		return false
	}
	return strings.Contains(lower, "codex review") || strings.Contains(lower, "didn't find any major issues") || strings.Contains(lower, "did not find any major issues")
}

func firstString(item map[string]any, keys ...string) string {
	for _, key := range keys {
		if raw, ok := item[key]; ok {
			if value, ok := raw.(string); ok && strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value)
			}
		}
	}
	return ""
}

func dedupe(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func valueOrUnknown(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	return value
}
