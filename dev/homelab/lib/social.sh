#!/usr/bin/env bash
# lib/social.sh — Social login source management (GitHub, Google)
# Source this file; do not execute directly.
# Requires: lib/common.sh and lib/authentik.sh sourced first.

GITHUB_OAUTH_CLIENT_ID="${GITHUB_OAUTH_CLIENT_ID:-}"
GITHUB_OAUTH_CLIENT_SECRET="${GITHUB_OAUTH_CLIENT_SECRET:-}"
GOOGLE_OAUTH_CLIENT_ID="${GOOGLE_OAUTH_CLIENT_ID:-}"
GOOGLE_OAUTH_CLIENT_SECRET="${GOOGLE_OAUTH_CLIENT_SECRET:-}"

upsert_oauth_source() {
  local provider_type="$1" name="$2" slug="$3" client_id="$4" client_secret="$5"

  if [[ -z "$client_id" || -z "$client_secret" ]]; then
    log_info "  [skip] ${name} — credentials not set"
    return 0
  fi

  local body
  body=$(jq -n \
    --arg t "$provider_type" --arg n "$name" --arg s "$slug" \
    --arg k "$client_id"     --arg v "$client_secret" \
    '{name:$n, slug:$s, enabled:true, provider_type:$t, consumer_key:$k, consumer_secret:$v, additional_scopes:""}')

  local pk
  pk=$(ak_get_source_pk "$slug")

  if [[ -n "$pk" ]]; then
    log_info "  ${name} source exists (pk=${pk}). Updating..."
    run_cmd ak_patch "/api/v3/sources/oauth/${pk}/" "$body" > /dev/null
    log_success "  ${name} updated"
  else
    log_info "  Creating ${name} source..."
    run_cmd ak_post "/api/v3/sources/oauth/" "$body" > /dev/null
    log_success "  ${name} created"
  fi
}

setup_social() {
  log_step "Setting up social login sources in Authentik"

  require_env AUTHENTIK_TOKEN || return 1
  require_env DOMAIN || return 1

  local any_configured=false

  if [[ -n "$GITHUB_OAUTH_CLIENT_ID" ]]; then
    upsert_oauth_source "github" "GitHub" "github" \
      "$GITHUB_OAUTH_CLIENT_ID" "$GITHUB_OAUTH_CLIENT_SECRET"
    any_configured=true
  else
    log_info "  [skip] GitHub — GITHUB_OAUTH_CLIENT_ID not set"
  fi

  if [[ -n "$GOOGLE_OAUTH_CLIENT_ID" ]]; then
    upsert_oauth_source "google" "Google" "google" \
      "$GOOGLE_OAUTH_CLIENT_ID" "$GOOGLE_OAUTH_CLIENT_SECRET"
    any_configured=true
  else
    log_info "  [skip] Google — GOOGLE_OAUTH_CLIENT_ID not set"
  fi

  if [[ "$any_configured" == "true" ]]; then
    log_success "Social login sources configured"
  else
    log_warn "No social login sources configured (set GITHUB_OAUTH_CLIENT_ID and/or GOOGLE_OAUTH_CLIENT_ID)"
  fi

  log_info "Callback URLs:"
  log_info "  GitHub: ${AUTHENTIK_URL}/source/oauth/callback/github/"
  log_info "  Google: ${AUTHENTIK_URL}/source/oauth/callback/google/"
}
