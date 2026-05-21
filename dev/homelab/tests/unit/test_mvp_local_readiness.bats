#!/usr/bin/env bats

bats_require_minimum_version 1.5.0

setup() {
  REPO_ROOT="$(cd "$(dirname "$BATS_TEST_FILENAME")/../../../.." && pwd)"
  SCRIPT="$REPO_ROOT/dev/homelab/mvp-local-readiness.sh"
}

@test "dry run shows local MVP preflight install status and smoke commands" {
  run "$SCRIPT" --dry-run

  [ "$status" -eq 0 ]
  [[ "$output" == *"+ docker version"* ]]
  [[ "$output" == *"+ docker compose version"* ]]
  [[ "$output" == *"+ go run ./cmd/homelab install --help"* ]]
  [[ "$output" == *"+ op --version"* ]]
  [[ "$output" == *"+ go run ./cmd/homelab install --non-interactive --domain caboose-ai.io --compose-dir dev/homelab --serve-mode local"* ]]

  for slug in authentik forgejo woodpecker homarr; do
    [[ "$output" == *"+ go run ./cmd/homelab service --domain caboose-ai.io --compose-dir dev/homelab --serve-mode local $slug status"* ]]
    [[ "$output" == *"+ env SMOKETEST_DOMAIN=caboose-ai.io SMOKETEST_ENV_FILE=dev/homelab/.env SMOKETEST_RECOVER_AUTHENTIK_TOKEN=1 SMOKETEST_FORCE_LOCAL_AUTH=1 go run ./cmd/homelab service --domain caboose-ai.io --compose-dir dev/homelab --serve-mode local $slug smoke"* ]]
  done
}

@test "secrets-env-only uses compose env preflight and forwards install flag" {
  run "$SCRIPT" --dry-run --secrets-env-only

  [ "$status" -eq 0 ]
  [[ "$output" == *"+ test -f dev/homelab/.env"* ]]
  [[ "$output" != *"+ op --version"* ]]
  [[ "$output" == *"--secrets-env-only"* ]]
  [[ "$output" == *"SMOKETEST_RECOVER_AUTHENTIK_TOKEN=0"* ]]
}

@test "environment defaults are overridable while serve mode stays local" {
  run env \
    HOMELAB_DOMAIN=example.test \
    HOMELAB_COMPOSE_DIR=/tmp/local-compose \
    HOMELAB_BIN=/tmp/homelab \
    HOMELAB_SERVE_MODE=public \
    "$SCRIPT" --dry-run

  [ "$status" -eq 0 ]
  [[ "$output" == *"+ /tmp/homelab install --non-interactive --domain example.test --compose-dir /tmp/local-compose --serve-mode local"* ]]
  [[ "$output" == *"+ /tmp/homelab service --domain example.test --compose-dir /tmp/local-compose --serve-mode local authentik status"* ]]
  [[ "$output" == *"+ env SMOKETEST_DOMAIN=example.test SMOKETEST_ENV_FILE=/tmp/local-compose/.env SMOKETEST_RECOVER_AUTHENTIK_TOKEN=1 SMOKETEST_FORCE_LOCAL_AUTH=1 /tmp/homelab service --domain example.test --compose-dir /tmp/local-compose --serve-mode local authentik smoke"* ]]
  [[ "$output" != *"--serve-mode public"* ]]
}

@test "recovery reset is omitted by default and only appears with with-recovery" {
  run "$SCRIPT" --dry-run

  [ "$status" -eq 0 ]
  [[ "$output" != *" reset "* ]]

  run "$SCRIPT" --dry-run --with-recovery

  [ "$status" -eq 0 ]
  [[ "$output" == *"+ go run ./cmd/homelab reset --keep-env --yes --domain caboose-ai.io --compose-dir dev/homelab --serve-mode local"* ]]
}

@test "install errors fail the release gate before stale status and smoke checks" {
  tmpdir="$(mktemp -d)"
  mkdir -p "$tmpdir/bin" "$tmpdir/compose"
  touch "$tmpdir/compose/.env"

  cat >"$tmpdir/bin/docker" <<'EOF'
#!/usr/bin/env bash
if [[ "$1" == "compose" && "$2" == "version" ]]; then
  echo "Docker Compose version test"
else
  echo "Docker version test"
fi
EOF
  cat >"$tmpdir/bin/homelab" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$*" >>"$HOMELAB_TEST_LOG"
if [[ "$1" == "install" && "$*" != *"--help"* ]]; then
  exit 4
fi
EOF
  chmod +x "$tmpdir/bin/docker" "$tmpdir/bin/homelab"

  run env \
    PATH="$tmpdir/bin:$PATH" \
    HOMELAB_BIN=homelab \
    HOMELAB_COMPOSE_DIR="$tmpdir/compose" \
    HOMELAB_TEST_LOG="$tmpdir/homelab.log" \
    "$SCRIPT" --secrets-env-only

  [ "$status" -eq 4 ]
  [[ "$output" != *"continuing with MVP status and smoke checks"* ]]

  for slug in authentik forgejo woodpecker homarr; do
    run grep -F -- "service --domain caboose-ai.io --compose-dir $tmpdir/compose --serve-mode local $slug status" "$tmpdir/homelab.log"
    [ "$status" -ne 0 ]
    run grep -F -- "service --domain caboose-ai.io --compose-dir $tmpdir/compose --serve-mode local $slug smoke" "$tmpdir/homelab.log"
    [ "$status" -ne 0 ]
  done
}
