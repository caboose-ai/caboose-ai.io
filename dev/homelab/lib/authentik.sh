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

# ── Token recovery ─────────────────────────────────────────────────────────

ak_recover_bootstrap_token() {
  require_cmd docker || return 1

  local script
  script=$(cat <<'PY'
import json
import traceback
try:
    from authentik.core.models import Group, Token, TokenIntents, User, UserTypes, default_token_key
    u = User.objects.filter(username='auth-admin').first() or User.objects.filter(username='akadmin').first()
    if u is None:
        u = User.objects.create(username='auth-admin', name='authentik Default Admin', type=UserTypes.INTERNAL)
    u.is_active = True
    u.save()
    g, _ = Group.objects.get_or_create(name='authentik Admins', defaults={'is_superuser': True})
    if not g.is_superuser:
        g.is_superuser = True
        g.save(update_fields=['is_superuser'])
    g.users.add(u)
    t, _ = Token.objects.update_or_create(identifier='authentik-bootstrap-token', defaults={'intent': TokenIntents.INTENT_API, 'user': u, 'expiring': False, 'key': default_token_key()})
    print(json.dumps({'token': t.key}))
except Exception as exc:
    print(json.dumps({'error': ''.join(traceback.format_exception_only(type(exc), exc)).strip()}))
PY
)

  local output
  output=$(docker exec authentik-server sh -c 'PATH=/ak-root/.venv/bin:$PATH /lifecycle/ak shell -c "$1"' homelab-recover-token "$script" 2>&1) || return 1

  local raw_result token error
  raw_result=$(printf "%s\n" "$output" | tail -1)
  error=$(printf "%s" "$raw_result" | jq -r '.error // empty' 2>/dev/null) || return 1
  if [[ -n "$error" ]]; then
    log_error "Authentik token recovery failed: $error"
    return 1
  fi
  token=$(printf "%s" "$raw_result" | jq -r '.token // empty' 2>/dev/null) || return 1
  if [[ -z "$token" ]]; then
    log_error "Authentik token recovery failed: empty token"
    return 1
  fi

  export AUTHENTIK_TOKEN="$token"
  secret_put AUTHENTIK_BOOTSTRAP_TOKEN "$token"
}

# ── Health check ────────────────────────────────────────────────────────────

ak_health_check() {
  require_env AUTHENTIK_TOKEN || return 1

  log_step "Checking Authentik health at ${AUTHENTIK_URL}..."

  local response
  response=$(ak_get "/api/v3/root/config/" 2>/dev/null) || {
    log_warn "Authentik API rejected the configured token; attempting bootstrap token recovery"
    if ak_recover_bootstrap_token; then
      log_success "Recovered Authentik bootstrap token"
      response=$(ak_get "/api/v3/root/config/" 2>/dev/null) || {
        log_error "Cannot reach Authentik API at ${AUTHENTIK_URL} after token recovery. Check network connectivity."
        return 1
      }
    else
      log_error "Cannot reach Authentik API at ${AUTHENTIK_URL}. Check AUTHENTIK_TOKEN and network connectivity."
      return 1
    fi
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
