#!/usr/bin/env bash
# lib/forgejo.sh — Forgejo OIDC helpers
# Source this file; do not execute directly.
# Requires: lib/common.sh and lib/authentik.sh sourced first.

# ── Internal connectivity ──────────────────────────────────────────────────

forgejo_internal_url() {
  local ip
  ip=$(docker inspect "$FORGEJO_CONTAINER" \
    --format='{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' 2>/dev/null) || {
    log_error "Cannot inspect container '$FORGEJO_CONTAINER'. Is it running?"
    return 1
  }
  [[ -z "$ip" ]] && {
    log_error "Container '$FORGEJO_CONTAINER' has no IP address."
    return 1
  }
  echo "http://${ip}:3000"
}

forgejo_admin_password() {
  local pass
  pass=$(secret_get "GITEA_ADMIN_PASSWORD")
  [[ -z "$pass" ]] && {
    log_error "GITEA_ADMIN_PASSWORD not found (checked fnox and $HOMELAB_ENV)"
    return 1
  }
  echo "$pass"
}

# ── Auth source management ─────────────────────────────────────────────────

forgejo_auth_source_id() {
  local name="${1:-Authentik}"
  docker exec "$FORGEJO_CONTAINER" \
    gitea admin auth list 2>/dev/null \
    | awk -v name="$name" '$0 ~ name {print $1}' \
    | head -1
}

forgejo_delete_auth_source() {
  local source_id="$1"
  run_cmd docker exec "$FORGEJO_CONTAINER" \
    gitea admin auth delete --id "$source_id"
}

forgejo_add_oidc() {
  local client_id="$1" client_secret="$2" source_name="${3:-Authentik}"
  local discovery_url="${AUTHENTIK_URL}/application/o/forgejo/.well-known/openid-configuration"

  run_cmd docker exec "$FORGEJO_CONTAINER" gitea admin auth add-oauth \
    --name              "$source_name" \
    --provider          "openidConnect" \
    --key               "$client_id" \
    --secret            "$client_secret" \
    --auto-discover-url "$discovery_url"
}

forgejo_update_oidc() {
  local source_id="$1" client_id="$2" client_secret="$3"
  local discovery_url="${AUTHENTIK_URL}/application/o/forgejo/.well-known/openid-configuration"

  run_cmd docker exec "$FORGEJO_CONTAINER" gitea admin auth update-oauth \
    --id                "$source_id" \
    --key               "$client_id" \
    --secret            "$client_secret" \
    --auto-discover-url "$discovery_url"
}

# ── Orchestrator ───────────────────────────────────────────────────────────

setup_forgejo() {
  log_step "Setting up Forgejo OIDC via Authentik"

  require_env AUTHENTIK_TOKEN || return 1
  require_env DOMAIN || return 1

  local client_id client_secret existing_id

  log_info "Fetching Forgejo provider credentials from Authentik..."
  client_id=$(ak_get_provider "forgejo" "client_id")
  client_secret=$(ak_get_provider "forgejo" "client_secret")

  if [[ -z "$client_id" || -z "$client_secret" ]]; then
    log_error "Forgejo provider not found in Authentik."
    log_error "Create it in Authentik Admin → Applications → Providers → OAuth2/OIDC"
    log_error "  Name: forgejo"
    log_error "  Redirect URI: ${FORGEJO_EXTERNAL_URL}/user/oauth2/Authentik/callback"
    return 1
  fi

  log_info "Provider found — client_id: ${client_id:0:8}..."

  existing_id=$(forgejo_auth_source_id "Authentik")

  if [[ -n "$existing_id" ]]; then
    if [[ "$FORCE" == "true" ]]; then
      log_warn "Auth source exists (ID $existing_id). --force: deleting and re-creating..."
      forgejo_delete_auth_source "$existing_id"
      forgejo_add_oidc "$client_id" "$client_secret"
      log_success "Forgejo OIDC re-created (--force)"
    else
      log_info "Auth source exists (ID $existing_id). Updating in place..."
      forgejo_update_oidc "$existing_id" "$client_id" "$client_secret"
      log_success "Forgejo OIDC updated"
    fi
  else
    log_info "No existing auth source. Creating..."
    forgejo_add_oidc "$client_id" "$client_secret"
    log_success "Forgejo OIDC created"
  fi

  log_info "Users can log in at: ${FORGEJO_EXTERNAL_URL} → 'Sign in with Authentik'"
}
