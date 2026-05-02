#!/usr/bin/env bats

setup() {
  export DOMAIN="test.example.com"
  export AUTHENTIK_TOKEN="test-token"
  export DRY_RUN="false"
  export FORCE="false"
  export VERBOSE="false"
  export HOMELAB_ENV="/dev/null"
  export FORGEJO_CONTAINER="forgejo"

  export GITHUB_OAUTH_CLIENT_ID=""
  export GITHUB_OAUTH_CLIENT_SECRET=""
  export GOOGLE_OAUTH_CLIENT_ID=""
  export GOOGLE_OAUTH_CLIENT_SECRET=""

  source "$(dirname "$BATS_TEST_FILENAME")/../../lib/common.sh"
  source "$(dirname "$BATS_TEST_FILENAME")/../../lib/authentik.sh"
  source "$(dirname "$BATS_TEST_FILENAME")/../../lib/social.sh"
}

# ── upsert_oauth_source — create ───────────────────────────────────────────

@test "upsert_oauth_source: creates source when not found" {
  curl() {
    case "$*" in
      *"sources/oauth/?slug"*)
        echo '{"results":[]}'
        ;;
      *POST*"sources/oauth/"*)
        echo '{"name":"GitHub","pk":"abc-123"}'
        ;;
    esac
  }
  export -f curl

  run upsert_oauth_source "github" "GitHub" "github" "gh-id" "gh-sec"
  [ "$status" -eq 0 ]
  [[ "$output" == *"created"* ]]
}

# ── upsert_oauth_source — update ───────────────────────────────────────────

@test "upsert_oauth_source: updates source when it exists" {
  curl() {
    case "$*" in
      *"sources/oauth/?slug"*)
        echo '{"results":[{"pk":"existing-pk"}]}'
        ;;
      *PATCH*"sources/oauth/existing-pk"*)
        echo '{"name":"GitHub","pk":"existing-pk"}'
        ;;
    esac
  }
  export -f curl

  run upsert_oauth_source "github" "GitHub" "github" "gh-id" "gh-sec"
  [ "$status" -eq 0 ]
  [[ "$output" == *"updated"* ]]
}

# ── upsert_oauth_source — skips when creds empty ──────────────────────────

@test "upsert_oauth_source: skips when client_id empty" {
  run upsert_oauth_source "github" "GitHub" "github" "" "gh-sec"
  [ "$status" -eq 0 ]
  [[ "$output" == *"skip"* ]]
}

# ── setup_social — no providers set ────────────────────────────────────────

@test "setup_social: warns when no providers configured" {
  run setup_social
  [ "$status" -eq 0 ]
  [[ "$output" == *"No social login"* ]]
}

# ── setup_social — GitHub only ─────────────────────────────────────────────

@test "setup_social: configures GitHub when set" {
  GITHUB_OAUTH_CLIENT_ID="gh-id"
  GITHUB_OAUTH_CLIENT_SECRET="gh-sec"

  curl() {
    case "$*" in
      *"sources/oauth/?slug"*)
        echo '{"results":[]}'
        ;;
      *POST*"sources/oauth/"*)
        echo '{"name":"GitHub","pk":"new-pk"}'
        ;;
    esac
  }
  export -f curl

  run setup_social
  [ "$status" -eq 0 ]
  [[ "$output" == *"GitHub"*"created"* ]]
  [[ "$output" == *"skip"*"Google"* ]]
}

# ── setup_social — both providers ──────────────────────────────────────────

@test "setup_social: configures both when both set" {
  GITHUB_OAUTH_CLIENT_ID="gh-id"
  GITHUB_OAUTH_CLIENT_SECRET="gh-sec"
  GOOGLE_OAUTH_CLIENT_ID="go-id"
  GOOGLE_OAUTH_CLIENT_SECRET="go-sec"

  curl() {
    case "$*" in
      *"sources/oauth/?slug"*)
        echo '{"results":[]}'
        ;;
      *POST*"sources/oauth/"*)
        echo '{"name":"Source","pk":"new-pk"}'
        ;;
    esac
  }
  export -f curl

  run setup_social
  [ "$status" -eq 0 ]
  [[ "$output" == *"configured"* ]]
}

# ── setup_social — dry-run ─────────────────────────────────────────────────

@test "setup_social: dry-run does not create sources" {
  DRY_RUN="true"
  GITHUB_OAUTH_CLIENT_ID="gh-id"
  GITHUB_OAUTH_CLIENT_SECRET="gh-sec"

  curl() {
    case "$*" in
      *"sources/oauth/?slug"*)
        echo '{"results":[]}'
        ;;
      *POST*)
        echo "SHOULD NOT POST IN DRY RUN"; return 1
        ;;
    esac
  }
  export -f curl

  run setup_social
  [ "$status" -eq 0 ]
  [[ "$output" == *"dry-run"* ]]
}
