#!/usr/bin/env bats

setup() {
  export DOMAIN="test.example.com"
  export AUTHENTIK_TOKEN="test-token"
  export DRY_RUN="false"
  export FORCE="false"
  export VERBOSE="false"
  export HOMELAB_ENV="/dev/null"
  export FORGEJO_CONTAINER="forgejo"
  export MATTERMOST_CONTAINER="mattermost"

  source "$(dirname "$BATS_TEST_FILENAME")/../../lib/common.sh"
  source "$(dirname "$BATS_TEST_FILENAME")/../../lib/authentik.sh"
  source "$(dirname "$BATS_TEST_FILENAME")/../../lib/mattermost.sh"
}

# ── setup_mattermost — container not found ─────────────────────────────────

@test "setup_mattermost: skips gracefully when container missing" {
  docker() { return 1; }
  export -f docker

  run setup_mattermost
  [ "$status" -eq 0 ]
  [[ "$output" == *"not found"* ]]
}

# ── setup_mattermost — fresh config ────────────────────────────────────────

@test "setup_mattermost: patches config.json in container" {
  docker() {
    case "$*" in
      *inspect*)
        echo '{"Id":"abc"}'
        ;;
      *"jq -r"*)
        echo ""
        ;;
      *"jq '"*)
        ;;
    esac
  }
  export -f docker

  curl() {
    echo '{"results":[{"name":"mattermost","client_id":"mmid123","client_secret":"mmsec456"}]}'
  }
  export -f curl

  run setup_mattermost
  [ "$status" -eq 0 ]
  [[ "$output" == *"configured"* ]]
}

# ── setup_mattermost — already configured ──────────────────────────────────

@test "setup_mattermost: skips when client_id already matches" {
  docker() {
    case "$*" in
      *inspect*)
        echo '{"Id":"abc"}'
        ;;
      *"jq -r"*)
        echo "mmid123"
        ;;
      *"jq '"*)
        echo "SHOULD NOT PATCH"; return 1
        ;;
    esac
  }
  export -f docker

  curl() {
    echo '{"results":[{"name":"mattermost","client_id":"mmid123","client_secret":"mmsec456"}]}'
  }
  export -f curl

  run setup_mattermost
  [ "$status" -eq 0 ]
  [[ "$output" == *"No changes needed"* ]]
}

# ── setup_mattermost — force ──────────────────────────────────────────────

@test "setup_mattermost: --force re-patches even when matching" {
  FORCE="true"

  docker() {
    case "$*" in
      *inspect*)
        echo '{"Id":"abc"}'
        ;;
      *"jq '"*)
        ;;
    esac
  }
  export -f docker

  curl() {
    echo '{"results":[{"name":"mattermost","client_id":"mmid123","client_secret":"mmsec456"}]}'
  }
  export -f curl

  run setup_mattermost
  [ "$status" -eq 0 ]
  [[ "$output" == *"configured"* ]]
}

# ── setup_mattermost — provider missing ────────────────────────────────────

@test "setup_mattermost: fails when provider not in Authentik" {
  docker() {
    case "$*" in
      *inspect*) echo '{"Id":"abc"}' ;;
    esac
  }
  export -f docker

  curl() {
    echo '{"results":[]}'
  }
  export -f curl

  run setup_mattermost
  [ "$status" -ne 0 ]
  [[ "$output" == *"not found"* ]]
}

# ── setup_mattermost — dry-run ─────────────────────────────────────────────

@test "setup_mattermost: dry-run does not patch" {
  DRY_RUN="true"

  docker() {
    case "$*" in
      *inspect*)
        echo '{"Id":"abc"}'
        ;;
      *"jq -r"*)
        echo ""
        ;;
      *"jq '"*)
        echo "SHOULD NOT PATCH IN DRY RUN"; return 1
        ;;
    esac
  }
  export -f docker

  curl() {
    echo '{"results":[{"name":"mattermost","client_id":"mmid123","client_secret":"mmsec456"}]}'
  }
  export -f curl

  run setup_mattermost
  [ "$status" -eq 0 ]
  [[ "$output" == *"dry-run"* ]]
}
