#!/usr/bin/env bash
# lib/grafana.sh — Grafana OAuth credential helpers
# Source this file; do not execute directly.
# Requires: lib/common.sh and lib/authentik.sh sourced first.

setup_grafana() {
  log_step "Setting up Grafana OAuth credentials"

  require_env AUTHENTIK_TOKEN || return 1
  require_env DOMAIN || return 1

  local client_id client_secret existing_id existing_secret

  log_info "Fetching Grafana provider credentials from Authentik..."
  client_id=$(ak_get_provider "grafana" "client_id")
  client_secret=$(ak_get_provider "grafana" "client_secret")

  if [[ -z "$client_id" || -z "$client_secret" ]]; then
    log_error "Grafana provider not found in Authentik."
    log_error "Create it in Authentik Admin → Applications → Providers → OAuth2/OIDC"
    log_error "  Name: grafana"
    log_error "  Redirect URI: ${GRAFANA_URL}/login/generic_oauth"
    return 1
  fi

  log_info "Provider found — client_id: ${client_id:0:8}..."

  existing_id=$(read_env_var "$HOMELAB_ENV" "GF_AUTH_GENERIC_OAUTH_CLIENT_ID")
  existing_secret=$(read_env_var "$HOMELAB_ENV" "GF_AUTH_GENERIC_OAUTH_CLIENT_SECRET")

  if [[ "$existing_id" == "$client_id" && "$existing_secret" == "$client_secret" && "$FORCE" != "true" ]]; then
    log_success "Grafana OAuth credentials already in .env. No changes needed."
    return 0
  fi

  if [[ -n "$existing_id" && "$existing_id" != "$client_id" ]]; then
    log_warn "Different client_id in .env (${existing_id:0:8}...). Overwriting..."
  fi

  log_info "Writing Grafana OAuth credentials to ${HOMELAB_ENV}..."
  upsert_env_var "$HOMELAB_ENV" "GF_AUTH_GENERIC_OAUTH_CLIENT_ID" "$client_id"
  upsert_env_var "$HOMELAB_ENV" "GF_AUTH_GENERIC_OAUTH_CLIENT_SECRET" "$client_secret"

  log_success "Grafana OAuth credentials written to .env"
  log_info "Restart Grafana to pick up the new credentials: docker compose restart grafana"
}
