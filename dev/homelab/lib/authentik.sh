#!/usr/bin/env bash
# lib/authentik.sh — Authentik API helpers
# Source this file; do not execute directly.
# Requires: lib/common.sh sourced first.

# ── API wrappers ────────────────────────────────────────────────────────────

ak_get() {
  curl_api GET "${AUTHENTIK_URL}${1}" \
    -H "Authorization: Bearer ${AUTHENTIK_TOKEN}"
}

ak_post() {
  curl_api POST "${AUTHENTIK_URL}${1}" \
    -H "Authorization: Bearer ${AUTHENTIK_TOKEN}" \
    -H "Content-Type: application/json" \
    -d "$2"
}

ak_patch() {
  curl_api PATCH "${AUTHENTIK_URL}${1}" \
    -H "Authorization: Bearer ${AUTHENTIK_TOKEN}" \
    -H "Content-Type: application/json" \
    -d "$2"
}

# ── Provider helpers ────────────────────────────────────────────────────────

ak_get_provider() {
  local name="$1" field="$2"
  require_env AUTHENTIK_TOKEN || return 1

  local slug
  slug=$(echo "$name" | tr '[:upper:]' '[:lower:]' | tr ' ' '-')

  ak_get "/api/v3/providers/oauth2/?search=${slug}" \
    | jq -r ".results[] | select(.name | ascii_downcase | contains(\"${slug}\")) | .${field}" \
    | head -1
}

ak_list_providers() {
  require_env AUTHENTIK_TOKEN || return 1
  ak_get "/api/v3/providers/oauth2/?page_size=50" \
    | jq -r '.results[] | "\(.name)\t\(.client_id)"'
}

# ── Source helpers ──────────────────────────────────────────────────────────

ak_get_source_pk() {
  local slug="$1"
  require_env AUTHENTIK_TOKEN || return 1
  ak_get "/api/v3/sources/oauth/?slug=${slug}" \
    | jq -r '.results[0].pk // empty' 2>/dev/null || true
}

# ── Health check ────────────────────────────────────────────────────────────

ak_health_check() {
  require_env AUTHENTIK_TOKEN || return 1

  log_step "Checking Authentik health at ${AUTHENTIK_URL}..."

  local response
  response=$(ak_get "/api/v3/root/config/" 2>/dev/null) || {
    log_error "Cannot reach Authentik API at ${AUTHENTIK_URL}. Check AUTHENTIK_TOKEN and network connectivity."
    return 1
  }

  local version
  version=$(echo "$response" | jq -r '.version_current // "unknown"')
  log_success "Authentik is healthy (version: ${version})"

  log_info "Configured OAuth2 providers:"
  local providers
  providers=$(ak_list_providers 2>/dev/null) || true
  if [[ -z "$providers" ]]; then
    log_warn "  No OAuth2 providers found. Create them in Authentik Admin → Applications → Providers."
  else
    while IFS=$'\t' read -r pname pid; do
      log_info "  - ${pname} (${pid})"
    done <<< "$providers"
  fi
}
