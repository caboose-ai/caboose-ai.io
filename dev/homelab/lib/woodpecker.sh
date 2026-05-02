#!/usr/bin/env bash
# lib/woodpecker.sh — Woodpecker CI OAuth helpers (via Forgejo)
# Source this file; do not execute directly.
# Requires: lib/common.sh, lib/authentik.sh, lib/forgejo.sh sourced first.

WOODPECKER_OAUTH_APP_NAME="Woodpecker CI"
WOODPECKER_REDIRECT="${WOODPECKER_URL}/authorize"

# ── Forgejo OAuth app management ───────────────────────────────────────────

woodpecker_find_oauth_app() {
  local forgejo_url="$1" auth="$2"
  curl_api GET "${forgejo_url}/api/v1/user/applications/oauth2" \
    -u "$auth" \
    | jq -r ".[] | select(.name == \"${WOODPECKER_OAUTH_APP_NAME}\") | .id" 2>/dev/null \
    | head -1
}

woodpecker_get_client_id() {
  local forgejo_url="$1" auth="$2" app_id="$3"
  curl_api GET "${forgejo_url}/api/v1/user/applications/oauth2" \
    -u "$auth" \
    | jq -r ".[] | select(.id == ${app_id}) | .client_id" 2>/dev/null
}

woodpecker_delete_oauth_app() {
  local forgejo_url="$1" auth="$2" app_id="$3"
  run_cmd curl_api DELETE "${forgejo_url}/api/v1/user/applications/oauth2/${app_id}" \
    -u "$auth"
}

woodpecker_create_oauth_app() {
  local forgejo_url="$1" auth="$2"
  run_cmd curl_api POST "${forgejo_url}/api/v1/user/applications/oauth2" \
    -u "$auth" \
    -H "Content-Type: application/json" \
    -d "{\"name\":\"${WOODPECKER_OAUTH_APP_NAME}\",\"redirect_uris\":[\"${WOODPECKER_REDIRECT}\"]}"
}

# ── Orchestrator ───────────────────────────────────────────────────────────

setup_woodpecker() {
  log_step "Setting up Woodpecker CI OAuth via Forgejo"

  require_env DOMAIN || return 1

  local forgejo_url admin_pass auth app_id client_id client_secret result

  log_info "Resolving Forgejo internal address..."
  forgejo_url=$(forgejo_internal_url) || return 1

  admin_pass=$(forgejo_admin_password) || return 1
  auth="${FORGEJO_ADMIN_USERNAME}:${admin_pass}"

  app_id=$(woodpecker_find_oauth_app "$forgejo_url" "$auth")

  if [[ -n "$app_id" ]]; then
    log_info "OAuth app '${WOODPECKER_OAUTH_APP_NAME}' exists (ID: ${app_id})"

    if [[ "$FORCE" == "true" ]]; then
      log_warn "--force: deleting existing app and re-creating..."
      woodpecker_delete_oauth_app "$forgejo_url" "$auth" "$app_id"
    else
      client_id=$(woodpecker_get_client_id "$forgejo_url" "$auth" "$app_id")
      client_secret=$(read_env_var "$HOMELAB_ENV" "WOODPECKER_GITEA_SECRET")

      if [[ -n "$client_secret" ]]; then
        log_info "Reusing existing client_id and secret from .env"
        upsert_env_var "$HOMELAB_ENV" "WOODPECKER_GITEA_CLIENT" "$client_id"
        log_success "Woodpecker OAuth already configured (app ID: ${app_id})"
        return 0
      fi

      log_error "OAuth app exists but WOODPECKER_GITEA_SECRET is not in ${HOMELAB_ENV}."
      log_error "Forgejo only returns the secret at creation time."
      log_error "Re-run with --force to delete and recreate the app."
      return 1
    fi
  fi

  log_info "Creating OAuth app in Forgejo..."
  result=$(woodpecker_create_oauth_app "$forgejo_url" "$auth") || {
    log_error "Failed to create OAuth app in Forgejo."
    return 1
  }

  if [[ "$DRY_RUN" == "true" ]]; then
    log_success "Woodpecker OAuth app would be created (dry-run)"
    return 0
  fi

  client_id=$(echo "$result" | jq -r '.client_id')
  client_secret=$(echo "$result" | jq -r '.client_secret')

  if [[ -z "$client_id" || -z "$client_secret" ]]; then
    log_error "Failed to parse client_id/client_secret from Forgejo response."
    return 1
  fi

  log_info "Writing credentials to ${HOMELAB_ENV}..."
  upsert_env_var "$HOMELAB_ENV" "WOODPECKER_GITEA_CLIENT" "$client_id"
  upsert_env_var "$HOMELAB_ENV" "WOODPECKER_GITEA_SECRET" "$client_secret"

  log_success "Woodpecker OAuth configured"
  log_info "client_id: ${client_id:0:8}..."
  log_info "Restart woodpecker-server to pick up the new credentials."
}
