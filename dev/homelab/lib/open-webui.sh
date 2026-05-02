#!/usr/bin/env bash
# lib/open-webui.sh — Open WebUI OIDC credential helpers
# Source this file; do not execute directly.
# Requires: lib/common.sh and lib/authentik.sh sourced first.

OPEN_WEBUI_URL="https://chat.${DOMAIN}"

setup_open_webui() {
  log_step "Setting up Open WebUI OIDC credentials"

  require_env AUTHENTIK_TOKEN || return 1
  require_env DOMAIN || return 1

  local client_id client_secret existing_id existing_secret

  log_info "Fetching Open WebUI provider credentials from Authentik..."
  client_id=$(ak_get_provider "open-webui" "client_id")
  client_secret=$(ak_get_provider "open-webui" "client_secret")

  if [[ -z "$client_id" || -z "$client_secret" ]]; then
    log_error "Open WebUI provider not found in Authentik."
    log_error "Create it in Authentik Admin → Applications → Providers → OAuth2/OIDC"
    log_error "  Name: open-webui"
    log_error "  Redirect URI: ${OPEN_WEBUI_URL}/oauth/oidc/callback"
    return 1
  fi

  log_info "Provider found — client_id: ${client_id:0:8}..."

  existing_id=$(read_env_var "$HOMELAB_ENV" "OPEN_WEBUI_OAUTH_CLIENT_ID")
  existing_secret=$(read_env_var "$HOMELAB_ENV" "OPEN_WEBUI_OAUTH_CLIENT_SECRET")

  if [[ "$existing_id" == "$client_id" && "$existing_secret" == "$client_secret" && "$FORCE" != "true" ]]; then
    log_success "Open WebUI OIDC credentials already in .env. No changes needed."
    return 0
  fi

  if [[ -n "$existing_id" && "$existing_id" != "$client_id" ]]; then
    log_warn "Different client_id in .env (${existing_id:0:8}...). Overwriting..."
  fi

  log_info "Writing Open WebUI OIDC credentials to ${HOMELAB_ENV}..."
  upsert_env_var "$HOMELAB_ENV" "OPEN_WEBUI_OAUTH_CLIENT_ID" "$client_id"
  upsert_env_var "$HOMELAB_ENV" "OPEN_WEBUI_OAUTH_CLIENT_SECRET" "$client_secret"

  log_success "Open WebUI OIDC credentials written to .env"
  log_info "Restart Open WebUI to pick up the new credentials: docker compose restart open-webui"
}
