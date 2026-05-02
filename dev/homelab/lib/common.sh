#!/usr/bin/env bash
# lib/common.sh — Shared helpers for the SSO CLI
# Source this file; do not execute directly.

# ── Global flags (set by sso.sh before sourcing) ────────────────────────────
DRY_RUN="${DRY_RUN:-false}"
FORCE="${FORCE:-false}"
VERBOSE="${VERBOSE:-false}"

# ── Required environment ────────────────────────────────────────────────────
DOMAIN="${DOMAIN:?Set DOMAIN to your homelab domain (e.g. example.com)}"
FORGEJO_ADMIN_USERNAME="${FORGEJO_ADMIN_USERNAME:-admin}"
FORGEJO_CONTAINER="${FORGEJO_CONTAINER:-forgejo}"
HOMELAB_DIR="${HOMELAB_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
HOMELAB_ENV="${HOMELAB_ENV:-${HOMELAB_DIR}/.env}"

# ── Derived URLs ────────────────────────────────────────────────────────────
AUTHENTIK_URL="https://auth.${DOMAIN}"
FORGEJO_EXTERNAL_URL="https://git.${DOMAIN}"
PORTAINER_URL="${PORTAINER_URL:-http://127.0.0.1:9000}"
WOODPECKER_URL="https://ci.${DOMAIN}"
GRAFANA_URL="https://grafana.${DOMAIN}"

# ── Logging ─────────────────────────────────────────────────────────────────

_log() {
  local level="$1" color="$2" msg="$3"
  if [[ "$VERBOSE" == "true" ]]; then
    printf "%b[%s %s]%b %s\n" "$color" "$(date +%H:%M:%S)" "$level" "\033[0m" "$msg" >&2
  else
    printf "%b[%s]%b %s\n" "$color" "$level" "\033[0m" "$msg" >&2
  fi
}

log_info()    { _log "info"    "\033[0;36m" "$1"; }
log_warn()    { _log "warn"    "\033[0;33m" "$1"; }
log_error()   { _log "error"   "\033[0;31m" "$1"; }
log_success() { _log "ok"      "\033[0;32m" "$1"; }

log_step() {
  printf "\n\033[1;37m==> %s\033[0m\n" "$1" >&2
}

# ── Validation ──────────────────────────────────────────────────────────────

require_env() {
  local var_name="$1"
  local hint="${2:-export ${var_name}=<value>}"
  if [[ -z "${!var_name:-}" ]]; then
    log_error "$var_name is not set. $hint"
    return 1
  fi
}

require_cmd() {
  local cmd="$1"
  if ! command -v "$cmd" &>/dev/null; then
    log_error "Required command not found: $cmd"
    return 1
  fi
}

# ── Dry-run wrapper ─────────────────────────────────────────────────────────

run_cmd() {
  if [[ "$DRY_RUN" == "true" ]]; then
    log_info "[dry-run] $*"
    return 0
  fi
  "$@"
}

# ── Secret store (fnox integration — optional) ─────────────────────────────
# When fnox is available, secrets are read from fnox first and written to both
# fnox and .env. When fnox is absent, .env is the only store.

HAS_FNOX="false"
if command -v fnox &>/dev/null; then
  HAS_FNOX="true"
fi

secret_get() {
  local key="$1"
  if [[ "$HAS_FNOX" == "true" ]]; then
    local val
    val=$(fnox get "$key" --if-missing ignore 2>/dev/null) || true
    if [[ -n "$val" ]]; then
      echo "$val"
      return 0
    fi
  fi
  read_env_var "$HOMELAB_ENV" "$key"
}

secret_put() {
  local key="$1" value="$2"
  upsert_env_var "$HOMELAB_ENV" "$key" "$value"
  if [[ "$HAS_FNOX" == "true" && "$DRY_RUN" != "true" ]]; then
    echo "$value" | fnox set "$key" 2>/dev/null && \
      log_info "  ↳ backed up ${key} to fnox" || \
      log_warn "  ↳ fnox set ${key} failed (non-fatal)"
  fi
}

# ── Environment file helpers ────────────────────────────────────────────────

upsert_env_var() {
  local file="$1" key="$2" value="$3"
  if [[ "$DRY_RUN" == "true" ]]; then
    log_info "[dry-run] upsert $key=<redacted> in $file"
    return 0
  fi
  if grep -q "^${key}=" "$file" 2>/dev/null; then
    sed -i "s|^${key}=.*|${key}=${value}|" "$file"
  else
    echo "${key}=${value}" >> "$file"
  fi
}

read_env_var() {
  local file="$1" key="$2"
  grep "^${key}=" "$file" 2>/dev/null | cut -d= -f2- || true
}

# ── HTTP helpers ────────────────────────────────────────────────────────────

curl_api() {
  local method="$1" url="$2"
  shift 2
  local curl_args=(-sf -X "$method" "$url")

  while [[ $# -gt 0 ]]; do
    case "$1" in
      -H|--header)  curl_args+=(-H "$2"); shift 2 ;;
      -d|--data)    curl_args+=(-d "$2"); shift 2 ;;
      -u|--user)    curl_args+=(-u "$2"); shift 2 ;;
      *)            curl_args+=("$1"); shift ;;
    esac
  done

  if [[ "$VERBOSE" == "true" ]]; then
    log_info "curl $method $url"
  fi

  curl "${curl_args[@]}"
}
