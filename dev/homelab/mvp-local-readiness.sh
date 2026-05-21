#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: dev/homelab/mvp-local-readiness.sh [--dry-run] [--secrets-env-only] [--with-recovery]

Run the local MVP release-readiness gate:
  - prerequisite checks
  - local-mode non-interactive install
  - status and smoke checks for authentik, forgejo, woodpecker, and homarr

Options:
  --dry-run           Print commands without running them.
  --secrets-env-only  Use compose .env as the only secret source.
  --with-recovery     Also run reset --keep-env, reinstall, and smoke again.
  -h, --help          Show this help.

Environment:
  HOMELAB_DOMAIN       Domain to pass to homelab. Default: caboose-ai.io
  HOMELAB_COMPOSE_DIR  Compose directory. Default: dev/homelab
  HOMELAB_BIN          homelab binary path. Default: go run ./cmd/homelab
EOF
}

dry_run=0
secrets_env_only=0
with_recovery=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run)
      dry_run=1
      ;;
    --secrets-env-only)
      secrets_env_only=1
      ;;
    --with-recovery)
      with_recovery=1
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown option: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
  shift
done

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$REPO_ROOT"

HOMELAB_DOMAIN="${HOMELAB_DOMAIN:-caboose-ai.io}"
HOMELAB_COMPOSE_DIR="${HOMELAB_COMPOSE_DIR:-dev/homelab}"
HOMELAB_BIN="${HOMELAB_BIN:-}"
readonly HOMELAB_DOMAIN HOMELAB_COMPOSE_DIR HOMELAB_BIN
readonly SERVE_MODE="local"
readonly MVP_SERVICES=(authentik forgejo woodpecker homarr)

if [[ -n "$HOMELAB_BIN" ]]; then
  readonly HOMELAB_CMD=("$HOMELAB_BIN")
else
  readonly HOMELAB_CMD=(go run ./cmd/homelab)
fi

print_cmd() {
  local sep=""
  printf '+ '
  for arg in "$@"; do
    printf '%s%s' "$sep" "$arg"
    sep=" "
  done
  printf '\n'
}

run_cmd() {
  print_cmd "$@"
  if [[ "$dry_run" -eq 0 ]]; then
    "$@"
  fi
}

run_preflight() {
  echo "==> Preflight"
  run_cmd docker version
  run_cmd docker compose version
  run_cmd "${HOMELAB_CMD[@]}" install --help

  if [[ "$secrets_env_only" -eq 1 ]]; then
    run_cmd test -f "$HOMELAB_COMPOSE_DIR/.env"
  else
    run_cmd op --version
  fi
}

install_args() {
  local args=(
    "${HOMELAB_CMD[@]}"
    install
    --non-interactive
    --domain "$HOMELAB_DOMAIN"
    --compose-dir "$HOMELAB_COMPOSE_DIR"
    --serve-mode "$SERVE_MODE"
  )
  if [[ "$secrets_env_only" -eq 1 ]]; then
    args+=(--secrets-env-only)
  fi
  printf '%s\0' "${args[@]}"
}

run_install() {
  echo "==> Install local profile"
  local -a args=()
  while IFS= read -r -d '' arg; do
    args+=("$arg")
  done < <(install_args)
  run_cmd "${args[@]}"
}

smoketest_recover_authentik_token() {
  if [[ -n "${SMOKETEST_RECOVER_AUTHENTIK_TOKEN+x}" ]]; then
    printf '%s' "$SMOKETEST_RECOVER_AUTHENTIK_TOKEN"
  elif [[ "$secrets_env_only" -eq 1 ]]; then
    printf '0'
  else
    printf '1'
  fi
}

run_status_and_smoke() {
  echo "==> Local MVP status and smoke"
  local recover_authentik_token
  recover_authentik_token="$(smoketest_recover_authentik_token)"
  local slug
  for slug in "${MVP_SERVICES[@]}"; do
    run_cmd "${HOMELAB_CMD[@]}" service \
      --domain "$HOMELAB_DOMAIN" \
      --compose-dir "$HOMELAB_COMPOSE_DIR" \
      --serve-mode "$SERVE_MODE" \
      "$slug" status
    run_cmd env \
      SMOKETEST_DOMAIN="$HOMELAB_DOMAIN" \
      SMOKETEST_ENV_FILE="$HOMELAB_COMPOSE_DIR/.env" \
      SMOKETEST_RECOVER_AUTHENTIK_TOKEN="$recover_authentik_token" \
      SMOKETEST_FORCE_LOCAL_AUTH="${SMOKETEST_FORCE_LOCAL_AUTH:-1}" \
      "${HOMELAB_CMD[@]}" service \
      --domain "$HOMELAB_DOMAIN" \
      --compose-dir "$HOMELAB_COMPOSE_DIR" \
      --serve-mode "$SERVE_MODE" \
      "$slug" smoke
  done
}

run_recovery() {
  echo "==> Recovery reset and reinstall"
  run_cmd "${HOMELAB_CMD[@]}" reset \
    --keep-env \
    --yes \
    --domain "$HOMELAB_DOMAIN" \
    --compose-dir "$HOMELAB_COMPOSE_DIR" \
    --serve-mode "$SERVE_MODE"
  run_install
  run_status_and_smoke
}

run_preflight
run_install
run_status_and_smoke

if [[ "$with_recovery" -eq 1 ]]; then
  run_recovery
fi
