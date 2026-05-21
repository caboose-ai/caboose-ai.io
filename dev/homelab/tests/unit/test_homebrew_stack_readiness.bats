#!/usr/bin/env bats

bats_require_minimum_version 1.5.0

setup() {
  TEST_ROOT="$(mktemp -d)"
  BIN_DIR="$TEST_ROOT/bin"
  CALL_LOG="$TEST_ROOT/calls.log"
  COMPOSE_DIR="$TEST_ROOT/compose"
  SCRIPT="$(dirname "$BATS_TEST_FILENAME")/../../homebrew-stack-readiness.sh"

  mkdir -p "$BIN_DIR" "$COMPOSE_DIR"
  touch "$COMPOSE_DIR/docker-compose.yml"
  export PATH="$BIN_DIR:$PATH"
  export CALL_LOG

  cat >"$BIN_DIR/homelab" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf 'homelab %s\n' "$*" >> "$CALL_LOG"
EOF
  chmod +x "$BIN_DIR/homelab"
}

teardown() {
  rm -rf "$TEST_ROOT"
}

@test "dry-run validates installed binary and compose directory without executing homelab" {
  run "$SCRIPT" --dry-run --homelab-bin homelab --compose-dir "$COMPOSE_DIR" --domain test.example.com

  [ "$status" -eq 0 ]
  [[ "$output" == *"would run: command -v homelab"* ]]
  [[ "$output" == *"would run: test -f $COMPOSE_DIR/docker-compose.yml"* ]]
  [[ "$output" == *"would run: homelab install --help"* ]]
  [[ "$output" == *"would run: homelab reset --help"* ]]
  [[ "$output" == *"would run: homelab service --domain test.example.com --compose-dir $COMPOSE_DIR authentik status"* ]]
  [ ! -f "$CALL_LOG" ]
}

@test "installed binary guard runs non-mutating checks against compose dir" {
  run "$SCRIPT" --homelab-bin homelab --compose-dir "$COMPOSE_DIR" --domain test.example.com

  [ "$status" -eq 0 ]
  run grep -F "homelab install --help" "$CALL_LOG"
  [ "$status" -eq 0 ]
  run grep -F "homelab reset --help" "$CALL_LOG"
  [ "$status" -eq 0 ]
  run grep -F "homelab service --domain test.example.com --compose-dir $COMPOSE_DIR authentik status" "$CALL_LOG"
  [ "$status" -eq 0 ]
}

@test "installed binary guard can skip live service status for post-reset validation" {
  run "$SCRIPT" --dry-run --skip-service-status --homelab-bin homelab --compose-dir "$COMPOSE_DIR" --domain test.example.com

  [ "$status" -eq 0 ]
  [[ "$output" == *"would run: homelab install --help"* ]]
  [[ "$output" != *" authentik status"* ]]
}
