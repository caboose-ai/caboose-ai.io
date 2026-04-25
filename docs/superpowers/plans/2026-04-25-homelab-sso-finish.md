# Homelab SSO Completion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Write `finish-sso.sh` to complete Forgejo admin reset, Woodpecker CI OAuth, and Portainer OAuth — with unit and integration tests.

**Architecture:** Helper functions are isolated in `finish-sso.sh` behind a `FINISH_SSO_LIB_ONLY` guard so bats tests can source the script without executing main. Tests mock external commands (curl, docker) inline in bats `setup()` blocks. Integration tests call live APIs and assert end-state.

**Tech Stack:** bash, bats (Bash Automated Testing System), curl, jq, docker, python3 (for .env mutation)

---

## File Map

| Action | Path |
|--------|------|
| Create | `dev/homelab/finish-sso.sh` |
| Create | `dev/homelab/tests/run_tests.sh` |
| Create | `dev/homelab/tests/unit/test_helpers.bats` |
| Create | `dev/homelab/tests/unit/test_idempotency.bats` |
| Create | `dev/homelab/tests/integration/test_forgejo.bats` |
| Create | `dev/homelab/tests/integration/test_portainer.bats` |
| Create | `dev/homelab/tests/integration/test_woodpecker.bats` |
| Modify | `/opt/homelab/docker-compose.yml` (add Woodpecker env vars) |
| Modify | `/opt/homelab/.env` (add WOODPECKER_GITEA_CLIENT/SECRET) |

---

## Task 1: Install bats

**Files:**
- No repo files changed

- [ ] **Step 1: Install bats via npm**

```bash
npm install -g bats
```

Expected output: `added 1 package` (or similar). Verify:

```bash
bats --version
```

Expected: `Bats 1.x.x`

- [ ] **Step 2: Commit nothing** — bats is a dev tool dep, not committed.

---

## Task 2: Scaffold test infrastructure

**Files:**
- Create: `dev/homelab/tests/run_tests.sh`

- [ ] **Step 1: Create test runner**

```bash
cat > dev/homelab/tests/run_tests.sh << 'EOF'
#!/usr/bin/env bash
# Usage: ./run_tests.sh [--unit | --integration | --all]
set -euo pipefail

TESTS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MODE="${1:---all}"

run_unit() {
  echo "==> Running unit tests..."
  bats "$TESTS_DIR/unit/"
}

run_integration() {
  echo "==> Running integration tests..."
  bats "$TESTS_DIR/integration/"
}

case "$MODE" in
  --unit)        run_unit ;;
  --integration) run_integration ;;
  --all)         run_unit; run_integration ;;
  *) echo "Usage: $0 [--unit | --integration | --all]"; exit 1 ;;
esac
EOF
chmod +x dev/homelab/tests/run_tests.sh
mkdir -p dev/homelab/tests/unit dev/homelab/tests/integration
```

- [ ] **Step 2: Verify structure**

```bash
ls dev/homelab/tests/
```

Expected: `integration/  run_tests.sh  unit/`

- [ ] **Step 3: Commit**

```bash
git add dev/homelab/tests/
git commit -m "chore(homelab): add bats test runner scaffold"
```

---

## Task 3: Write unit tests for helper functions

**Files:**
- Create: `dev/homelab/tests/unit/test_helpers.bats`

- [ ] **Step 1: Write the tests**

```bash
cat > dev/homelab/tests/unit/test_helpers.bats << 'EOF'
#!/usr/bin/env bats

setup() {
  TEST_ENV=$(mktemp)
  echo "GITEA_ADMIN_PASSWORD=testpass123" > "$TEST_ENV"
  export HOMELAB_ENV="$TEST_ENV"
  export HOMELAB_COMPOSE="/dev/null"
  export AUTHENTIK_TOKEN="dummy"
  export AUTHENTIK_URL="https://fake.example.com"
  export PORTAINER_ADMIN_PASS="dummy"
  export FORGEJO_CONTAINER="forgejo"
  export FINISH_SSO_LIB_ONLY=true
  source "$(dirname "$BATS_TEST_FILENAME")/../../finish-sso.sh"
}

teardown() {
  rm -f "$TEST_ENV"
}

@test "upsert_env_var: adds new key when not present" {
  upsert_env_var "$TEST_ENV" "NEW_KEY" "newvalue"
  run grep "^NEW_KEY=newvalue" "$TEST_ENV"
  [ "$status" -eq 0 ]
}

@test "upsert_env_var: replaces existing key value" {
  echo "EXISTING_KEY=oldvalue" >> "$TEST_ENV"
  upsert_env_var "$TEST_ENV" "EXISTING_KEY" "newvalue"
  run grep "^EXISTING_KEY=newvalue" "$TEST_ENV"
  [ "$status" -eq 0 ]
  run grep "oldvalue" "$TEST_ENV"
  [ "$status" -ne 0 ]
}

@test "upsert_env_var: does not duplicate key" {
  echo "DUP_KEY=value1" >> "$TEST_ENV"
  upsert_env_var "$TEST_ENV" "DUP_KEY" "value2"
  count=$(grep -c "^DUP_KEY=" "$TEST_ENV")
  [ "$count" -eq 1 ]
}

@test "get_forgejo_oauth_app_id: returns empty string when app not found" {
  curl() { echo "[]"; }
  export -f curl
  result=$(get_forgejo_oauth_app_id "http://fake:3000" "user:pass" "Woodpecker CI")
  [ "$result" = "" ]
}

@test "get_forgejo_oauth_app_id: returns id when app exists" {
  curl() { echo '[{"id": 42, "name": "Woodpecker CI", "client_id": "abc"}]'; }
  export -f curl
  result=$(get_forgejo_oauth_app_id "http://fake:3000" "user:pass" "Woodpecker CI")
  [ "$result" = "42" ]
}

@test "get_forgejo_oauth_app_id: returns empty when different app exists" {
  curl() { echo '[{"id": 99, "name": "Other App", "client_id": "xyz"}]'; }
  export -f curl
  result=$(get_forgejo_oauth_app_id "http://fake:3000" "user:pass" "Woodpecker CI")
  [ "$result" = "" ]
}
EOF
```

- [ ] **Step 2: Run tests — expect failures (finish-sso.sh doesn't exist yet)**

```bash
cd dev/homelab && bats tests/unit/test_helpers.bats 2>&1 | head -20
```

Expected: errors about missing `finish-sso.sh` — that's correct, tests fail first.

- [ ] **Step 3: Commit failing tests**

```bash
git add dev/homelab/tests/unit/test_helpers.bats
git commit -m "test(homelab): add unit tests for finish-sso helpers"
```

---

## Task 4: Write unit tests for idempotency

**Files:**
- Create: `dev/homelab/tests/unit/test_idempotency.bats`

- [ ] **Step 1: Write the tests**

```bash
cat > dev/homelab/tests/unit/test_idempotency.bats << 'EOF'
#!/usr/bin/env bats

setup() {
  TEST_ENV=$(mktemp)
  echo "GITEA_ADMIN_PASSWORD=testpass123" > "$TEST_ENV"
  export HOMELAB_ENV="$TEST_ENV"
  export HOMELAB_COMPOSE="/dev/null"
  export AUTHENTIK_TOKEN="dummy"
  export AUTHENTIK_URL="https://fake.example.com"
  export PORTAINER_ADMIN_PASS="dummy"
  export FORGEJO_CONTAINER="forgejo"
  export FINISH_SSO_LIB_ONLY=true
  source "$(dirname "$BATS_TEST_FILENAME")/../../finish-sso.sh"
}

teardown() {
  rm -f "$TEST_ENV"
}

@test "upsert_env_var: calling twice with same value is safe" {
  upsert_env_var "$TEST_ENV" "IDEM_KEY" "value"
  upsert_env_var "$TEST_ENV" "IDEM_KEY" "value"
  count=$(grep -c "^IDEM_KEY=" "$TEST_ENV")
  [ "$count" -eq 1 ]
}

@test "get_forgejo_oauth_app_id: skips creation when app already exists" {
  # When app exists, function returns its id (non-empty) so caller skips POST
  curl() { echo '[{"id": 7, "name": "Woodpecker CI", "client_id": "existing_id"}]'; }
  export -f curl
  result=$(get_forgejo_oauth_app_id "http://fake:3000" "user:pass" "Woodpecker CI")
  [ -n "$result" ]  # non-empty means "exists, skip creation"
}
EOF
```

- [ ] **Step 2: Run — expect failures**

```bash
cd dev/homelab && bats tests/unit/test_idempotency.bats 2>&1 | head -10
```

Expected: failures (script doesn't exist yet).

- [ ] **Step 3: Commit**

```bash
git add dev/homelab/tests/unit/test_idempotency.bats
git commit -m "test(homelab): add idempotency unit tests"
```

---

## Task 5: Implement finish-sso.sh

**Files:**
- Create: `dev/homelab/finish-sso.sh`

- [ ] **Step 1: Write the script**

```bash
cat > dev/homelab/finish-sso.sh << 'SCRIPT'
#!/usr/bin/env bash
# finish-sso.sh — Complete remaining SSO configuration for homelab
# Requires: AUTHENTIK_TOKEN, PORTAINER_ADMIN_PASS
# Usage: bash finish-sso.sh
set -euo pipefail

AUTHENTIK_URL="https://auth.caboose-ai.io"
PORTAINER_URL="http://127.0.0.1:9000"
FORGEJO_CONTAINER="forgejo"
HOMELAB_ENV="${HOMELAB_ENV:-/opt/homelab/.env}"
HOMELAB_COMPOSE="${HOMELAB_COMPOSE:-/opt/homelab/docker-compose.yml}"

AUTHENTIK_TOKEN="${AUTHENTIK_TOKEN:?export AUTHENTIK_TOKEN=<api-token>}"
PORTAINER_ADMIN_PASS="${PORTAINER_ADMIN_PASS:?export PORTAINER_ADMIN_PASS=<password>}"

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

# ── Step 1: Reset Forgejo password-change flag ────────────────────────────────

step1_reset_forgejo_password() {
  echo "==> Step 1: Resetting Forgejo password-change flag..."
  local gitea_pass
  gitea_pass=$(grep "^GITEA_ADMIN_PASSWORD=" "$HOMELAB_ENV" | cut -d= -f2-)
  [[ -z "$gitea_pass" ]] && { echo "ERROR: GITEA_ADMIN_PASSWORD not in $HOMELAB_ENV"; exit 1; }

  docker exec -u git "$FORGEJO_CONTAINER" gitea admin user change-password \
    --username caboose --password "$gitea_pass" 2>&1
  echo "✓ Forgejo password-change flag cleared"
}

# ── Step 2: Create Woodpecker OAuth2 app in Forgejo ──────────────────────────

WOODPECKER_CLIENT_ID=""
WOODPECKER_CLIENT_SECRET=""

step2_create_woodpecker_oauth_app() {
  echo "==> Step 2: Creating Woodpecker OAuth2 app in Forgejo..."
  local gitea_pass forgejo_ip forgejo_url existing_id result

  gitea_pass=$(grep "^GITEA_ADMIN_PASSWORD=" "$HOMELAB_ENV" | cut -d= -f2-)
  forgejo_ip=$(docker inspect forgejo --format='{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}')
  forgejo_url="http://$forgejo_ip:3000"

  existing_id=$(get_forgejo_oauth_app_id "$forgejo_url" "caboose:$gitea_pass" "Woodpecker CI")

  if [[ -n "$existing_id" ]]; then
    echo "   Woodpecker CI app already exists (ID: $existing_id)"
    echo "   NOTE: Re-run requires deleting the existing app in Forgejo UI to regenerate secret."
    echo "   Fetching existing client_id..."
    WOODPECKER_CLIENT_ID=$(curl -sf "$forgejo_url/api/v1/user/applications/oauth2" \
      -u "caboose:$gitea_pass" \
      | jq -r ".[] | select(.id == $existing_id) | .client_id")
    echo "   client_id: $WOODPECKER_CLIENT_ID (secret not re-fetchable)"
  else
    result=$(curl -sf -X POST "$forgejo_url/api/v1/user/applications/oauth2" \
      -u "caboose:$gitea_pass" \
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
  local portainer_token client_id client_secret

  portainer_token=$(curl -sf -X POST "$PORTAINER_URL/api/auth" \
    -H "Content-Type: application/json" \
    -d "{\"Username\":\"admin\",\"Password\":\"$PORTAINER_ADMIN_PASS\"}" \
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
        \"RedirectURI\":          \"$PORTAINER_URL/\",
        \"UserIdentifier\":       \"preferred_username\",
        \"Scopes\":               \"openid email profile\",
        \"OAuthAutoCreateUsers\": true
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
SCRIPT
chmod +x dev/homelab/finish-sso.sh
```

- [ ] **Step 2: Run unit tests — all should pass now**

```bash
cd dev/homelab && bats tests/unit/
```

Expected output:
```
 ✓ upsert_env_var: adds new key when not present
 ✓ upsert_env_var: replaces existing key value
 ✓ upsert_env_var: does not duplicate key
 ✓ get_forgejo_oauth_app_id: returns empty string when app not found
 ✓ get_forgejo_oauth_app_id: returns id when app exists
 ✓ get_forgejo_oauth_app_id: returns empty when different app exists
 ✓ upsert_env_var: calling twice with same value is safe
 ✓ get_forgejo_oauth_app_id: skips creation when app already exists

8 tests, 0 failures
```

- [ ] **Step 3: Commit**

```bash
git add dev/homelab/finish-sso.sh
git commit -m "feat(homelab): add finish-sso.sh to complete SSO configuration"
```

---

## Task 6: Write integration tests — Forgejo

**Files:**
- Create: `dev/homelab/tests/integration/test_forgejo.bats`

- [ ] **Step 1: Write the tests**

```bash
cat > dev/homelab/tests/integration/test_forgejo.bats << 'EOF'
#!/usr/bin/env bats
# Requires: live Forgejo container, finish-sso.sh already run

FORGEJO_IP=$(docker inspect forgejo --format='{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' 2>/dev/null)
FORGEJO_URL="http://$FORGEJO_IP:3000"
GITEA_PASS=$(grep "^GITEA_ADMIN_PASSWORD=" /opt/homelab/.env | cut -d= -f2-)

@test "Forgejo: caboose user has no must-change-password flag" {
  run docker exec -u git forgejo gitea admin user list
  [ "$status" -eq 0 ]
  # Forgejo API returns user info — verify API is accessible (no 403)
  result=$(curl -sf "$FORGEJO_URL/api/v1/user" -u "caboose:$GITEA_PASS" -w "%{http_code}")
  http_code="${result: -3}"
  [ "$http_code" = "200" ]
}

@test "Forgejo: Woodpecker CI OAuth app exists" {
  run curl -sf "$FORGEJO_URL/api/v1/user/applications/oauth2" -u "caboose:$GITEA_PASS"
  [ "$status" -eq 0 ]
  echo "$output" | jq -e '.[] | select(.name == "Woodpecker CI")' > /dev/null
}

@test "Forgejo: Woodpecker CI redirect URI is correct" {
  result=$(curl -sf "$FORGEJO_URL/api/v1/user/applications/oauth2" -u "caboose:$GITEA_PASS" \
    | jq -r '.[] | select(.name == "Woodpecker CI") | .redirect_uris[0]')
  [ "$result" = "https://ci.caboose-ai.io/authorize" ]
}
EOF
```

- [ ] **Step 2: Commit**

```bash
git add dev/homelab/tests/integration/test_forgejo.bats
git commit -m "test(homelab): add Forgejo integration tests"
```

---

## Task 7: Write integration tests — Portainer

**Files:**
- Create: `dev/homelab/tests/integration/test_portainer.bats`

- [ ] **Step 1: Write the tests**

```bash
cat > dev/homelab/tests/integration/test_portainer.bats << 'EOF'
#!/usr/bin/env bats
# Requires: live Portainer, finish-sso.sh already run, PORTAINER_ADMIN_PASS set

PORTAINER_URL="http://127.0.0.1:9000"

@test "Portainer: authentication method is OAuth (3)" {
  [ -n "${PORTAINER_ADMIN_PASS:-}" ] || skip "PORTAINER_ADMIN_PASS not set"
  token=$(curl -sf -X POST "$PORTAINER_URL/api/auth" \
    -H "Content-Type: application/json" \
    -d "{\"Username\":\"admin\",\"Password\":\"$PORTAINER_ADMIN_PASS\"}" \
    | jq -r '.jwt')
  method=$(curl -sf "$PORTAINER_URL/api/settings" \
    -H "Authorization: Bearer $token" \
    | jq -r '.AuthenticationMethod')
  [ "$method" = "3" ]
}

@test "Portainer: OAuth ClientID matches Authentik Portainer SSO provider" {
  [ -n "${PORTAINER_ADMIN_PASS:-}" ] || skip "PORTAINER_ADMIN_PASS not set"
  [ -n "${AUTHENTIK_TOKEN:-}" ]      || skip "AUTHENTIK_TOKEN not set"
  token=$(curl -sf -X POST "$PORTAINER_URL/api/auth" \
    -H "Content-Type: application/json" \
    -d "{\"Username\":\"admin\",\"Password\":\"$PORTAINER_ADMIN_PASS\"}" \
    | jq -r '.jwt')
  portainer_client_id=$(curl -sf "$PORTAINER_URL/api/settings" \
    -H "Authorization: Bearer $token" | jq -r '.OAuthSettings.ClientID')
  expected_client_id=$(curl -sf "https://auth.caboose-ai.io/api/v3/providers/oauth2/?search=portainer" \
    -H "Authorization: Bearer $AUTHENTIK_TOKEN" \
    | jq -r '.results[] | select(.name | ascii_downcase | contains("portainer")) | .client_id' | head -1)
  [ "$portainer_client_id" = "$expected_client_id" ]
}
EOF
```

- [ ] **Step 2: Commit**

```bash
git add dev/homelab/tests/integration/test_portainer.bats
git commit -m "test(homelab): add Portainer integration tests"
```

---

## Task 8: Write integration tests — Woodpecker

**Files:**
- Create: `dev/homelab/tests/integration/test_woodpecker.bats`

- [ ] **Step 1: Write the tests**

```bash
cat > dev/homelab/tests/integration/test_woodpecker.bats << 'EOF'
#!/usr/bin/env bats
# Requires: finish-sso.sh already run

HOMELAB_ENV="/opt/homelab/.env"

@test "Woodpecker: WOODPECKER_GITEA_CLIENT is set in .env" {
  run grep "^WOODPECKER_GITEA_CLIENT=" "$HOMELAB_ENV"
  [ "$status" -eq 0 ]
  [[ "$output" != *"CHANGE_ME"* ]]
  # Value should be non-empty after the = sign
  value="${output#*=}"
  [ -n "$value" ]
}

@test "Woodpecker: WOODPECKER_GITEA_SECRET is set in .env" {
  run grep "^WOODPECKER_GITEA_SECRET=" "$HOMELAB_ENV"
  [ "$status" -eq 0 ]
  value="${output#*=}"
  [ -n "$value" ]
}

@test "Woodpecker: docker-compose.yml references WOODPECKER_GITEA_CLIENT" {
  run grep "WOODPECKER_GITEA_CLIENT" /opt/homelab/docker-compose.yml
  [ "$status" -eq 0 ]
}

@test "Woodpecker: woodpecker-server container is running" {
  run docker ps --filter "name=woodpecker-server" --filter "status=running" --format "{{.Names}}"
  [ "$status" -eq 0 ]
  [[ "$output" == *"woodpecker-server"* ]]
}

@test "Woodpecker: woodpecker-agent container is running" {
  run docker ps --filter "name=woodpecker-agent" --filter "status=running" --format "{{.Names}}"
  [ "$status" -eq 0 ]
  [[ "$output" == *"woodpecker-agent"* ]]
}
EOF
```

- [ ] **Step 2: Commit**

```bash
git add dev/homelab/tests/integration/test_woodpecker.bats
git commit -m "test(homelab): add Woodpecker integration tests"
```

---

## Task 9: Run the script and verify with integration tests

**Files:**
- Modify: `/opt/homelab/.env` (written by script)
- Modify: `/opt/homelab/docker-compose.yml` (written by script)

- [ ] **Step 1: Export required env vars and run**

```bash
export AUTHENTIK_TOKEN="pSxUmCtbFqQZ6sbLPrYRwBeGjuQXXsXyJpKFIVPXP147G7mvnzyw1M8Uu3bY"
export PORTAINER_ADMIN_PASS="<your-portainer-password>"

bash dev/homelab/finish-sso.sh
```

Expected output (all four steps should print `✓`):
```
==> Step 1: Resetting Forgejo password-change flag...
✓ Forgejo password-change flag cleared
==> Step 2: Creating Woodpecker OAuth2 app in Forgejo...
   ✓ Woodpecker CI OAuth app created
   client_id: <generated-id>
==> Step 3: Writing Woodpecker credentials to stack...
   ✓ .env updated
   ✓ docker-compose.yml updated
✓ Woodpecker restarted with Forgejo OAuth
==> Step 4: Configuring Portainer OAuth...
✓ Portainer OAuth configured

Done. All SSO gaps closed.
```

- [ ] **Step 2: Run integration tests**

```bash
export AUTHENTIK_TOKEN="pSxUmCtbFqQZ6sbLPrYRwBeGjuQXXsXyJpKFIVPXP147G7mvnzyw1M8Uu3bY"
export PORTAINER_ADMIN_PASS="<your-portainer-password>"

cd dev/homelab && bash tests/run_tests.sh --integration
```

Expected:
```
==> Running integration tests...
 ✓ Forgejo: caboose user has no must-change-password flag
 ✓ Forgejo: Woodpecker CI OAuth app exists
 ✓ Forgejo: Woodpecker CI redirect URI is correct
 ✓ Portainer: authentication method is OAuth (3)
 ✓ Portainer: OAuth ClientID matches Authentik Portainer SSO provider
 ✓ Woodpecker: WOODPECKER_GITEA_CLIENT is set in .env
 ✓ Woodpecker: WOODPECKER_GITEA_SECRET is set in .env
 ✓ Woodpecker: docker-compose.yml references WOODPECKER_GITEA_CLIENT
 ✓ Woodpecker: woodpecker-server container is running
 ✓ Woodpecker: woodpecker-agent container is running

10 tests, 0 failures
```

- [ ] **Step 3: Commit updated stack files**

```bash
git -C /opt/homelab add docker-compose.yml
# .env is gitignored — don't commit it
git add dev/homelab/
git commit -m "feat(homelab): complete SSO — Woodpecker OAuth, Portainer OAuth, Forgejo admin"
```

---

## Task 10: Push to GitHub

- [ ] **Step 1: Push**

```bash
git push origin main
```

- [ ] **Step 2: Verify repo at github.com/caboose-ai/caboose-ai.io**

Check that `dev/homelab/finish-sso.sh` and `dev/homelab/tests/` are visible in the repo.

---

## Running tests going forward

```bash
# Unit tests only (no live services needed)
cd dev/homelab && bash tests/run_tests.sh --unit

# Integration tests (requires running stack + env vars)
export AUTHENTIK_TOKEN="..." PORTAINER_ADMIN_PASS="..."
cd dev/homelab && bash tests/run_tests.sh --integration

# Everything
cd dev/homelab && bash tests/run_tests.sh --all
```
