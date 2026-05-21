#!/usr/bin/env bash
# Validate the supported Homebrew operator shape: installed binary plus compose dir.
set -euo pipefail

HOMELAB_BIN="${HOMELAB_BIN:-homelab}"
DOMAIN="${HOMELAB_DOMAIN:-caboose-ai.io}"
COMPOSE_DIR="${HOMELAB_COMPOSE_DIR:-/opt/homelab}"
SERVICE="${HOMELAB_READINESS_SERVICE:-authentik}"
DRY_RUN=false
SKIP_SERVICE_STATUS=false

usage() {
  cat <<EOF
Usage: $0 [options]

Validate that an installed homelab binary can operate against a compose
directory supplied by a source checkout or deployed stack directory.

Options:
  --dry-run              Print planned checks without executing them.
  --skip-service-status  Skip live service status; useful immediately after reset.
  --homelab-bin PATH     homelab binary to validate. Default: $HOMELAB_BIN.
  --compose-dir DIR      Homelab compose directory. Default: $COMPOSE_DIR.
  --domain DOMAIN        Homelab domain. Default: $DOMAIN.
  --service SLUG         Service status slug. Default: $SERVICE.
  -h, --help             Show this help.
EOF
}

join_args() {
  local out="" arg
  for arg in "$@"; do
    if [ -n "$out" ]; then
      out+=" "
    fi
    out+="$arg"
  done
  printf '%s' "$out"
}

would_run() {
  printf 'would run: %s\n' "$(join_args "$@")"
}

run_check() {
  if "$DRY_RUN"; then
    would_run "$@"
    return 0
  fi
  "$@"
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --dry-run)
      DRY_RUN=true
      ;;
    --skip-service-status)
      SKIP_SERVICE_STATUS=true
      ;;
    --homelab-bin)
      shift
      [ "$#" -gt 0 ] || { echo "error: --homelab-bin requires a value" >&2; exit 1; }
      HOMELAB_BIN="$1"
      ;;
    --compose-dir)
      shift
      [ "$#" -gt 0 ] || { echo "error: --compose-dir requires a value" >&2; exit 1; }
      COMPOSE_DIR="$1"
      ;;
    --domain)
      shift
      [ "$#" -gt 0 ] || { echo "error: --domain requires a value" >&2; exit 1; }
      DOMAIN="$1"
      ;;
    --service)
      shift
      [ "$#" -gt 0 ] || { echo "error: --service requires a value" >&2; exit 1; }
      SERVICE="$1"
      ;;
    -h | --help)
      usage
      exit 0
      ;;
    *)
      echo "error: unknown option: $1" >&2
      exit 1
      ;;
  esac
  shift
done

run_check command -v "$HOMELAB_BIN"
run_check test -d "$COMPOSE_DIR"
run_check test -f "$COMPOSE_DIR/docker-compose.yml"
run_check "$HOMELAB_BIN" install --help
run_check "$HOMELAB_BIN" reset --help

if ! "$SKIP_SERVICE_STATUS"; then
  run_check "$HOMELAB_BIN" service --domain "$DOMAIN" --compose-dir "$COMPOSE_DIR" "$SERVICE" status
fi
