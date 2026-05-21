#!/usr/bin/env bash
# Check Homebrew for a caboose-homelab update, upgrade only that formula, then
# run the homelab reset path.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FORMULA="${HOMELAB_BREW_FORMULA:-caboose-homelab}"
HOMELAB_BIN="${HOMELAB_BIN:-homelab}"
DOMAIN="${HOMELAB_DOMAIN:-caboose-ai.io}"
COMPOSE_DIR="${HOMELAB_COMPOSE_DIR:-/opt/homelab}"
READINESS_SCRIPT="${HOMELAB_STACK_READINESS_SCRIPT:-${SCRIPT_DIR}/homebrew-stack-readiness.sh}"
INTERVAL="${HOMELAB_UPDATE_POLL_INTERVAL:-300}"
WAIT=false
DRY_RUN=false
YES=false
STORE_STATIC_ENV=false

usage() {
  cat <<EOF
Usage: $0 --yes [options]

Checks Homebrew for an update to caboose-homelab, upgrades only that formula,
then runs:
  homelab reset --yes --keep-env --domain <domain> --compose-dir <dir>
  dev/homelab/homebrew-stack-readiness.sh --skip-service-status

Pass --store-static-env to run reset with --store-static-env instead of
--keep-env, which requires 1Password access and removes .env after copying
static external credentials into the static vault.

Options:
  --yes                 Confirm the destructive reset path.
  --wait                Poll until a caboose-homelab update is available.
  --store-static-env    Store static .env credentials in 1Password before reset
                        and remove .env instead of keeping it.
  --interval SECONDS    Poll interval for --wait. Default: $INTERVAL.
  --dry-run             Print planned commands without executing them.
  --formula NAME        Homebrew formula to check. Default: $FORMULA.
  --homelab-bin PATH    homelab binary to run after upgrade. Default: $HOMELAB_BIN.
  --domain DOMAIN       Homelab domain. Default: $DOMAIN.
  --compose-dir DIR     Homelab compose directory. Default: $COMPOSE_DIR.
  -h, --help            Show this help.
EOF
}

log() {
  printf '%s\n' "$*" >&2
}

die() {
  log "error: $*"
  exit 1
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

run_cmd() {
  log "+ $(join_args "$@")"
  "$@"
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --yes)
      YES=true
      ;;
    --wait)
      WAIT=true
      ;;
    --store-static-env)
      STORE_STATIC_ENV=true
      ;;
    --interval)
      shift
      [ "$#" -gt 0 ] || die "--interval requires a value"
      INTERVAL="$1"
      ;;
    --dry-run)
      DRY_RUN=true
      ;;
    --formula)
      shift
      [ "$#" -gt 0 ] || die "--formula requires a value"
      FORMULA="$1"
      ;;
    --homelab-bin)
      shift
      [ "$#" -gt 0 ] || die "--homelab-bin requires a value"
      HOMELAB_BIN="$1"
      ;;
    --domain)
      shift
      [ "$#" -gt 0 ] || die "--domain requires a value"
      DOMAIN="$1"
      ;;
    --compose-dir)
      shift
      [ "$#" -gt 0 ] || die "--compose-dir requires a value"
      COMPOSE_DIR="$1"
      ;;
    -h | --help)
      usage
      exit 0
      ;;
    *)
      die "unknown option: $1"
      ;;
  esac
  shift
done

case "$INTERVAL" in
  '' | *[!0-9]*)
    die "--interval must be a positive integer"
    ;;
  0)
    die "--interval must be greater than zero"
    ;;
esac

if "$DRY_RUN"; then
  would_run brew update
  would_run brew list --formula "$FORMULA"
  would_run brew outdated --quiet "$FORMULA"
  if "$WAIT"; then
    printf 'would wait %s seconds between update checks\n' "$INTERVAL"
  fi
  would_run brew upgrade "$FORMULA"
  reset_env_args=(--keep-env)
  if "$STORE_STATIC_ENV"; then
    reset_env_args=(--store-static-env)
  fi
  would_run "$HOMELAB_BIN" reset --yes "${reset_env_args[@]}" --domain "$DOMAIN" --compose-dir "$COMPOSE_DIR"
  would_run "$READINESS_SCRIPT" --homelab-bin "$HOMELAB_BIN" --compose-dir "$COMPOSE_DIR" --domain "$DOMAIN" --skip-service-status --dry-run
  exit 0
fi

if ! "$YES"; then
  die "rerun with --yes to allow homelab reset after upgrade"
fi

command -v brew >/dev/null 2>&1 || die "brew is required"

check_update_available() {
  run_cmd brew update
  brew list --formula "$FORMULA" >/dev/null 2>&1 || die "Homebrew formula is not installed: $FORMULA"

  local outdated
  outdated="$(brew outdated --quiet "$FORMULA" || true)"
  [ -n "$outdated" ]
}

while true; do
  if check_update_available; then
    break
  fi

  if ! "$WAIT"; then
    log "$FORMULA is already current; reset was not run."
    exit 0
  fi

  log "$FORMULA is already current; checking again in ${INTERVAL}s."
  sleep "$INTERVAL"
done

run_cmd brew upgrade "$FORMULA"
command -v "$HOMELAB_BIN" >/dev/null 2>&1 || die "homelab binary not found after upgrade: $HOMELAB_BIN"
reset_env_args=(--keep-env)
if "$STORE_STATIC_ENV"; then
  reset_env_args=(--store-static-env)
fi
run_cmd "$HOMELAB_BIN" reset --yes "${reset_env_args[@]}" --domain "$DOMAIN" --compose-dir "$COMPOSE_DIR"
run_cmd "$READINESS_SCRIPT" --homelab-bin "$HOMELAB_BIN" --compose-dir "$COMPOSE_DIR" --domain "$DOMAIN" --skip-service-status
