# SSO Skeleton Lockdown & Template Extraction

**Date:** 2026-04-28
**Status:** Approved

## Context

The homelab SSO stack (`dev/homelab/`) is functionally complete as a reference architecture:
Authentik as the central IdP, with OAuth2/OIDC wired into Grafana, Open-WebUI, Portainer, Forgejo,
Woodpecker, and Mattermost. Several integration gaps remain (documented in `IMPROVEMENTS.md`), but
the skeleton is solid enough to serve as a reusable starting point.

**Goal:** Lock the current state as a versioned baseline and extract a generic `homelab-sso-template`
GitHub repo that future projects can clone and adapt.

---

## Design

### Two-repo split

**This repo (`caboose-ai.io`):**
- `dev/homelab/IMPROVEMENTS.md` — annotated gap list with fix notes
- Git tag `homelab-sso-v1` marking the locked baseline

**New repo (`homelab-sso-template`):**
- Mirrors `dev/homelab/` structure exactly
- All domain-specific values replaced with `${DOMAIN}` / `<YOUR_*>` placeholders
- WIP services flagged with `# STATUS: partial — see IMPROVEMENTS.md` comments
- Enabled as a GitHub template repo ("Use this template" button)

---

### Template repo structure

```
homelab-sso-template/
├── README.md                  # Setup checklist
├── docker-compose.yml         # All services, domain/secret placeholders
├── .env.example               # Fully commented, every var explained
├── prometheus.yml             # Unchanged (no domain-specific values)
├── configure-sso.sh           # DOMAIN-parameterized
├── finish-sso.sh              # DOMAIN + FORGEJO_ADMIN_USERNAME parameterized
├── add-social-login.sh        # DOMAIN-parameterized
├── IMPROVEMENTS.md            # Copied from this repo
└── tests/
    ├── run_tests.sh
    ├── unit/
    │   ├── test_helpers.bats
    │   └── test_idempotency.bats
    └── integration/
        ├── test_finish_sso.bats
        └── test_add_social_login.bats
```

---

### Placeholder substitution map

| Live value | Template form |
|---|---|
| `caboose-ai.io` (domain) | `${DOMAIN}` in scripts; `<YOUR_DOMAIN>` in docs |
| `auth.caboose-ai.io` | `auth.${DOMAIN}` |
| `git.caboose-ai.io` | `git.${DOMAIN}` |
| `ci.caboose-ai.io` | `ci.${DOMAIN}` |
| `docker.caboose-ai.io` | `docker.${DOMAIN}` |
| Grafana hardcoded `client_id` / `client_secret` | `${GF_AUTH_GENERIC_OAUTH_CLIENT_ID}` / `${GF_AUTH_GENERIC_OAUTH_CLIENT_SECRET}` (moved to `.env`) |
| `--username caboose` (Forgejo admin) | `--username "${FORGEJO_ADMIN_USERNAME:-admin}"` |
| `caboose:$gitea_pass` (basic auth) | `"${FORGEJO_ADMIN_USERNAME:-admin}:$gitea_pass"` |
| Admin email | `<YOUR_ADMIN_EMAIL>` |
| GitHub/Google OAuth app creds | `<YOUR_GITHUB_CLIENT_ID>` etc. |

Scripts use `DOMAIN="${DOMAIN:?Set DOMAIN to your homelab domain}"` as a required env var so
misconfiguration fails loudly.

---

### README setup checklist (template)

1. **Prerequisites:** Docker, Docker Compose v2, Caddy, `bats` (for tests), `jq`, `curl`
2. **DNS:** Point `auth.*`, `git.*`, `ci.*`, `docker.*`, `grafana.*`, etc. at your server
3. **Configure:** `cp .env.example .env` and fill in all `CHANGE_ME` values
4. **Start:** `docker compose up -d`
5. **Bootstrap Authentik:** Wait ~60s, then open `https://auth.YOUR_DOMAIN` and complete setup
6. **Wire OAuth:** `export DOMAIN=YOUR_DOMAIN AUTHENTIK_TOKEN=... PORTAINER_ADMIN_PASS=...`
   then `bash configure-sso.sh`
7. **Finish SSO:** `bash finish-sso.sh` (wires Woodpecker + Portainer)
8. **Social login (optional):** `bash add-social-login.sh`
9. **Test:** `./tests/run_tests.sh --integration`

---

### WIP service callouts

Services with partial/manual SSO get a comment block in `docker-compose.yml` and scripts:

```yaml
  # STATUS: partial — Forgejo OIDC enrollment has email-conflict issue on existing accounts.
  # See IMPROVEMENTS.md #2 for fix options.
  forgejo:
```

---

## Verification

- [ ] `dev/homelab/IMPROVEMENTS.md` exists with all 6 gap entries
- [ ] `git tag homelab-sso-v1` exists and is pushed
- [ ] `homelab-sso-template` repo exists on GitHub, enabled as template repo
- [ ] No literal `caboose-ai.io` strings in template repo (`grep -r caboose-ai.io .`)
- [ ] No `.env` committed to template (only `.env.example`)
- [ ] Grafana OAuth creds in template reference env vars, not literals
- [ ] `README.md` setup checklist is complete
