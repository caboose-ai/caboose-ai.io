#!/usr/bin/env bats

setup() {
  TEST_ENV=$(mktemp)
  echo "EXISTING=original" > "$TEST_ENV"

  export DOMAIN="test.example.com"
  export AUTHENTIK_TOKEN="dummy"
  export DRY_RUN="false"
  export FORCE="false"
  export VERBOSE="false"
  export HOMELAB_ENV="$TEST_ENV"

  source "$(dirname "$BATS_TEST_FILENAME")/../../lib/common.sh"
}

teardown() {
  rm -f "$TEST_ENV"
}

# ── upsert_env_var ──────────────────────────────────────────────────────────

@test "upsert_env_var: adds new key when not present" {
  upsert_env_var "$TEST_ENV" "NEW_KEY" "newvalue"
  run grep "^NEW_KEY=newvalue" "$TEST_ENV"
  [ "$status" -eq 0 ]
}

@test "upsert_env_var: replaces existing key value" {
  upsert_env_var "$TEST_ENV" "EXISTING" "updated"
  run grep "^EXISTING=updated" "$TEST_ENV"
  [ "$status" -eq 0 ]
  run grep "original" "$TEST_ENV"
  [ "$status" -ne 0 ]
}

@test "upsert_env_var: does not duplicate key" {
  upsert_env_var "$TEST_ENV" "EXISTING" "updated"
  count=$(grep -c "^EXISTING=" "$TEST_ENV")
  [ "$count" -eq 1 ]
}

@test "upsert_env_var: dry-run does not modify file" {
  DRY_RUN="true"
  upsert_env_var "$TEST_ENV" "SHOULD_NOT_EXIST" "value"
  run grep "SHOULD_NOT_EXIST" "$TEST_ENV"
  [ "$status" -ne 0 ]
}

# ── read_env_var ────────────────────────────────────────────────────────────

@test "read_env_var: reads existing value" {
  result=$(read_env_var "$TEST_ENV" "EXISTING")
  [ "$result" = "original" ]
}

@test "read_env_var: returns empty for missing key" {
  result=$(read_env_var "$TEST_ENV" "MISSING")
  [ "$result" = "" ]
}

# ── require_env ─────────────────────────────────────────────────────────────

@test "require_env: succeeds when var is set" {
  export TEST_VAR="hello"
  run require_env TEST_VAR
  [ "$status" -eq 0 ]
}

@test "require_env: fails when var is empty" {
  export TEST_VAR=""
  run require_env TEST_VAR
  [ "$status" -ne 0 ]
}

@test "require_env: fails when var is unset" {
  unset TEST_VAR 2>/dev/null || true
  run require_env TEST_VAR
  [ "$status" -ne 0 ]
}

# ── run_cmd ─────────────────────────────────────────────────────────────────

@test "run_cmd: executes command normally" {
  result=$(run_cmd echo "hello")
  [ "$result" = "hello" ]
}

@test "run_cmd: dry-run skips execution" {
  DRY_RUN="true"
  run run_cmd false
  [ "$status" -eq 0 ]
}

# ── Derived URLs ────────────────────────────────────────────────────────────

@test "derived URLs use DOMAIN" {
  [ "$AUTHENTIK_URL" = "https://auth.test.example.com" ]
  [ "$FORGEJO_EXTERNAL_URL" = "https://git.test.example.com" ]
  [ "$WOODPECKER_URL" = "https://ci.test.example.com" ]
  [ "$GRAFANA_URL" = "https://grafana.test.example.com" ]
}

# ── Logging ─────────────────────────────────────────────────────────────────

@test "log_info produces output on stderr" {
  run log_info "test message"
  [ "$status" -eq 0 ]
  [[ "$output" == *"test message"* ]]
}

@test "log_error produces output on stderr" {
  run log_error "bad thing"
  [ "$status" -eq 0 ]
  [[ "$output" == *"bad thing"* ]]
}

# ── secret_get ─────────────────────────────────────────────────────────────

@test "secret_get: reads from .env when fnox unavailable" {
  HAS_FNOX="false"
  echo "MY_SECRET=from-env" >> "$TEST_ENV"
  result=$(secret_get "MY_SECRET")
  [ "$result" = "from-env" ]
}

@test "secret_get: returns empty when key missing everywhere" {
  HAS_FNOX="false"
  result=$(secret_get "NONEXISTENT")
  [ "$result" = "" ]
}

@test "secret_get: prefers fnox when available" {
  HAS_FNOX="true"
  echo "MY_SECRET=from-env" >> "$TEST_ENV"

  fnox() { echo "from-fnox"; }
  export -f fnox

  result=$(secret_get "MY_SECRET")
  [ "$result" = "from-fnox" ]
}

@test "secret_get: falls back to .env when fnox returns empty" {
  HAS_FNOX="true"
  echo "MY_SECRET=from-env" >> "$TEST_ENV"

  fnox() { echo ""; }
  export -f fnox

  result=$(secret_get "MY_SECRET")
  [ "$result" = "from-env" ]
}

# ── secret_put ─────────────────────────────────────────────────────────────

@test "secret_put: writes to .env" {
  HAS_FNOX="false"
  secret_put "NEW_SECRET" "the-value"
  run grep "^NEW_SECRET=the-value" "$TEST_ENV"
  [ "$status" -eq 0 ]
}

@test "secret_put: writes to both .env and fnox when available" {
  HAS_FNOX="true"
  export FNOX_CALLS_FILE
  FNOX_CALLS_FILE=$(mktemp)
  fnox() { echo "$*" >> "$FNOX_CALLS_FILE"; cat > /dev/null; }
  export -f fnox

  secret_put "NEW_SECRET" "the-value"

  run grep "^NEW_SECRET=the-value" "$TEST_ENV"
  [ "$status" -eq 0 ]

  # Verify fnox was actually called with "set NEW_SECRET"
  run grep "set NEW_SECRET" "$FNOX_CALLS_FILE"
  [ "$status" -eq 0 ]

  rm -f "$FNOX_CALLS_FILE"
}

@test "secret_put: dry-run skips both stores" {
  DRY_RUN="true"
  HAS_FNOX="true"
  secret_put "SKIP_ME" "nope"
  run grep "SKIP_ME" "$TEST_ENV"
  [ "$status" -ne 0 ]
}
