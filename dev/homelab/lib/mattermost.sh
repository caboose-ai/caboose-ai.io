#!/usr/bin/env bash
# lib/mattermost.sh — Mattermost OIDC helpers (config.json patching)
# Source this file; do not execute directly.
# Requires: lib/common.sh and lib/authentik.sh sourced first.

MATTERMOST_CONTAINER="${MATTERMOST_CONTAINER:-mattermost}"
MATTERMOST_CONFIG="/opt/mattermost/config/config.json"
MATTERMOST_URL="https://mattermost.${DOMAIN}"

setup_mattermost() {
  log_step "Setting up Mattermost OIDC via Authentik"

  require_env AUTHENTIK_TOKEN || return 1
  require_env DOMAIN || return 1

  local client_id client_secret

  if ! docker inspect "$MATTERMOST_CONTAINER" &>/dev/null; then
    log_warn "Container '${MATTERMOST_CONTAINER}' not found. Skipping Mattermost setup."
    log_info "Deploy Mattermost first, then re-run this command."
    return 0
  fi

  log_info "Fetching Mattermost provider credentials from Authentik..."
  client_id=$(ak_get_provider "mattermost" "client_id")
  client_secret=$(ak_get_provider "mattermost" "client_secret")

  if [[ -z "$client_id" || -z "$client_secret" ]]; then
    log_error "Mattermost provider not found in Authentik."
    log_error "Create it in Authentik Admin → Applications → Providers → OAuth2/OIDC"
    log_error "  Name: mattermost"
    log_error "  Redirect URI: ${MATTERMOST_URL}/signup/gitlab/complete"
    return 1
  fi

  log_info "Provider found — client_id: ${client_id:0:8}..."

  if [[ "$FORCE" != "true" ]]; then
    local current_id
    current_id=$(docker exec "$MATTERMOST_CONTAINER" \
      sh -c "jq -r '.GitLabSettings.Id // empty' ${MATTERMOST_CONFIG} 2>/dev/null" 2>/dev/null || true)
    if [[ "$current_id" == "$client_id" ]]; then
      log_success "Mattermost OIDC already configured with correct client_id. No changes needed."
      return 0
    fi
  fi

  log_info "Patching ${MATTERMOST_CONFIG} inside container..."

  local jq_filter
  jq_filter=$(cat <<JQEOF
.GitLabSettings.Enable = true |
.GitLabSettings.Id = "${client_id}" |
.GitLabSettings.Secret = "${client_secret}" |
.GitLabSettings.AuthEndpoint = "${AUTHENTIK_URL}/application/o/authorize/" |
.GitLabSettings.TokenEndpoint = "${AUTHENTIK_URL}/application/o/token/" |
.GitLabSettings.UserAPIEndpoint = "${AUTHENTIK_URL}/application/o/userinfo/" |
.GitLabSettings.Scope = "openid email profile"
JQEOF
  )

  run_cmd docker exec "$MATTERMOST_CONTAINER" \
    sh -c "jq '${jq_filter}' ${MATTERMOST_CONFIG} > /tmp/mm-config.json && mv /tmp/mm-config.json ${MATTERMOST_CONFIG}"

  log_success "Mattermost OIDC configured"
  log_info "Restart Mattermost to apply: docker restart ${MATTERMOST_CONTAINER}"
}
