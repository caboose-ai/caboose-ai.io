#!/usr/bin/env bash
# configure-sso.sh — Wire up Portainer and Forgejo OAuth against Authentik
# Requires: AUTHENTIK_TOKEN (API token from Authentik Admin → System → Tokens)
#           PORTAINER_ADMIN_PASSWORD
# Usage: bash configure-sso.sh
set -euo pipefail

AUTHENTIK_URL="https://auth.caboose-ai.io"
PORTAINER_URL="http://127.0.0.1:9000"
FORGEJO_CONTAINER="forgejo"

AUTHENTIK_TOKEN="${AUTHENTIK_TOKEN:?export AUTHENTIK_TOKEN=<your-authentik-api-token>}"
PORTAINER_ADMIN_PASS="${PORTAINER_ADMIN_PASSWORD:-${PORTAINER_ADMIN_PASS:-}}"
: "${PORTAINER_ADMIN_PASS:?export PORTAINER_ADMIN_PASSWORD=<your-portainer-password>}"
export PORTAINER_ADMIN_PASS

# ── Helper: fetch OAuth2 provider credentials from Authentik by name ──────────
get_provider() {
  local name="$1"
  local field="$2"
  curl -sf "$AUTHENTIK_URL/api/v3/providers/oauth2/?search=$(python3 -c "import urllib.parse; print(urllib.parse.quote('$name'))")" \
    -H "Authorization: Bearer $AUTHENTIK_TOKEN" \
    | jq -r ".results[] | select(.name | ascii_downcase | contains(\"$(echo "$name" | tr '[:upper:]' '[:lower:]')\")) | .$field" \
    | head -1
}

# ── Fetch credentials from Authentik API ─────────────────────────────────────
echo "==> Fetching OAuth2 provider credentials from Authentik..."

PORTAINER_CLIENT_ID=$(get_provider "portainer" "client_id")
PORTAINER_CLIENT_SECRET=$(get_provider "portainer" "client_secret")
FORGEJO_CLIENT_ID=$(get_provider "forgejo" "client_id")
FORGEJO_CLIENT_SECRET=$(get_provider "forgejo" "client_secret")

[[ -z "$PORTAINER_CLIENT_ID" ]]  && { echo "ERROR: Portainer provider not found in Authentik"; exit 1; }
[[ -z "$FORGEJO_CLIENT_ID" ]]    && { echo "ERROR: Forgejo provider not found in Authentik"; exit 1; }

echo "   ✓ Portainer client_id: $PORTAINER_CLIENT_ID"
echo "   ✓ Forgejo  client_id: $FORGEJO_CLIENT_ID"

# ── Portainer OAuth ───────────────────────────────────────────────────────────
echo "==> Configuring Portainer OAuth..."

PORTAINER_AUTH_PAYLOAD=$(jq -cn --arg username "admin" --arg password "$PORTAINER_ADMIN_PASS" '{Username:$username,Password:$password}')
PORTAINER_TOKEN=$(curl -sf -X POST "$PORTAINER_URL/api/auth" \
  -H "Content-Type: application/json" \
  -d "$PORTAINER_AUTH_PAYLOAD" \
  | jq -r '.jwt')

curl -sf -X PUT "$PORTAINER_URL/api/settings" \
  -H "Authorization: Bearer $PORTAINER_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"AuthenticationMethod\": 3,
    \"OAuthSettings\": {
      \"ClientID\":             \"$PORTAINER_CLIENT_ID\",
      \"ClientSecret\":         \"$PORTAINER_CLIENT_SECRET\",
      \"AuthorizationURI\":     \"$AUTHENTIK_URL/application/o/authorize/\",
      \"AccessTokenURI\":       \"$AUTHENTIK_URL/application/o/token/\",
      \"ResourceURI\":          \"$AUTHENTIK_URL/application/o/userinfo/\",
      \"RedirectURI\":          \"https://docker.caboose-ai.io/\",
      \"UserIdentifier\":       \"email\",
      \"Scopes\":               \"openid email profile\",
      \"OAuthAutoCreateUsers\": true,
      \"SSO\":                  true
    }
  }" | jq .

echo "✓ Portainer OAuth configured"

# ── Forgejo OAuth ─────────────────────────────────────────────────────────────
echo "==> Configuring Forgejo OAuth..."

EXISTING_ID=$(docker exec "$FORGEJO_CONTAINER" \
  gitea admin auth list 2>/dev/null | awk '/Authentik/{print $1}' || true)

if [[ -n "$EXISTING_ID" ]]; then
  echo "   Removing existing Authentik source (ID $EXISTING_ID)..."
  docker exec "$FORGEJO_CONTAINER" gitea admin auth delete --id "$EXISTING_ID"
fi

docker exec "$FORGEJO_CONTAINER" gitea admin auth add-oauth \
  --name              "Authentik" \
  --provider          "openidConnect" \
  --key               "$FORGEJO_CLIENT_ID" \
  --secret            "$FORGEJO_CLIENT_SECRET" \
  --auto-discover-url "$AUTHENTIK_URL/application/o/forgejo/.well-known/openid-configuration"

echo "✓ Forgejo OAuth configured"
echo ""
echo "Done. Log out and back in to test SSO on both services."
