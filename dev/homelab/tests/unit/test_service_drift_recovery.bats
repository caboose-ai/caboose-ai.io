#!/usr/bin/env bats

bats_require_minimum_version 1.5.0

setup() {
  TEST_ROOT="$(mktemp -d)"
  BIN_DIR="$TEST_ROOT/bin"
  CALL_LOG="$TEST_ROOT/calls.log"
  COMPOSE_DIR="$TEST_ROOT/compose"
  ENV_FILE="$COMPOSE_DIR/.env"
  SCRIPT="$(dirname "$BATS_TEST_FILENAME")/../../service-drift-recovery.sh"

  mkdir -p "$BIN_DIR" "$COMPOSE_DIR"
  cat >"$ENV_FILE" <<'EOF'
SONAR_DB_PASS=dollar$sonar
SONAR_ADMIN_PASSWORD=admin$sonar
MATTERMOST_DB_PASS=dollar$mattermost
EOF
  export PATH="$BIN_DIR:$PATH"
  export CALL_LOG

  cat >"$BIN_DIR/docker" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf 'docker %s\n' "$*" >> "$CALL_LOG"
EOF
  chmod +x "$BIN_DIR/docker"

  cat >"$BIN_DIR/go" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf 'go %s\n' "$*" >> "$CALL_LOG"
EOF
  chmod +x "$BIN_DIR/go"
}

teardown() {
  rm -rf "$TEST_ROOT"
}

@test "sonarqube db password recovery is dry-run by default and refuses mutation without yes" {
  run "$SCRIPT" sonarqube-db-password --env-file "$ENV_FILE" --compose-dir "$COMPOSE_DIR" --domain test.example.com

  [ "$status" -eq 0 ]
  [[ "$output" == *"dry-run only; pass --yes to mutate live state"* ]]
  [[ "$output" == *"docker exec -e SONAR_DB_PASS sonarqube-db"* ]]
  [[ "$output" == *"ALTER USER sonar WITH PASSWORD"* ]]
  [[ "$output" == *"docker restart sonarqube"* ]]
  [[ "$output" != *'dollar$sonar'* ]]
  [ ! -f "$CALL_LOG" ]
}

@test "sonarqube db password recovery mutates only with yes and keeps secret values out of logs" {
  run "$SCRIPT" sonarqube-db-password --yes --env-file "$ENV_FILE" --compose-dir "$COMPOSE_DIR" --domain test.example.com

  [ "$status" -eq 0 ]
  [[ "$output" != *'dollar$sonar'* ]]
  run grep -F "docker exec -e SONAR_DB_PASS sonarqube-db sh -lc" "$CALL_LOG"
  [ "$status" -eq 0 ]
  run grep -F "ALTER USER sonar WITH PASSWORD" "$CALL_LOG"
  [ "$status" -eq 0 ]
  run grep -F "docker restart sonarqube" "$CALL_LOG"
  [ "$status" -eq 0 ]
  run grep -F 'dollar$sonar' "$CALL_LOG"
  [ "$status" -ne 0 ]
}

@test "mattermost db password recovery plans and runs postgres password repair" {
  run "$SCRIPT" mattermost-db-password --yes --env-file "$ENV_FILE" --compose-dir "$COMPOSE_DIR" --domain test.example.com

  [ "$status" -eq 0 ]
  run grep -F "docker exec -e MATTERMOST_DB_PASS mattermost-db sh -lc" "$CALL_LOG"
  [ "$status" -eq 0 ]
  run grep -F "ALTER USER mattermost WITH PASSWORD" "$CALL_LOG"
  [ "$status" -eq 0 ]
  run grep -F "docker restart mattermost" "$CALL_LOG"
  [ "$status" -eq 0 ]
  run grep -F 'dollar$mattermost' "$CALL_LOG"
  [ "$status" -ne 0 ]
}

@test "sonarqube admin password recovery resets built-in admin and reruns configurator" {
  run "$SCRIPT" sonarqube-admin-password --yes --env-file "$ENV_FILE" --compose-dir "$COMPOSE_DIR" --domain test.example.com

  [ "$status" -eq 0 ]
  run grep -F "docker stop sonarqube" "$CALL_LOG"
  [ "$status" -eq 0 ]
  run grep -F "docker exec sonarqube-db psql -v ON_ERROR_STOP=1 -U sonar -d sonar" "$CALL_LOG"
  [ "$status" -eq 0 ]
  run grep -F "hash_method='PBKDF2'" "$CALL_LOG"
  [ "$status" -eq 0 ]
  run grep -F "docker start sonarqube" "$CALL_LOG"
  [ "$status" -eq 0 ]
  run grep -F "docker exec sonarqube wget -q --spider http://127.0.0.1:9000/api/system/status" "$CALL_LOG"
  [ "$status" -eq 0 ]
  run grep -F "go run ./cmd/homelab --help" "$CALL_LOG"
  [ "$status" -eq 0 ]
  run grep -F "go run ./cmd/homelab service --domain test.example.com --compose-dir $COMPOSE_DIR sonarqube configure --force" "$CALL_LOG"
  [ "$status" -eq 0 ]
  run grep -F 'admin$sonar' "$CALL_LOG"
  [ "$status" -ne 0 ]
}
