#!/usr/bin/env bats

setup() {
  TEST_ENV=$(mktemp)
  echo "SOME_OTHER_KEY=value" > "$TEST_ENV"

  export DOMAIN="test.example.com"
  export AUTHENTIK_TOKEN="test-token"
  export DRY_RUN="false"
  export FORCE="false"
  export VERBOSE="false"
  export HOMELAB_ENV="$TEST_ENV"
  export FORGEJO_CONTAINER="forgejo"

  source "$(dirname "$BATS_TEST_FILENAME")/../../lib/common.sh"
  source "$(dirname "$BATS_TEST_FILENAME")/../../lib/authentik.sh"
  source "$(dirname "$BATS_TEST_FILENAME")/../../lib/open-webui.sh"
}

teardown() {
  rm -f "$TEST_ENV"
}

# ── setup_open_webui — fresh install ───────────────────────────────────────

@test "setup_open_webui: writes credentials to .env" {
  curl() {
    echo '{"results":[{"name":"open-webui","client_id":"owid123","client_secret":"owsec456"}]}'
  }
  export -f curl

  run setup_open_webui
  [ "$status" -eq 0 ]
  [[ "$output" == *"stored"* ]]

  run grep "OPEN_WEBUI_OAUTH_CLIENT_ID=owid123" "$TEST_ENV"
  [ "$status" -eq 0 ]
  run grep "OPEN_WEBUI_OAUTH_CLIENT_SECRET=owsec456" "$TEST_ENV"
  [ "$status" -eq 0 ]
}

# ── setup_open_webui — already configured ──────────────────────────────────

@test "setup_open_webui: skips when credentials already match" {
  echo "OPEN_WEBUI_OAUTH_CLIENT_ID=owid123" >> "$TEST_ENV"
  echo "OPEN_WEBUI_OAUTH_CLIENT_SECRET=owsec456" >> "$TEST_ENV"

  curl() {
    echo '{"results":[{"name":"open-webui","client_id":"owid123","client_secret":"owsec456"}]}'
  }
  export -f curl

  run setup_open_webui
  [ "$status" -eq 0 ]
  [[ "$output" == *"No changes needed"* ]]
}

# ── setup_open_webui — force ───────────────────────────────────────────────

@test "setup_open_webui: --force overwrites even when matching" {
  FORCE="true"
  echo "OPEN_WEBUI_OAUTH_CLIENT_ID=owid123" >> "$TEST_ENV"
  echo "OPEN_WEBUI_OAUTH_CLIENT_SECRET=owsec456" >> "$TEST_ENV"

  curl() {
    echo '{"results":[{"name":"open-webui","client_id":"owid123","client_secret":"owsec456"}]}'
  }
  export -f curl

  run setup_open_webui
  [ "$status" -eq 0 ]
  [[ "$output" == *"stored"* ]]
}

# ── setup_open_webui — provider missing ────────────────────────────────────

@test "setup_open_webui: fails when provider not in Authentik" {
  curl() {
    echo '{"results":[]}'
  }
  export -f curl

  run setup_open_webui
  [ "$status" -ne 0 ]
  [[ "$output" == *"not found"* ]]
}

# ── setup_open_webui — dry-run ─────────────────────────────────────────────

@test "setup_open_webui: dry-run does not write to .env" {
  DRY_RUN="true"

  curl() {
    echo '{"results":[{"name":"open-webui","client_id":"owid123","client_secret":"owsec456"}]}'
  }
  export -f curl

  run setup_open_webui
  [ "$status" -eq 0 ]
  [[ "$output" == *"dry-run"* ]]

  run grep "OPEN_WEBUI_OAUTH_CLIENT_ID" "$TEST_ENV"
  [ "$status" -ne 0 ]
}
