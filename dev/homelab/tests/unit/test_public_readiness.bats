#!/usr/bin/env bats

bats_require_minimum_version 1.5.0

setup() {
  REPO_ROOT="$(cd "$(dirname "$BATS_TEST_FILENAME")/../../../.." && pwd)"
  SCRIPT="$REPO_ROOT/dev/homelab/public-readiness.sh"
}

@test "dry run shows public profile install tunnel caddy status and smoke commands" {
  run "$SCRIPT" --dry-run

  [ "$status" -eq 0 ]
  [[ "$output" == *"+ docker version"* ]]
  [[ "$output" == *"+ docker compose version"* ]]
  [[ "$output" == *"+ go run ./cmd/homelab install --help"* ]]
  [[ "$output" == *"+ test -f /etc/caddy/Caddyfile"* ]]
  [[ "$output" == *"+ caddy validate --config /etc/caddy/Caddyfile"* ]]
  [[ "$output" == *"+ dev/homelab/setup-cloudflare-tunnel.sh --print"* ]]
  [[ "$output" == *"+ dev/homelab/setup-cloudflare-tunnel.sh --config "* ]]
  [[ "$output" == *" --validate"* ]]
  [[ "$output" == *"+ go run ./cmd/homelab install --non-interactive --domain caboose-ai.io --compose-dir dev/homelab --serve-mode public"* ]]

  for slug in authentik forgejo woodpecker homarr; do
    [[ "$output" == *"+ go run ./cmd/homelab service --domain caboose-ai.io --compose-dir dev/homelab --serve-mode public $slug status"* ]]
    [[ "$output" == *"+ env SMOKETEST_DOMAIN=caboose-ai.io SMOKETEST_ENV_FILE=dev/homelab/.env SMOKETEST_RECOVER_AUTHENTIK_TOKEN=1 SMOKETEST_FORCE_LOCAL_AUTH=0 go run ./cmd/homelab service --domain caboose-ai.io --compose-dir dev/homelab --serve-mode public $slug smoke"* ]]
  done
}

@test "environment defaults are overridable while serve mode stays public" {
  run env \
    HOMELAB_DOMAIN=example.test \
    HOMELAB_COMPOSE_DIR=/tmp/public-compose \
    HOMELAB_BIN=/tmp/homelab \
    HOMELAB_SERVE_MODE=local \
    HOMELAB_CADDY_CONFIG=/tmp/Caddyfile \
    HOMELAB_TUNNEL_CONFIG=/tmp/homelab.yml \
    "$SCRIPT" --dry-run

  [ "$status" -eq 0 ]
  [[ "$output" == *"+ /tmp/homelab install --non-interactive --domain example.test --compose-dir /tmp/public-compose --serve-mode public"* ]]
  [[ "$output" == *"+ test -f /tmp/Caddyfile"* ]]
  [[ "$output" == *"+ caddy validate --config /tmp/Caddyfile"* ]]
  [[ "$output" == *"+ dev/homelab/setup-cloudflare-tunnel.sh --config /tmp/homelab.yml --validate"* ]]
  [[ "$output" != *"--serve-mode local"* ]]
}

@test "secrets-env-only uses compose env preflight and forwards install flag" {
  run "$SCRIPT" --dry-run --secrets-env-only

  [ "$status" -eq 0 ]
  [[ "$output" == *"+ test -f dev/homelab/.env"* ]]
  [[ "$output" != *"+ op --version"* ]]
  [[ "$output" == *"--secrets-env-only"* ]]
  [[ "$output" == *"SMOKETEST_RECOVER_AUTHENTIK_TOKEN=0"* ]]
}
