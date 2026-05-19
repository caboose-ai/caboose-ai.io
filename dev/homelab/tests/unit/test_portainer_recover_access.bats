#!/usr/bin/env bats

setup() {
  TEST_ROOT="$(mktemp -d)"
  BIN_DIR="$TEST_ROOT/bin"
  CALL_LOG="$TEST_ROOT/calls.log"
  AUTH_READY="$TEST_ROOT/auth-ready"
  AUTH_COUNT="$TEST_ROOT/auth-count"
  COMPOSE_DIR="$TEST_ROOT/compose"
  ENV_FILE="$COMPOSE_DIR/.env"
  SCRIPT="$(dirname "$BATS_TEST_FILENAME")/../../portainer-recover-access.sh"

  mkdir -p "$BIN_DIR" "$COMPOSE_DIR"
  printf 'PORTAINER_ADMIN_PASSWORD=stored-secret\n' > "$ENV_FILE"
  export PATH="$BIN_DIR:$PATH"
  export CALL_LOG AUTH_READY AUTH_COUNT

  cat > "$BIN_DIR/curl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf 'curl %s\n' "$*" >> "$CALL_LOG"
case "$*" in
  *"/api/system/status"*)
    exit 0
    ;;
  *"/api/auth"*)
    count=0
    if [ -f "$AUTH_COUNT" ]; then
      count="$(cat "$AUTH_COUNT")"
    fi
    count=$((count + 1))
    printf '%s' "$count" > "$AUTH_COUNT"
    if [ -f "$AUTH_READY" ]; then
      printf '{"jwt":"test-jwt"}\n200'
    else
      printf '{"message":"Invalid credentials"}\n422'
    fi
    ;;
  *)
    printf 'unexpected curl call: %s\n' "$*" >&2
    exit 2
    ;;
esac
EOF
  chmod +x "$BIN_DIR/curl"

  cat > "$BIN_DIR/docker" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf 'docker %s\n' "$*" >> "$CALL_LOG"
case "$1" in
  inspect)
    printf 'homelab_portainer_data\n'
    ;;
  stop)
    exit 0
    ;;
  run)
    if [ "${FAIL_RESET:-0}" = "1" ]; then
      exit 7
    fi
    case "$*" in
      *"portainer/helper-reset-password"*"--password"*)
        touch "$AUTH_READY"
        ;;
      *)
        printf 'unexpected docker run: %s\n' "$*" >&2
        exit 2
        ;;
    esac
    ;;
  start)
    exit 0
    ;;
  *)
    printf 'unexpected docker command: %s\n' "$*" >&2
    exit 2
    ;;
esac
EOF
  chmod +x "$BIN_DIR/docker"

  cat > "$BIN_DIR/go" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf 'go %s\n' "$*" >> "$CALL_LOG"
EOF
  chmod +x "$BIN_DIR/go"
}

teardown() {
  rm -rf "$TEST_ROOT"
}

@test "dry-run prints planned Portainer recovery without touching Docker or API" {
  run "$SCRIPT" --dry-run --yes --compose-dir "$COMPOSE_DIR" --domain test.example.com

  [ "$status" -eq 0 ]
  [[ "$output" == *"would check Portainer health"* ]]
  [[ "$output" == *"would reset Portainer admin password with portainer/helper-reset-password"* ]]
  [ ! -f "$CALL_LOG" ]
}

@test "refuses admin password reset unless --yes is passed" {
  run "$SCRIPT" --compose-dir "$COMPOSE_DIR" --domain test.example.com

  [ "$status" -ne 0 ]
  [[ "$output" == *"stored Portainer admin password does not authenticate"* ]]
  [[ "$output" == *"--yes"* ]]
  run grep -F "docker stop" "$CALL_LOG"
  [ "$status" -ne 0 ]
  run grep -F "portainer/helper-reset-password" "$CALL_LOG"
  [ "$status" -ne 0 ]
}

@test "resets drifted admin password, restarts Portainer, and reapplies service configuration" {
  run "$SCRIPT" --yes --compose-dir "$COMPOSE_DIR" --domain test.example.com

  [ "$status" -eq 0 ]
  run grep -F "docker inspect portainer" "$CALL_LOG"
  [ "$status" -eq 0 ]
  run grep -F "docker stop portainer" "$CALL_LOG"
  [ "$status" -eq 0 ]
  run grep -F "docker run --rm -v homelab_portainer_data:/data portainer/helper-reset-password --password stored-secret" "$CALL_LOG"
  [ "$status" -eq 0 ]
  run grep -F "docker start portainer" "$CALL_LOG"
  [ "$status" -eq 0 ]
  run grep -F "go run ./cmd/homelab service --domain test.example.com --compose-dir $COMPOSE_DIR portainer configure --force" "$CALL_LOG"
  [ "$status" -eq 0 ]
}

@test "uses successful admin auth to reapply configuration without reset" {
  touch "$AUTH_READY"

  run "$SCRIPT" --compose-dir "$COMPOSE_DIR" --domain test.example.com

  [ "$status" -eq 0 ]
  run grep -F "portainer/helper-reset-password" "$CALL_LOG"
  [ "$status" -ne 0 ]
  run grep -F "go run ./cmd/homelab service --domain test.example.com --compose-dir $COMPOSE_DIR portainer configure --force" "$CALL_LOG"
  [ "$status" -eq 0 ]
}

@test "reads generated password characters literally from env file" {
  printf 'PORTAINER_ADMIN_PASSWORD=dollar$secret\n' > "$ENV_FILE"

  run "$SCRIPT" --yes --compose-dir "$COMPOSE_DIR" --domain test.example.com

  [ "$status" -eq 0 ]
  run grep -F 'docker run --rm -v homelab_portainer_data:/data portainer/helper-reset-password --password dollar$secret' "$CALL_LOG"
  [ "$status" -eq 0 ]
}

@test "restarts Portainer when password reset helper fails after stop" {
  export FAIL_RESET=1

  run "$SCRIPT" --yes --compose-dir "$COMPOSE_DIR" --domain test.example.com

  [ "$status" -ne 0 ]
  run grep -F "docker stop portainer" "$CALL_LOG"
  [ "$status" -eq 0 ]
  run grep -F "docker start portainer" "$CALL_LOG"
  [ "$status" -eq 0 ]
}
