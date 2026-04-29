#!/usr/bin/env bats
# Integration tests for add-social-login.sh
# Skips credential-dependent assertions when vars are unset.

AUTHENTIK_URL="${AUTHENTIK_URL:-https://auth.caboose-ai.io}"
HOMELAB_ENV="${HOMELAB_ENV:-/opt/homelab/.env}"

skip_unless_env() {
  local var="$1"
  local val="${!var:-}"
  if [[ -z "$val" ]]; then
    val=$(grep "^${var}=" "$HOMELAB_ENV" 2>/dev/null | cut -d= -f2- || true)
  fi
  [[ -z "$val" ]] && skip "requires $var to be set"
}

ak_sources() {
  local token="${AUTHENTIK_TOKEN:-$(grep '^AUTHENTIK_TOKEN=' "$HOMELAB_ENV" 2>/dev/null | cut -d= -f2-)}"
  curl -sf "$AUTHENTIK_URL/api/v3/sources/oauth/" \
    -H "Authorization: Bearer $token"
}

# ── API reachability ──────────────────────────────────────────────────────────

@test "authentik: /api/v3/sources/oauth/ endpoint responds" {
  skip_unless_env AUTHENTIK_TOKEN
  run bash -c 'curl -sf -o /dev/null -w "%{http_code}" \
    -H "Authorization: Bearer '"${AUTHENTIK_TOKEN}"'" \
    "'"$AUTHENTIK_URL"'/api/v3/sources/oauth/"'
  [ "$status" -eq 0 ]
  [ "$output" = "200" ]
}

# ── GitHub source ─────────────────────────────────────────────────────────────

@test "authentik: github OAuth source exists" {
  skip_unless_env AUTHENTIK_TOKEN
  skip_unless_env GITHUB_OAUTH_CLIENT_ID
  result=$(ak_sources)
  slug=$(echo "$result" | jq -r '.results[] | select(.slug == "github") | .slug')
  [ "$slug" = "github" ]
}

@test "authentik: github OAuth source is enabled" {
  skip_unless_env AUTHENTIK_TOKEN
  skip_unless_env GITHUB_OAUTH_CLIENT_ID
  result=$(ak_sources)
  enabled=$(echo "$result" | jq -r '.results[] | select(.slug == "github") | .enabled')
  [ "$enabled" = "true" ]
}

@test "authentik: github OAuth source has correct callback URL format" {
  skip_unless_env AUTHENTIK_TOKEN
  skip_unless_env GITHUB_OAUTH_CLIENT_ID
  result=$(ak_sources)
  callback=$(echo "$result" | jq -r '.results[] | select(.slug == "github") | .callback_url // ""')
  # callback_url is computed by Authentik; if empty, check provider_type
  if [[ -z "$callback" ]]; then
    ptype=$(echo "$result" | jq -r '.results[] | select(.slug == "github") | .provider_type')
    [ "$ptype" = "github" ]
  else
    [[ "$callback" == *"/source/oauth/callback/github/"* ]]
  fi
}

# ── Google source ─────────────────────────────────────────────────────────────

@test "authentik: google OAuth source exists" {
  skip_unless_env AUTHENTIK_TOKEN
  skip_unless_env GOOGLE_OAUTH_CLIENT_ID
  result=$(ak_sources)
  slug=$(echo "$result" | jq -r '.results[] | select(.slug == "google") | .slug')
  [ "$slug" = "google" ]
}

@test "authentik: google OAuth source is enabled" {
  skip_unless_env AUTHENTIK_TOKEN
  skip_unless_env GOOGLE_OAUTH_CLIENT_ID
  result=$(ak_sources)
  enabled=$(echo "$result" | jq -r '.results[] | select(.slug == "google") | .enabled')
  [ "$enabled" = "true" ]
}
