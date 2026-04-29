# Homelab SSO — Known Improvements

This file documents gaps, known issues, and future additions in the current SSO stack.
Items are ordered roughly by effort/impact.

---

## Known Gaps

### 1. Grafana OAuth credentials hardcoded in `docker-compose.yml`

**Status:** Works, but secrets live in compose instead of `.env`
**Fix:** Replace the literal `GF_AUTH_GENERIC_OAUTH_CLIENT_ID` and `GF_AUTH_GENERIC_OAUTH_CLIENT_SECRET`
values with `"${GF_AUTH_GENERIC_OAUTH_CLIENT_ID}"` / `"${GF_AUTH_GENERIC_OAUTH_CLIENT_SECRET}"` env var
references, and add them to `.env`.

---

### 2. Forgejo → Authentik OIDC email conflict loop

**Status:** Auth source is added, but first-time enrollment fails with an email conflict when a
local Forgejo account already exists with the same address.
**Root cause:** Forgejo's `external_login_user` table has no entry for the Authentik source, so
it refuses to link the incoming OIDC identity to the existing local account.
**Fix options:**
- Delete the local Forgejo account (or change its email) and re-enroll via OIDC.
- Use the Forgejo admin API to insert a row in `external_login_user` linking the existing user to
  the Authentik source (account-linking endpoint: `POST /api/v1/admin/users/{username}/oauth2`).
- Add a Forgejo admin CLI step to `configure-sso.sh` that pre-links accounts after adding the
  auth source.

---

### 3. Woodpecker secret idempotency

**Status:** `finish-sso.sh` step 2 can retrieve the existing OAuth app's `client_id` but cannot
re-fetch the `client_secret` (Forgejo only returns it at creation time).
**Symptom:** Re-running `finish-sso.sh` when the Woodpecker CI OAuth app already exists prints
`"secret not re-fetchable"` and aborts step 3.
**Fix:** Add a `--force` flag to `finish-sso.sh` that deletes the existing OAuth app and
re-creates it, generating a fresh secret. The delete call is:
`DELETE /api/v1/user/applications/oauth2/{id}` on the Forgejo API.

---

### 4. Mattermost OIDC requires manual `config.json` edit

**Status:** Credentials are wired in `.env`, but Mattermost reads OAuth config from
`/opt/mattermost/config/config.json` (`GitLabSettings`), which is not Docker-managed.
**Fix options:**
- Mount a templated `config.json` as a Docker volume (requires restructuring the Mattermost service).
- Add a `step5_configure_mattermost` to `finish-sso.sh` that uses `docker exec` + `jq` to patch
  the JSON in-place:
  ```bash
  docker exec mattermost sh -c "jq '.GitLabSettings.Enable = true | ...' /opt/mattermost/config/config.json > /tmp/mm.json && mv /tmp/mm.json /opt/mattermost/config/config.json"
  ```

---

### 5. N8N — no SSO (enterprise blocker)

**Status:** N8N community edition does not support OIDC/SAML. Basic auth only.
**Fix:** No workaround available without an N8N enterprise license. Acceptable as-is; the service
is behind Caddy which can provide an auth gate if needed.

---

### 6. SonarQube — no native SSO (community edition)

**Status:** SonarQube community edition lacks SAML/OIDC support. Currently accessible behind
Caddy's forward-auth gate (Authentik) as a coarse access control.
**Fix:** Either upgrade to SonarQube Developer Edition (paid) for native SAML SSO, or accept the
Caddy auth gate as sufficient.

---

## Potential Future Additions

- **Vaultwarden** (self-hosted Bitwarden) with Authentik SSO — Vaultwarden supports OIDC natively
- **Traefik** as a Caddy alternative (labels-based routing, built-in Let's Encrypt, better
  middleware ecosystem for forward auth)
- **Homarr → Authentik OIDC** — Homarr has an OAuth integration but requires additional config;
  not currently enabled
- **Automated Authentik backup** — `authentik export` on a cron, stored to a mounted volume
