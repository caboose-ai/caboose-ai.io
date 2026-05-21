#!/usr/bin/env bats

setup() {
  TEST_ROOT="$(mktemp -d)"
  BIN_DIR="$TEST_ROOT/bin"
  CALL_LOG="$TEST_ROOT/calls.log"
  STATE_FILE="$TEST_ROOT/state"
  mkdir -p "$BIN_DIR"
  export PATH="$BIN_DIR:$PATH"
  export CALL_LOG
  export STATE_FILE
  export HOMELAB_DOMAIN="test.example.com"
  export HOMELAB_COMPOSE_DIR="$TEST_ROOT/compose"
  SCRIPT="$(dirname "$BATS_TEST_FILENAME")/../../update-homelab-and-reset.sh"

  mkdir -p "$HOMELAB_COMPOSE_DIR"
  touch "$HOMELAB_COMPOSE_DIR/docker-compose.yml"

  cat > "$BIN_DIR/brew" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf 'brew %s\n' "$*" >> "$CALL_LOG"
case "$1" in
  update)
    exit 0
    ;;
  list)
    exit "${BREW_LIST_STATUS:-0}"
    ;;
  outdated)
    count=0
    if [ -f "$STATE_FILE" ]; then
      count="$(cat "$STATE_FILE")"
    fi
    count=$((count + 1))
    printf '%s' "$count" > "$STATE_FILE"
    if [ -n "${BREW_OUTDATED_AFTER:-}" ]; then
      if [ "$count" -ge "$BREW_OUTDATED_AFTER" ]; then
        printf 'caboose-homelab\n'
      fi
    elif [ "${BREW_OUTDATED:-0}" = "1" ]; then
      printf 'caboose-homelab\n'
    fi
    ;;
  upgrade)
    exit 0
    ;;
  *)
    printf 'unexpected brew command: %s\n' "$*" >&2
    exit 2
    ;;
esac
EOF
  chmod +x "$BIN_DIR/brew"

  cat > "$BIN_DIR/homelab" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf 'homelab %s\n' "$*" >> "$CALL_LOG"
EOF
  chmod +x "$BIN_DIR/homelab"

  cat > "$BIN_DIR/sleep" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf 'sleep %s\n' "$*" >> "$CALL_LOG"
EOF
  chmod +x "$BIN_DIR/sleep"
}

teardown() {
  rm -rf "$TEST_ROOT"
}

@test "updates only caboose-homelab and resets when formula is outdated" {
  export BREW_OUTDATED=1

  run "$SCRIPT" --yes

  [ "$status" -eq 0 ]
  run grep -F "brew update" "$CALL_LOG"
  [ "$status" -eq 0 ]
  run grep -F "brew list --formula caboose-homelab" "$CALL_LOG"
  [ "$status" -eq 0 ]
  run grep -F "brew outdated --quiet caboose-homelab" "$CALL_LOG"
  [ "$status" -eq 0 ]
  run grep -F "brew upgrade caboose-homelab" "$CALL_LOG"
  [ "$status" -eq 0 ]
  run grep -F "homelab reset --yes --keep-env --domain test.example.com --compose-dir $HOMELAB_COMPOSE_DIR" "$CALL_LOG"
  [ "$status" -eq 0 ]
  run grep -F "homelab install --help" "$CALL_LOG"
  [ "$status" -eq 0 ]
  run grep -F "homelab reset --help" "$CALL_LOG"
  [ "$status" -eq 0 ]
}

@test "does not reset when caboose-homelab is already current" {
  export BREW_OUTDATED=0

  run "$SCRIPT" --yes

  [ "$status" -eq 0 ]
  run grep -F "brew upgrade caboose-homelab" "$CALL_LOG"
  [ "$status" -ne 0 ]
  run grep -F "homelab reset" "$CALL_LOG"
  [ "$status" -ne 0 ]
}

@test "requires --yes before running destructive reset path" {
  export BREW_OUTDATED=1

  run "$SCRIPT"

  [ "$status" -ne 0 ]
  [[ "$output" == *"--yes"* ]]
  run grep -F "brew upgrade caboose-homelab" "$CALL_LOG"
  [ "$status" -ne 0 ]
}

@test "waits until a caboose-homelab update is available" {
  export BREW_OUTDATED_AFTER=2

  run "$SCRIPT" --yes --wait --interval 1

  [ "$status" -eq 0 ]
  run grep -F "sleep 1" "$CALL_LOG"
  [ "$status" -eq 0 ]
  run grep -F "brew upgrade caboose-homelab" "$CALL_LOG"
  [ "$status" -eq 0 ]
  run grep -F "homelab reset --yes --keep-env --domain test.example.com --compose-dir $HOMELAB_COMPOSE_DIR" "$CALL_LOG"
  [ "$status" -eq 0 ]
  run grep -F "homelab install --help" "$CALL_LOG"
  [ "$status" -eq 0 ]
}

@test "can store static env in secret store before reset after upgrade" {
  export BREW_OUTDATED=1

  run "$SCRIPT" --yes --store-static-env

  [ "$status" -eq 0 ]
  run grep -F "brew upgrade caboose-homelab" "$CALL_LOG"
  [ "$status" -eq 0 ]
  run grep -F "homelab reset --yes --store-static-env --domain test.example.com --compose-dir $HOMELAB_COMPOSE_DIR" "$CALL_LOG"
  [ "$status" -eq 0 ]
  run grep -F "homelab install --help" "$CALL_LOG"
  [ "$status" -eq 0 ]
  run grep -F "homelab reset --help" "$CALL_LOG"
  [ "$status" -eq 0 ]
}

@test "dry-run prints planned actions without executing brew or homelab" {
  run "$SCRIPT" --dry-run

  [ "$status" -eq 0 ]
  [[ "$output" == *"brew update"* ]]
  [[ "$output" == *"brew upgrade caboose-homelab"* ]]
  [[ "$output" == *"homelab reset --yes --keep-env"* ]]
  [[ "$output" == *"homebrew-stack-readiness.sh --homelab-bin homelab --compose-dir $HOMELAB_COMPOSE_DIR --domain test.example.com --skip-service-status --dry-run"* ]]
  [ ! -f "$CALL_LOG" ]
}
