#!/usr/bin/env bash
# lib/portainer.sh — Portainer OAuth helpers
# Source this file; do not execute directly.
# Requires: lib/common.sh and lib/authentik.sh sourced first.

PORTAINER_REDIRECT_URI="https://docker.${DOMAIN}/"

# ── Portainer API auth ─────────────────────────────────────────────────────

portainer_get_jwt() {
  require_env PORTAINER_ADMIN_PASS || return 1

  local jwt
  jwt=$(curl_api POST "${PORTAINER_URL}/api/auth" \
    -H "Content-Type: application/json" \
    -d "{\"Username\":\"admin\",\"Password\":\"${PORTAINER_ADMIN_PASS}\"}" \
  ) || {
    log_error "Cannot authenticate with Portainer at ${PORTAINER_URL}. Check PORTAINER_ADMIN_PASS."
    return 1
  }

  echo "$jwt" | jq -r '.jwt'
}

# ── Current settings check ─────────────────────────────────────────────────

portainer_current_oauth_client() {
  local jwt="$1"
  curl_api GET "${PORTAINER_URL}/api/settings" \
    -H "Authorization: Bearer ${jwt}" \
    | jq -r '.OAuthSettings.ClientID // empty' 2>/dev/null || true
}

# ── Apply OAuth settings ───────────────────────────────────────────────────

portainer_apply_oauth() {
  local jwt="$1" client_id="$2" client_secret="$3"

  run_cmd curl_api PUT "${PORTAINER_URL}/api/settings" \
    -H "Authorization: Bearer ${jwt}" \
    -H "Content-Type: application/json" \
    -d "{
      \"AuthenticationMethod\": 3,
      \"OAuthSettings\": {
        \"ClientID\":             \"${client_id}\",
        \"ClientSecret\":         \"${client_secret}\",
        \"AuthorizationURI\":     \"${AUTHENTIK_URL}/application/o/authorize/\",
        \"AccessTokenURI\":       \"${AUTHENTIK_URL}/application/o/token/\",
        \"ResourceURI\":          \"${AUTHENTIK_URL}/application/o/userinfo/\",
        \"RedirectURI\":          \"${PORTAINER_REDIRECT_URI}\",
        \"UserIdentifier\":       \"preferred_username\",
        \"Scopes\":               \"openid email profile\",
        \"OAuthAutoCreateUsers\": true
      }
    }"
}

# ── Orchestrator ───────────────────────────────────────────────────────────

setup_portainer() {
  log_step "Setting up Portainer OAuth via Authentik"

  require_env AUTHENTIK_TOKEN || return 1
  require_env PORTAINER_ADMIN_PASS || return 1

  local client_id client_secret jwt current_client

  log_info "Fetching Portainer provider credentials from Authentik..."
  client_id=$(ak_get_provider "portainer" "client_id")
  client_secret=$(ak_get_provider "portainer" "client_secret")

  if [[ -z "$client_id" || -z "$client_secret" ]]; then
    log_error "Portainer provider not found in Authentik."
    log_error "Create it in Authentik Admin → Applications → Providers → OAuth2/OIDC"
    log_error "  Name: portainer"
    log_error "  Redirect URI: ${PORTAINER_REDIRECT_URI}"
    return 1
  fi

  log_info "Provider found — client_id: ${client_id:0:8}..."

  log_info "Authenticating with Portainer API..."
  jwt=$(portainer_get_jwt) || return 1

  current_client=$(portainer_current_oauth_client "$jwt")
  if [[ "$current_client" == "$client_id" && "$FORCE" != "true" ]]; then
    log_success "Portainer OAuth already configured with correct client_id. No changes needed."
    return 0
  fi

  if [[ -n "$current_client" && "$current_client" != "$client_id" ]]; then
    log_warn "Portainer has a different OAuth client_id (${current_client:0:8}...). Overwriting..."
  fi

  portainer_apply_oauth "$jwt" "$client_id" "$client_secret" > /dev/null
  log_success "Portainer OAuth configured"
  log_info "Redirect URI: ${PORTAINER_REDIRECT_URI}"
}
