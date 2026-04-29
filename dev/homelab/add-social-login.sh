#!/usr/bin/env bash
# add-social-login.sh — Add GitHub and/or Google as social login sources in Authentik
#
# Prerequisites:
#   1. Create an Authentik API token:
#      Authentik Admin UI → System → Tokens → Create (intent: API)
#   2. Create OAuth apps with these callback URLs:
#      GitHub: https://auth.caboose-ai.io/source/oauth/callback/github/
#      Google: https://auth.caboose-ai.io/source/oauth/callback/google/
#
# Usage:
#   export AUTHENTIK_TOKEN=<token>
#   export GITHUB_OAUTH_CLIENT_ID=<id>    GITHUB_OAUTH_CLIENT_SECRET=<secret>
#   export GOOGLE_OAUTH_CLIENT_ID=<id>    GOOGLE_OAUTH_CLIENT_SECRET=<secret>
#   bash add-social-login.sh
#
# Either provider can be omitted by leaving its _CLIENT_ID unset.
set -euo pipefail

AUTHENTIK_URL="${AUTHENTIK_URL:-https://auth.caboose-ai.io}"
AUTHENTIK_TOKEN="${AUTHENTIK_TOKEN:?Set AUTHENTIK_TOKEN to an Authentik API token}"

GITHUB_OAUTH_CLIENT_ID="${GITHUB_OAUTH_CLIENT_ID:-}"
GITHUB_OAUTH_CLIENT_SECRET="${GITHUB_OAUTH_CLIENT_SECRET:-}"
GOOGLE_OAUTH_CLIENT_ID="${GOOGLE_OAUTH_CLIENT_ID:-}"
GOOGLE_OAUTH_CLIENT_SECRET="${GOOGLE_OAUTH_CLIENT_SECRET:-}"

# ── Helpers ───────────────────────────────────────────────────────────────────

ak_get() {
  curl -sf -H "Authorization: Bearer $AUTHENTIK_TOKEN" "$AUTHENTIK_URL$1"
}

ak_post() {
  curl -sf -X POST \
    -H "Authorization: Bearer $AUTHENTIK_TOKEN" \
    -H "Content-Type: application/json" \
    -d "$2" \
    "$AUTHENTIK_URL$1"
}

ak_patch() {
  curl -sf -X PATCH \
    -H "Authorization: Bearer $AUTHENTIK_TOKEN" \
    -H "Content-Type: application/json" \
    -d "$2" \
    "$AUTHENTIK_URL$1"
}

get_source_pk() {
  ak_get "/api/v3/sources/oauth/?slug=$1" \
    | jq -r '.results[0].pk // empty' 2>/dev/null || true
}

upsert_oauth_source() {
  local provider_type="$1" name="$2" slug="$3" client_id="$4" client_secret="$5"

  [[ -z "$client_id" ]] && { echo "  [skip] $name — CLIENT_ID not set"; return; }
  [[ -z "$client_secret" ]] && { echo "  [skip] $name — CLIENT_SECRET not set"; return; }

  local body
  body=$(jq -n \
    --arg t "$provider_type" --arg n "$name" --arg s "$slug" \
    --arg k "$client_id"     --arg v "$client_secret" \
    '{name:$n, slug:$s, enabled:true, provider_type:$t, consumer_key:$k, consumer_secret:$v, additional_scopes:""}')

  local pk
  pk=$(get_source_pk "$slug")

  if [[ -n "$pk" ]]; then
    ak_patch "/api/v3/sources/oauth/$pk/" "$body" | jq -r '"  [updated] " + .name + " (pk=" + (.pk|tostring) + ")"'
  else
    ak_post "/api/v3/sources/oauth/" "$body" | jq -r '"  [created] " + .name + " (pk=" + (.pk|tostring) + ")"'
  fi
}

# ── Step 0: Verify token ──────────────────────────────────────────────────────

echo "==> Verifying Authentik API token..."
ak_get "/api/v3/root/config/" > /dev/null || { echo "ERROR: Cannot reach Authentik API — check AUTHENTIK_TOKEN and AUTHENTIK_URL"; exit 1; }
echo "   ✓ Token OK"

# ── Step 1: Upsert sources ────────────────────────────────────────────────────

echo "==> Upserting OAuth sources..."
upsert_oauth_source "github" "GitHub" "github" "$GITHUB_OAUTH_CLIENT_ID" "$GITHUB_OAUTH_CLIENT_SECRET"
upsert_oauth_source "google" "Google" "google" "$GOOGLE_OAUTH_CLIENT_ID" "$GOOGLE_OAUTH_CLIENT_SECRET"

# ── Done ──────────────────────────────────────────────────────────────────────

echo ""
echo "Done. Configured sources will appear as login buttons on:"
echo "  https://auth.caboose-ai.io"
echo ""
echo "Callback URLs (for reference when creating OAuth apps):"
echo "  GitHub : $AUTHENTIK_URL/source/oauth/callback/github/"
echo "  Google : $AUTHENTIK_URL/source/oauth/callback/google/"
echo ""
echo "Optional next step: restrict which emails can enroll via"
echo "  Authentik Admin → Flows → default-source-enrollment → Bindings → add email Policy."
