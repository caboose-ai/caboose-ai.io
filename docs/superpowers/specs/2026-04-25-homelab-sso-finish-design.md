# Homelab SSO Completion — Design Spec

**Date:** 2026-04-25  
**Status:** Approved  
**Repo:** caboose-ai/caboose-ai.io — `dev/homelab/`

---

## Problem

Three remaining gaps in the homelab SSO setup prevent a fully automated, reproducible stack:

1. **Forgejo admin account** — `caboose` user has a forced password-change flag set, blocking Forgejo API access and Woodpecker OAuth app creation.
2. **Woodpecker CI SSO** — `woodpecker-server` is configured for Gitea OAuth but is missing `WOODPECKER_GITEA_CLIENT` and `WOODPECKER_GITEA_SECRET` env vars. The OAuth2 app in Forgejo doesn't exist yet.
3. **Portainer OAuth** — Authentik has a Portainer SSO provider configured with correct credentials, but Portainer's own auth settings haven't been updated via its REST API.

---

## Approach

A single script `finish-sso.sh` following the same pattern as `configure-sso.sh`. One required env var (`PORTAINER_ADMIN_PASS`); all other credentials are derived from the running stack and Authentik API automatically.

---

## Architecture

### Script: `dev/homelab/finish-sso.sh`

Executes four steps in dependency order:

```
finish-sso.sh
  │
  ├─ Step 1: Reset Forgejo password-change flag
  │     docker exec -u git forgejo gitea admin user change-password
  │     --username caboose --password $GITEA_ADMIN_PASSWORD
  │     (re-uses existing password; just clears the forced-change flag)
  │
  ├─ Step 2: Create Woodpecker OAuth2 app in Forgejo
  │     GET /api/v1/user/applications/oauth2 — check if "Woodpecker CI" exists
  │     If not: POST /api/v1/user/applications/oauth2
  │       name: "Woodpecker CI"
  │       redirect_uris: ["https://ci.caboose-ai.io/authorize"]
  │     Capture client_id + client_secret
  │
  ├─ Step 3: Write Woodpecker credentials to stack
  │     Append/update WOODPECKER_GITEA_CLIENT and WOODPECKER_GITEA_SECRET in
  │     /opt/homelab/.env (upsert — add if missing, replace if present)
  │     Update docker-compose.yml woodpecker-server environment block to
  │     reference ${WOODPECKER_GITEA_CLIENT} and ${WOODPECKER_GITEA_SECRET}
  │     docker compose restart woodpecker-server woodpecker-agent
  │
  └─ Step 4: Configure Portainer OAuth
        POST /api/auth — get JWT with PORTAINER_ADMIN_PASS
        Fetch Portainer SSO client_id + client_secret from Authentik API
        PUT /api/settings — write OAuth config:
          AuthenticationMethod: 3
          ClientID, ClientSecret, AuthorizationURI, AccessTokenURI,
          ResourceURI, RedirectURI, UserIdentifier, Scopes,
          OAuthAutoCreateUsers: true
```

### Idempotency

- Step 1: Safe to re-run (password stays the same, flag is cleared).
- Step 2: Checks for existing app by name before creating; skips if found.
- Step 3: Uses upsert logic for `.env` lines; compose update is idempotent.
- Step 4: PUT is idempotent.

### Required env vars

| Variable | Source |
|---|---|
| `PORTAINER_ADMIN_PASS` | Must be exported by user |
| `AUTHENTIK_TOKEN` | Already created (`cli-configure-sso` token) |
| `GITEA_ADMIN_PASSWORD` | Already in `/opt/homelab/.env` |

---

## Testing

### Framework

`bats` (Bash Automated Testing System) — industry standard for bash unit + integration testing.

### File layout

```
dev/homelab/
  finish-sso.sh
  configure-sso.sh
  tests/
    unit/
      test_helpers.bats        # pure function tests with stubbed curl/docker
      test_idempotency.bats    # verify re-run paths don't duplicate state
    integration/
      test_forgejo.bats        # Forgejo API: user flag cleared, OAuth app exists
      test_portainer.bats      # Portainer API: AuthenticationMethod=3, correct client_id
      test_woodpecker.bats     # .env has WOODPECKER_GITEA_CLIENT, container running
    run_tests.sh               # runner with --unit / --integration / --all flags
```

### Unit test scope

- `get_provider()`: test JSON extraction with mocked curl response
- Credential validation: test empty-string guard exits with error
- Idempotency: stub Forgejo API to return existing app — verify no duplicate POST
- `.env` upsert: test append vs. replace behaviour on a temp file

### Integration test scope (requires live stack)

| Test | Assertion |
|---|---|
| `test_forgejo.bats` | `gitea admin user list` shows no must-change-password; OAuth app "Woodpecker CI" exists in Forgejo API |
| `test_portainer.bats` | `GET /api/settings` returns `AuthenticationMethod=3`, `ClientID` matches Authentik |
| `test_woodpecker.bats` | `/opt/homelab/.env` contains `WOODPECKER_GITEA_CLIENT`; `docker ps` shows woodpecker-server running |

---

## Error handling

- Each step prints a clear `✓` or `ERROR:` message.
- Non-zero curl responses abort with the HTTP status and response body.
- Missing required env vars exit immediately with usage hint.

---

## File locations

| File | Path in repo |
|---|---|
| Main script | `dev/homelab/finish-sso.sh` |
| Test runner | `dev/homelab/tests/run_tests.sh` |
| Unit tests | `dev/homelab/tests/unit/` |
| Integration tests | `dev/homelab/tests/integration/` |
| This spec | `docs/superpowers/specs/2026-04-25-homelab-sso-finish-design.md` |

---

## Out of scope

- N8N native OIDC (requires enterprise license)
- Mattermost config changes (already fully configured)
- Open-WebUI (env vars already correct)
- Grafana (already working)
