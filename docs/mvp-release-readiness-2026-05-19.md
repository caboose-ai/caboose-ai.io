# MVP Release Readiness Pass (2026-05-19)

## Objective

Provide an implementation-ready, operator-facing release-readiness checklist for
shipping the homelab MVP across three deployment profiles:

- `local`
- `public`
- `public+mcp-external`

This document is written to be executed directly by an operator and used as a
release gate by maintainers.

## Scope and assumptions

- Host OS: Linux
- Operator can run Docker and Docker Compose v2
- `homelab` binary is available in `PATH` (installed from release/Homebrew or
  built from source)
- For MCP external lifecycle management, `systemd --user` is available
- Secrets are provided by **either** 1Password CLI **or** a pre-populated
  compose `.env` strategy

---

## Deployment profiles

### Profile definitions

- **`local`**: single-host/private network evaluation, no public DNS exposure
  required
- **`public`**: domain-exposed services with reverse proxy and SSO/OAuth flows
- **`public+mcp-external`**: `public` plus external MCP onboarding and approval
  workflows

### Prerequisite matrix (operator-ready)

| Dependency / capability | `local` | `public` | `public+mcp-external` | Verification command | Pass criteria |
|---|---|---|---|---|---|
| Docker Engine | Required | Required | Required | `docker version` | Client and Server sections render without daemon error |
| Docker Compose v2 plugin | Required | Required | Required | `docker compose version` | Version string prints and exits 0 |
| `homelab` binary in `PATH` | Required | Required | Required | `homelab --help` | Help output prints usage/commands |
| Secrets strategy selected (`op` or `.env`) | Required | Required | Required | `op --version` (if 1Password mode) or `test -f .env` (if `.env` mode) | For chosen mode, command succeeds |
| Domain + DNS control | Not required | Required | Required | `dig +short <your-domain>` | Returns expected record(s) for host exposure |
| Caddy runtime (for host reverse proxy mode) | Not required | Required | Required | `caddy version` | Version string prints |
| Cloudflare API credentials | Not required | Optional (Turnstile automation) | Required | `env | rg 'CLOUDFLARE_(API_TOKEN|ZONE_ID)'` | Required variables present for selected features |
| `systemd --user` available | Not required | Not required | Required | `systemctl --user status` | Command connects to user manager (not “Failed to connect to bus”) |

> Operator rule: do not begin installation until all rows marked “Required” for
> your selected profile have passed.

---

## 15-minute local quickstart (`local` profile)

Use this when validating first-run MVP experience on a clean-ish Linux host.

### 0) Choose working directory and profile

```bash
export CABOOSE_PROFILE=local
mkdir -p "$HOME/caboose-run" && cd "$HOME/caboose-run"
```

**Pass criteria:** `pwd` shows your intended working directory.

### 1) Verify mandatory runtime prerequisites

```bash
docker version
docker compose version
homelab --help
```

**Pass criteria:** all commands return successfully and print normal version/help
output.

### 2) Select and validate secrets strategy

Choose one path only:

- **Path A (1Password-backed):**

  ```bash
  op --version
  ```

  **Pass criteria:** version prints; operator can authenticate per environment
  policy.

- **Path B (`.env`-backed):**

  ```bash
  test -f .env && echo ".env present"
  ```

  **Pass criteria:** `.env present` is printed.

### 3) Install/bootstrap stack

```bash
homelab install
```

**Pass criteria:** command exits successfully and reports completed bootstrap for
configured services.

### 4) Validate service health and smoke readiness

```bash
homelab service status
homelab service smoke --all
```

**Pass criteria:**

- status output indicates services are up/healthy (no failed core services)
- smoke command completes with passing checks (no failures)

### 5) Validate recovery path

```bash
homelab reset --keep-env
homelab install
homelab service smoke --all
```

**Pass criteria:** reinstall and smoke checks pass after reset without manual
undocumented edits.

---

## Acceptance checks by profile (release gate)

Run these checks on at least one clean host per profile before calling the MVP
release ready.

### A. `local` profile acceptance

1. Prerequisite matrix: all `local` required rows pass.
2. Fresh install completes (`homelab install`).
3. Health/smoke passes:

   ```bash
   homelab service status
   homelab service smoke --all
   ```

4. Recovery pass:

   ```bash
   homelab reset --keep-env
   homelab install
   homelab service smoke --all
   ```

**Release pass criteria (`local`):** all four checks pass with no undocumented
manual intervention.

### B. `public` profile acceptance

1. All `local` acceptance checks pass.
2. Public prerequisites verified (domain/DNS + Caddy + OAuth config).
3. Public endpoints reachable over intended domain(s).
4. SSO callback/authentication flow succeeds end-to-end.

Suggested verification commands:

```bash
dig +short <your-domain>
homelab service status
homelab service smoke --all
```

**Release pass criteria (`public`):** public route + authentication success and
no blocking smoke failures.

### C. `public+mcp-external` profile acceptance

1. All `public` acceptance checks pass.
2. Cloudflare credentials exported and valid for target zone.
3. External MCP setup/probe/access flow completes.

Suggested verification commands:

```bash
env | rg 'CLOUDFLARE_(API_TOKEN|ZONE_ID)'
homelab mcp setup-external
homelab mcp probe
```

**Release pass criteria (`public+mcp-external`):** external setup completes,
probe succeeds, and approval/import onboarding workflow works end-to-end.

---

## Operator run order (single-page checklist)

1. Select profile: `local` / `public` / `public+mcp-external`.
2. Execute prerequisite matrix and clear all required rows.
3. Run install.
4. Run profile acceptance checks.
5. Run recovery check (`reset --keep-env` + reinstall + smoke).
6. Record pass/fail outcome for release gate.

---

## Release decision

Declare MVP release-ready only when:

- `local` passes on a clean host, and
- any additional advertised profiles (`public`, `public+mcp-external`) also
  pass their full acceptance sections on clean hosts.

If a profile is not yet passing, do not claim it in release messaging.
