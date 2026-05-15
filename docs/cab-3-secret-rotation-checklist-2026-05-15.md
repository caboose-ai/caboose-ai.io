# CAB-3 Secret Rotation Checklist (May 15, 2026)

This checklist tracks rotation/revocation for secrets exposed by a prior committed
root `.env` file.

## Forensic Scope

- Evidence source: unreachable commit `37630f2e4ac0f3ff55d87a4380cd16adb5882278`
  (authored May 3, 2026), path `.env`.
- Extraction method used: key-name-only parsing from blob
  `c43a0eb2e5c5162719fc00460f74f859274cf5a8`.
- Full-history Gitleaks triage was used as a discovery aid. It should not be
  promoted to a required CI gate until the historical CAB-3 findings are closed.
- No secret values are stored in this document.

Keys observed in the leaked `.env` snapshot:

- `AUTHENTIK_SECRET_KEY`
- `AUTHENTIK_PG_PASS`
- `AUTHENTIK_BOOTSTRAP_PASSWORD`
- `AUTHENTIK_BOOTSTRAP_TOKEN`
- `AUTHENTIK_BOOTSTRAP_EMAIL` (identifier; verify if sensitive in your threat model)
- `WOODPECKER_AGENT_SECRET`
- `GRAFANA_ADMIN_PASSWORD`
- `GITEA_ADMIN_PASSWORD`
- `SONAR_DB_PASS`
- `N8N_USER`
- `N8N_PASSWORD`

## Required Rotation / Revocation

1. Rotate Authentik credentials and signing keys:
   - `AUTHENTIK_SECRET_KEY`
   - `AUTHENTIK_PG_PASS`
   - `AUTHENTIK_BOOTSTRAP_PASSWORD`
   - `AUTHENTIK_BOOTSTRAP_TOKEN`
2. Rotate service admin and agent secrets:
   - `WOODPECKER_AGENT_SECRET`
   - `GRAFANA_ADMIN_PASSWORD`
   - `GITEA_ADMIN_PASSWORD`
   - `SONAR_DB_PASS`
3. Rotate n8n credentials:
   - `N8N_USER`
   - `N8N_PASSWORD`
4. Reissue any downstream tokens derived from the rotated roots.
5. Remove/revoke stale credentials in secret stores and provider dashboards.

## Recommended Execution Order

1. Revoke high-risk bearer/token material first (`AUTHENTIK_BOOTSTRAP_TOKEN`).
2. Rotate Authentik core and DB credentials.
3. Rotate service-local admin and agent secrets.
4. Roll/restart dependent services.
5. Validate login/OIDC/service health after each batch.

## Verification Evidence To Attach In CAB-3

- Timestamped command/log output showing each key was rotated or revoked.
- Service health checks after rotation (`mise run sso:check-quick`, and full
  `mise run sso:check` if quick checks pass).
- Access-log audit notes covering the exposure window and 24h after rotation.

## Policy Confirmation

- `.env` and `dev/homelab/.env` are ignored and not tracked in current refs.
- Placeholder values remain in `dev/homelab/.env.example` only.
- CI secret scanning is enabled for new commits.
