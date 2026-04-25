#!/usr/bin/env bats
# Integration tests for finish-sso.sh
# Run with real services active. Credential-dependent tests are auto-skipped when vars are unset.

HOMELAB_ENV="${HOMELAB_ENV:-/opt/homelab/.env}"
HOMELAB_COMPOSE="${HOMELAB_COMPOSE:-/opt/homelab/docker-compose.yml}"
AUTHENTIK_URL="${AUTHENTIK_URL:-https://auth.caboose-ai.io}"
PORTAINER_URL="${PORTAINER_URL:-http://127.0.0.1:9000}"
FORGEJO_CONTAINER="${FORGEJO_CONTAINER:-forgejo}"

# ── Helpers ──────────────────────────────────────────────────────────────────

skip_unless_env() {
  local var="$1"
  # Check exported env var first, then fall back to reading from HOMELAB_ENV file
  local val="${!var:-}"
  if [[ -z "$val" ]]; then
    val=$(grep "^${var}=" "$HOMELAB_ENV" 2>/dev/null | cut -d= -f2- || true)
  fi
  if [[ -z "$val" ]]; then
    skip "requires $var to be set"
  fi
}

portainer_token() {
  curl -sf -X POST "$PORTAINER_URL/api/auth" \
    -H "Content-Type: application/json" \
    -d "{\"Username\":\"admin\",\"Password\":\"${PORTAINER_ADMIN_PASS}\"}" \
    | jq -r '.jwt'
}

forgejo_api() {
  local forgejo_ip path method="${1:-GET}" body="$2"
  forgejo_ip=$(docker inspect "$FORGEJO_CONTAINER" \
    --format='{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' | head -1)
  path="$1"; method="${2:-GET}"; body="${3:-}"
  local gitea_pass
  gitea_pass=$(grep "^GITEA_ADMIN_PASSWORD=" "$HOMELAB_ENV" | cut -d= -f2-)
  curl -sf "http://$forgejo_ip:3000/api/v1$path" \
    -u "caboose:$gitea_pass" \
    -H "Content-Type: application/json" ${body:+-d "$body"}
}

authentik_api() {
  local path="$1"
  curl -sf "$AUTHENTIK_URL/api/v3$path" \
    -H "Authorization: Bearer ${AUTHENTIK_TOKEN}"
}

# ── Container health ──────────────────────────────────────────────────────────

@test "container: woodpecker-server is running" {
  run docker inspect --format='{{.State.Running}}' woodpecker-server
  [ "$status" -eq 0 ]
  [ "$output" = "true" ]
}

@test "container: woodpecker-agent is running" {
  run docker inspect --format='{{.State.Running}}' woodpecker-agent
  [ "$status" -eq 0 ]
  [ "$output" = "true" ]
}

@test "container: forgejo is running" {
  run docker inspect --format='{{.State.Running}}' "$FORGEJO_CONTAINER"
  [ "$status" -eq 0 ]
  [ "$output" = "true" ]
}

@test "container: portainer is running" {
  run docker inspect --format='{{.State.Running}}' portainer
  [ "$status" -eq 0 ]
  [ "$output" = "true" ]
}

@test "container: authentik-server is healthy" {
  run docker inspect --format='{{.State.Health.Status}}' authentik-server
  [ "$status" -eq 0 ]
  [ "$output" = "healthy" ]
}

# ── Woodpecker Forgejo OAuth ──────────────────────────────────────────────────

@test "woodpecker: container has WOODPECKER_GITEA env var enabled" {
  run docker inspect woodpecker-server \
    --format='{{range .Config.Env}}{{println .}}{{end}}'
  [ "$status" -eq 0 ]
  [[ "$output" == *"WOODPECKER_GITEA=true"* ]]
}

@test "woodpecker: container has WOODPECKER_GITEA_URL set" {
  run docker inspect woodpecker-server \
    --format='{{range .Config.Env}}{{println .}}{{end}}'
  [ "$status" -eq 0 ]
  [[ "$output" == *"WOODPECKER_GITEA_URL="* ]]
}

@test "woodpecker: container has WOODPECKER_GITEA_CLIENT configured" {
  run docker inspect woodpecker-server \
    --format='{{range .Config.Env}}{{println .}}{{end}}'
  [ "$status" -eq 0 ]
  [[ "$output" == *"WOODPECKER_GITEA_CLIENT="* ]]
}

@test "woodpecker: WOODPECKER_GITEA_CLIENT is in homelab .env" {
  run grep "^WOODPECKER_GITEA_CLIENT=" "$HOMELAB_ENV"
  [ "$status" -eq 0 ]
  [[ -n "$output" ]]
}

@test "woodpecker: WOODPECKER_GITEA_SECRET is in homelab .env" {
  run grep "^WOODPECKER_GITEA_SECRET=" "$HOMELAB_ENV"
  [ "$status" -eq 0 ]
  [[ -n "$output" ]]
}

# ── Forgejo OAuth apps ────────────────────────────────────────────────────────

skip_if_forgejo_locked() {
  # Must be called directly in test body (not in subshell) for skip to work
  local forgejo_ip gitea_pass http_code
  forgejo_ip=$(docker inspect "$FORGEJO_CONTAINER" \
    --format='{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' | head -1)
  gitea_pass=$(grep "^GITEA_ADMIN_PASSWORD=" "$HOMELAB_ENV" | cut -d= -f2-)
  http_code=$(curl -o /dev/null -w "%{http_code}" -s \
    "http://$forgejo_ip:3000/api/v1/user/applications/oauth2" \
    -u "caboose:$gitea_pass" 2>/dev/null || echo "0")
  if [[ "$http_code" == "403" ]]; then
    skip "Forgejo user has forced-password-change flag — run: docker exec -u git forgejo gitea admin user change-password --username caboose"
  fi
}

forgejo_oauth_apps() {
  local forgejo_ip gitea_pass
  forgejo_ip=$(docker inspect "$FORGEJO_CONTAINER" \
    --format='{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' | head -1)
  gitea_pass=$(grep "^GITEA_ADMIN_PASSWORD=" "$HOMELAB_ENV" | cut -d= -f2-)
  curl -sf "http://$forgejo_ip:3000/api/v1/user/applications/oauth2" \
    -u "caboose:$gitea_pass"
}

@test "forgejo: Woodpecker CI OAuth app exists" {
  skip_unless_env GITEA_ADMIN_PASSWORD
  skip_if_forgejo_locked
  result=$(forgejo_oauth_apps)
  woodpecker_app=$(echo "$result" | jq -r '.[] | select(.name == "Woodpecker CI") | .name' 2>/dev/null)
  [ "$woodpecker_app" = "Woodpecker CI" ]
}

@test "forgejo: Woodpecker CI OAuth app has correct redirect_uri" {
  skip_unless_env GITEA_ADMIN_PASSWORD
  skip_if_forgejo_locked
  result=$(forgejo_oauth_apps)
  redirect=$(echo "$result" | jq -r '.[] | select(.name == "Woodpecker CI") | .redirect_uris[0]' 2>/dev/null)
  [ "$redirect" = "https://ci.caboose-ai.io/authorize" ]
}

# ── Portainer OAuth ───────────────────────────────────────────────────────────

@test "portainer: health endpoint responds" {
  run curl -sf -o /dev/null -w "%{http_code}" "$PORTAINER_URL/api/system/status"
  [ "$status" -eq 0 ]
  [ "$output" = "200" ]
}

@test "portainer: OAuth is configured (AuthenticationMethod=3)" {
  skip_unless_env PORTAINER_ADMIN_PASS
  token=$(portainer_token)
  [ -n "$token" ] && [ "$token" != "null" ] || skip "failed to get Portainer token"
  auth_method=$(curl -sf "$PORTAINER_URL/api/settings" \
    -H "Authorization: Bearer $token" | jq '.AuthenticationMethod')
  [ "$auth_method" = "3" ]
}

@test "portainer: OAuth ClientID matches Authentik PORTAINER_OAUTH_CLIENT_ID" {
  skip_unless_env PORTAINER_ADMIN_PASS
  expected_client_id=$(grep "^PORTAINER_OAUTH_CLIENT_ID=" "$HOMELAB_ENV" | cut -d= -f2-)
  [ -n "$expected_client_id" ] || skip "PORTAINER_OAUTH_CLIENT_ID not in .env"
  token=$(portainer_token)
  [ -n "$token" ] && [ "$token" != "null" ] || skip "failed to get Portainer token"
  configured_client=$(curl -sf "$PORTAINER_URL/api/settings" \
    -H "Authorization: Bearer $token" | jq -r '.OAuthSettings.ClientID')
  [ "$configured_client" = "$expected_client_id" ]
}

# ── Authentik providers ───────────────────────────────────────────────────────

@test "authentik: health endpoint is alive (via internal Docker network)" {
  authentik_ip=$(docker inspect authentik-server \
    --format='{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' | head -1)
  run curl -sf -o /dev/null -w "%{http_code}" "http://$authentik_ip:9000/-/health/live/"
  [ "$status" -eq 0 ]
  [ "$output" = "200" ]
}

@test "authentik: portainer OAuth2 provider exists" {
  skip_unless_env AUTHENTIK_TOKEN
  result=$(authentik_api "/providers/oauth2/?search=portainer")
  count=$(echo "$result" | jq '.count')
  [ "$count" -gt 0 ]
}

@test "authentik: forgejo OAuth2 provider exists" {
  skip_unless_env AUTHENTIK_TOKEN
  result=$(authentik_api "/providers/oauth2/?search=forgejo")
  count=$(echo "$result" | jq '.count')
  [ "$count" -gt 0 ]
}

@test "authentik: woodpecker OAuth2 provider exists" {
  skip_unless_env AUTHENTIK_TOKEN
  result=$(authentik_api "/providers/oauth2/?search=woodpecker")
  count=$(echo "$result" | jq '.count')
  [ "$count" -gt 0 ]
}
