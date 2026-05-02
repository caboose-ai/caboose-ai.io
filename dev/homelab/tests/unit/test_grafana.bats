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
  source "$(dirname "$BATS_TEST_FILENAME")/../../lib/grafana.sh"
}

teardown() {
  rm -f "$TEST_ENV"
}

# ── setup_grafana — fresh install ──────────────────────────────────────────

@test "setup_grafana: writes credentials to .env" {
  curl() {
    echo '{"results":[{"name":"grafana","client_id":"gid123","client_secret":"gsec456"}]}'
  }
  export -f curl

  run setup_grafana
  [ "$status" -eq 0 ]
  [[ "$output" == *"stored"* ]]

  run grep "GF_AUTH_GENERIC_OAUTH_CLIENT_ID=gid123" "$TEST_ENV"
  [ "$status" -eq 0 ]
  run grep "GF_AUTH_GENERIC_OAUTH_CLIENT_SECRET=gsec456" "$TEST_ENV"
  [ "$status" -eq 0 ]
}

# ── setup_grafana — already configured ─────────────────────────────────────

@test "setup_grafana: skips when credentials already match" {
  echo "GF_AUTH_GENERIC_OAUTH_CLIENT_ID=gid123" >> "$TEST_ENV"
  echo "GF_AUTH_GENERIC_OAUTH_CLIENT_SECRET=gsec456" >> "$TEST_ENV"

  curl() {
    echo '{"results":[{"name":"grafana","client_id":"gid123","client_secret":"gsec456"}]}'
  }
  export -f curl

  run setup_grafana
  [ "$status" -eq 0 ]
  [[ "$output" == *"No changes needed"* ]]
}

# ── setup_grafana — force re-apply ─────────────────────────────────────────

@test "setup_grafana: --force overwrites even when matching" {
  FORCE="true"
  echo "GF_AUTH_GENERIC_OAUTH_CLIENT_ID=gid123" >> "$TEST_ENV"
  echo "GF_AUTH_GENERIC_OAUTH_CLIENT_SECRET=gsec456" >> "$TEST_ENV"

  curl() {
    echo '{"results":[{"name":"grafana","client_id":"gid123","client_secret":"gsec456"}]}'
  }
  export -f curl

  run setup_grafana
  [ "$status" -eq 0 ]
  [[ "$output" == *"stored"* ]]
}

# ── setup_grafana — different client_id ────────────────────────────────────

@test "setup_grafana: overwrites when client_id differs" {
  echo "GF_AUTH_GENERIC_OAUTH_CLIENT_ID=old-id" >> "$TEST_ENV"
  echo "GF_AUTH_GENERIC_OAUTH_CLIENT_SECRET=old-sec" >> "$TEST_ENV"

  curl() {
    echo '{"results":[{"name":"grafana","client_id":"new-id","client_secret":"new-sec"}]}'
  }
  export -f curl

  run setup_grafana
  [ "$status" -eq 0 ]
  [[ "$output" == *"Overwriting"* ]]

  run grep "GF_AUTH_GENERIC_OAUTH_CLIENT_ID=new-id" "$TEST_ENV"
  [ "$status" -eq 0 ]
}

# ── setup_grafana — provider missing ───────────────────────────────────────

@test "setup_grafana: fails when provider not in Authentik" {
  curl() {
    echo '{"results":[]}'
  }
  export -f curl

  run setup_grafana
  [ "$status" -ne 0 ]
  [[ "$output" == *"not found"* ]]
}

# ── setup_grafana — dry-run ────────────────────────────────────────────────

@test "setup_grafana: dry-run does not write to .env" {
  DRY_RUN="true"

  curl() {
    echo '{"results":[{"name":"grafana","client_id":"gid123","client_secret":"gsec456"}]}'
  }
  export -f curl

  run setup_grafana
  [ "$status" -eq 0 ]
  [[ "$output" == *"dry-run"* ]]

  run grep "GF_AUTH_GENERIC_OAUTH_CLIENT_ID" "$TEST_ENV"
  [ "$status" -ne 0 ]
}
