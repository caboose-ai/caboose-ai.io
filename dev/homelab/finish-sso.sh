#!/usr/bin/env bash
# finish-sso.sh — Complete remaining SSO configuration for homelab
# Requires: AUTHENTIK_TOKEN, PORTAINER_ADMIN_PASSWORD
# Usage: bash finish-sso.sh
set -euo pipefail

AUTHENTIK_URL="https://auth.caboose-ai.io"
PORTAINER_URL="http://127.0.0.1:9000"
FORGEJO_URL="${FORGEJO_URL:-http://127.0.0.1:3000}"
FORGEJO_CONTAINER="forgejo"
HOMELAB_ENV="${HOMELAB_ENV:-/opt/homelab/.env}"
HOMELAB_COMPOSE="${HOMELAB_COMPOSE:-/opt/homelab/docker-compose.yml}"

AUTHENTIK_TOKEN="${AUTHENTIK_TOKEN:?export AUTHENTIK_TOKEN=<api-token>}"
PORTAINER_ADMIN_PASS="${PORTAINER_ADMIN_PASSWORD:-${PORTAINER_ADMIN_PASS:-}}"
: "${PORTAINER_ADMIN_PASS:?export PORTAINER_ADMIN_PASSWORD=<password>}"
export PORTAINER_ADMIN_PASS

# ── Helpers ───────────────────────────────────────────────────────────────────

upsert_env_var() {
  local file="$1" key="$2" value="$3"
  if grep -q "^${key}=" "$file"; then
    sed -i "s|^${key}=.*|${key}=${value}|" "$file"
  else
    echo "${key}=${value}" >> "$file"
  fi
}

get_forgejo_oauth_app_id() {
  local forgejo_url="$1" auth="$2" app_name="$3"
  curl -sf "$forgejo_url/api/v1/user/applications/oauth2" \
    -u "$auth" \
    | jq -r ".[] | select(.name == \"$app_name\") | .id" 2>/dev/null || true
}

get_authentik_provider() {
  local name="$1" field="$2"
  local encoded_name
  encoded_name=$(python3 -c "import urllib.parse; print(urllib.parse.quote('$name'))")
  curl -sf "$AUTHENTIK_URL/api/v3/providers/oauth2/?search=$encoded_name" \
    -H "Authorization: Bearer $AUTHENTIK_TOKEN" \
    | jq -r ".results[] | select(.name | ascii_downcase | contains(\"$(echo "$name" | tr '[:upper:]' '[:lower:]')\")) | .$field" \
    | head -1
}

forgejo_admin_username() {
  local username="${FORGEJO_ADMIN_USERNAME:-}"
  if [[ -z "$username" ]]; then
    username=$(grep "^FORGEJO_ADMIN_USERNAME=" "$HOMELAB_ENV" 2>/dev/null | cut -d= -f2- || true)
  fi
  echo "${username:-auth-admin}"
}

# ── Step 1: Reset Forgejo password-change flag ────────────────────────────────

step1_reset_forgejo_password() {
  echo "==> Step 1: Resetting Forgejo password-change flag..."
  local gitea_pass forgejo_user
  gitea_pass=$(grep "^GITEA_ADMIN_PASSWORD=" "$HOMELAB_ENV" | cut -d= -f2-)
  [[ -z "$gitea_pass" ]] && { echo "ERROR: GITEA_ADMIN_PASSWORD not in $HOMELAB_ENV"; exit 1; }
  forgejo_user=$(forgejo_admin_username)

  docker exec -u git "$FORGEJO_CONTAINER" gitea admin user change-password \
    --username "$forgejo_user" --password "$gitea_pass" 2>&1
  echo "✓ Forgejo password-change flag cleared"
}

# ── Step 2: Create Woodpecker OAuth2 app in Forgejo ──────────────────────────

WOODPECKER_CLIENT_ID=""
WOODPECKER_CLIENT_SECRET=""

step2_create_woodpecker_oauth_app() {
  echo "==> Step 2: Creating Woodpecker OAuth2 app in Forgejo..."
  local gitea_pass forgejo_user forgejo_url forgejo_auth existing_id result

  gitea_pass=$(grep "^GITEA_ADMIN_PASSWORD=" "$HOMELAB_ENV" | cut -d= -f2-)
  forgejo_user=$(forgejo_admin_username)
  forgejo_url="$FORGEJO_URL"
  forgejo_auth="$forgejo_user:$gitea_pass"

  existing_id=$(get_forgejo_oauth_app_id "$forgejo_url" "$forgejo_auth" "Woodpecker CI")

  if [[ -n "$existing_id" ]]; then
    echo "   Woodpecker CI app already exists (ID: $existing_id)"
    echo "   NOTE: Re-run requires deleting the existing app in Forgejo UI to regenerate secret."
    echo "   Fetching existing client_id..."
    WOODPECKER_CLIENT_ID=$(curl -sf "$forgejo_url/api/v1/user/applications/oauth2" \
      -u "$forgejo_auth" \
      | jq -r ".[] | select(.id == $existing_id) | .client_id")
    echo "   client_id: $WOODPECKER_CLIENT_ID (secret not re-fetchable)"
  else
    result=$(curl -sf -X POST "$forgejo_url/api/v1/user/applications/oauth2" \
      -u "$forgejo_auth" \
      -H "Content-Type: application/json" \
      -d '{"name":"Woodpecker CI","redirect_uris":["https://ci.caboose-ai.io/authorize"]}')
    WOODPECKER_CLIENT_ID=$(echo "$result" | jq -r '.client_id')
    WOODPECKER_CLIENT_SECRET=$(echo "$result" | jq -r '.client_secret')
    echo "   ✓ Woodpecker CI OAuth app created"
    echo "   client_id: $WOODPECKER_CLIENT_ID"
  fi
}

# ── Step 3: Write Woodpecker credentials and restart ─────────────────────────

step3_configure_woodpecker() {
  echo "==> Step 3: Writing Woodpecker credentials to stack..."
  [[ -z "$WOODPECKER_CLIENT_ID" ]]     && { echo "ERROR: WOODPECKER_CLIENT_ID is empty"; exit 1; }
  [[ -z "$WOODPECKER_CLIENT_SECRET" ]] && { echo "ERROR: WOODPECKER_CLIENT_SECRET is empty (app already existed — delete and re-run)"; exit 1; }

  upsert_env_var "$HOMELAB_ENV" "WOODPECKER_GITEA_CLIENT"  "$WOODPECKER_CLIENT_ID"
  upsert_env_var "$HOMELAB_ENV" "WOODPECKER_GITEA_SECRET" "$WOODPECKER_CLIENT_SECRET"
  echo "   ✓ .env updated"

  if ! grep -q "WOODPECKER_GITEA_CLIENT" "$HOMELAB_COMPOSE"; then
    python3 - "$HOMELAB_COMPOSE" <<'PYEOF'
import sys, re
path = sys.argv[1]
content = open(path).read()
insert = ('      WOODPECKER_GITEA_CLIENT: "${WOODPECKER_GITEA_CLIENT}"\n'
          '      WOODPECKER_GITEA_SECRET: "${WOODPECKER_GITEA_SECRET}"\n')
# Insert after the WOODPECKER_GITEA_URL line
content = re.sub(
    r'(      WOODPECKER_GITEA_URL:.*\n)',
    r'\1' + insert,
    content
)
open(path, 'w').write(content)
print("   ✓ docker-compose.yml updated")
PYEOF
  else
    echo "   docker-compose.yml already has Woodpecker Gitea vars"
  fi

  docker compose -f "$HOMELAB_COMPOSE" restart woodpecker-server woodpecker-agent
  echo "✓ Woodpecker restarted with Forgejo OAuth"
}

# ── Step 4: Configure Portainer OAuth ────────────────────────────────────────

step4_configure_portainer() {
  echo "==> Step 4: Configuring Portainer OAuth..."
  local portainer_token client_id client_secret auth_payload

  auth_payload=$(jq -cn --arg username "admin" --arg password "$PORTAINER_ADMIN_PASS" '{Username:$username,Password:$password}')
  portainer_token=$(curl -sf -X POST "$PORTAINER_URL/api/auth" \
    -H "Content-Type: application/json" \
    -d "$auth_payload" \
    | jq -r '.jwt')

  client_id=$(get_authentik_provider "portainer" "client_id")
  client_secret=$(get_authentik_provider "portainer" "client_secret")

  [[ -z "$client_id" ]] && { echo "ERROR: Portainer provider not found in Authentik"; exit 1; }

  curl -sf -X PUT "$PORTAINER_URL/api/settings" \
    -H "Authorization: Bearer $portainer_token" \
    -H "Content-Type: application/json" \
    -d "{
      \"AuthenticationMethod\": 3,
      \"OAuthSettings\": {
        \"ClientID\":             \"$client_id\",
        \"ClientSecret\":         \"$client_secret\",
        \"AuthorizationURI\":     \"$AUTHENTIK_URL/application/o/authorize/\",
        \"AccessTokenURI\":       \"$AUTHENTIK_URL/application/o/token/\",
        \"ResourceURI\":          \"$AUTHENTIK_URL/application/o/userinfo/\",
        \"RedirectURI\":          \"https://docker.caboose-ai.io/\",
        \"UserIdentifier\":       \"email\",
        \"Scopes\":               \"openid email profile\",
        \"OAuthAutoCreateUsers\": true,
        \"SSO\":                  true
      }
    }" | jq . > /dev/null
  echo "✓ Portainer OAuth configured"
}

# ── Main ──────────────────────────────────────────────────────────────────────

if [[ "${FINISH_SSO_LIB_ONLY:-}" != "true" ]]; then
  step1_reset_forgejo_password
  step2_create_woodpecker_oauth_app
  step3_configure_woodpecker
  step4_configure_portainer
  echo ""
  echo "Done. All SSO gaps closed."
fi
