# MVP Release Readiness Pass (2026-05-19)

## Objective

Assess whether a new user can install and run the homelab stack with a released
binary plus baseline host dependencies, and identify what needs tightening
before broader public release.

## Executive Summary

- **Current state:** close to MVP, with strong automation for bootstrap,
  reconfigure, and smoke validation.
- **Primary blocker for "one binary + Docker and go":** runtime has optional but
  practically important dependency branches (1Password, Cloudflare, Caddy,
  systemd user services) that are spread across README and scripts rather than
  presented as a single operator matrix.
- **Release recommendation:** ship as a **guided MVP** with explicit deployment
  profiles (`local`, `public`, `public+mcp-external`) and profile-specific
  prerequisites.

## What looks solid already

1. **Install/reinstall/reset workflows are explicit and scriptable** through
   `homelab install`, `reset`, and `reinstall` task wrappers.
2. **Service registry pattern is coherent** (`services/<slug>/service.yaml` +
   shared `internal/service` contract), reducing drift risk.
3. **Smoke testing paths exist** for both full and per-service checks, plus MCP
   live checks.
4. **Release path is automated** (Release Please + Homebrew tap update workflow).

## Gaps to resolve before broad rollout

1. **Prerequisite discoverability is fragmented.**
   - Runtime dependencies are mentioned, but not in one decision table a new
     operator can execute top-to-bottom.
2. **"Minimum viable install" vs "full public stack" not sharply separated.**
   - New users need one short "start here" profile with defaults and expected
     outcomes.
3. **External integration dependencies are easy to miss.**
   - MCP external setup and Turnstile automation require specific credentials,
     DNS, and host privileges.
4. **Post-install acceptance criteria are implied, not consolidated.**
   - There should be one canonical "MVP is healthy" checklist.

## Dependency matrix from-scratch

### Mandatory (all profiles)

- Docker Engine with Docker Compose v2 plugin
- Linux host with permissions to run Docker and write compose directory
- Homelab binary (`homelab`) installed from Homebrew or built from source

### Required for source-build workflow

- Go toolchain matching repository contract (`go 1.26.x`)
- `mise` (if using repository task wrappers)

### Optional but strongly recommended

- 1Password CLI (`op`) for managed secret storage; otherwise a populated
  compose `.env` fallback is needed

### Required for public/external features

- Caddy (for host reverse proxy modes)
- Cloudflare account + `CLOUDFLARE_API_TOKEN` + `CLOUDFLARE_ZONE_ID`
  (Turnstile automation, MCP external DNS/route setup)
- `systemd --user` availability for managed MCP service lifecycle

### Feature-specific dependencies

- Homebrew (if consuming official formula distribution path)
- Telegram bot token + allowlist values (Telegram agent bridge)
- Local OpenClaw/Ollama runtime availability for gateway-dependent agent flows

## Is it "binary + one pass" ready?

**Answer:** *conditionally yes* for local/private profile; *not yet fully yes* for
public profile unless prerequisites are made profile-explicit.

- A technically fluent user can succeed today with existing docs.
- A first-time operator is likely to stall on prerequisite sequencing,
  especially secrets mode, external DNS/TLS, and MCP external route setup.

## Recommended MVP release paths

### Path A — Local-first MVP (recommended first public claim)

**Audience:** single-host evaluation users.

1. Install Docker + Compose.
2. Install `caboose-homelab` binary.
3. Choose secrets mode:
   - 1Password CLI, or
   - pre-populated `.env`.
4. Run non-interactive install.
5. Run quick smoke checks and per-service status checks.

**Release statement:** "Local/private homelab bootstrap in one command with
service-level status and smoke coverage."

### Path B — Public-domain MVP

**Audience:** operators exposing services on a real domain.

Path A +:

1. Domain DNS and host reachability.
2. Caddy deployment mode configuration.
3. OAuth provider setup and callback verification.
4. Optional Turnstile automation.

**Release statement:** "Public SSO-enabled deployment with guided OAuth setup."

### Path C — External MCP MVP

**Audience:** operators enabling remote MCP clients.

Path B +:

1. Cloudflare DNS/API credentials.
2. `mcp:setup-external` and probe checks.
3. Access request/approve/import live flow.

**Release statement:** "Admin-approved encrypted client onboarding for remote
MCP access."

## Pre-release hardening checklist

1. Add one top-level prerequisite table keyed by profile (A/B/C).
2. Add one "15-minute first install" quickstart for Path A.
3. Add one acceptance checklist with exact commands and expected pass signals.
4. Ensure all docs reference one canonical secrets strategy decision point.
5. Add a short "known environment assumptions" section (Linux host, systemd
   user services for MCP, Docker permissions).

## Suggested immediate next docs updates

1. README: add profile-based prerequisites section.
2. README: add an "MVP acceptance checks" block (status/smoke/probe commands).
3. Operator runbook: add explicit branching for `local` vs `public` vs
   `public+mcp-external`.
4. Optional: publish this checklist in release notes template for each tag.

## Proposed MVP release gate

Recommend shipping when all of the following are true for at least one clean
host run per profile:

- Install succeeds without manual file edits beyond documented `.env`/secrets.
- `service:status` and representative `service:smoke` commands pass.
- Recovery path (`reset --keep-env` + reinstall) succeeds.
- For Path C, MCP external probe and access workflow succeed end-to-end.
