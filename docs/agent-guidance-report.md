# Agent Guidance Report

Repository: `/home/caboose/dev/caboose-ai.io`
Generated: `2026-05-09`

## Executive Summary

This refresh was run after PR #33 landed on `main`. The repository is still a Go homelab infrastructure monorepo centered on Authentik SSO, but the active service surface has changed: n8n is no longer present in compose, service manifests, service links, prompts, smoke flows, or docs.

The previous `2026-05-08` report was stale because it still described n8n as active. This report replaces it as the canonical guidance report. The current proposed guidance set is:

- Existing: `docs/proposed-AGENTS.md`
- Existing: `docs/proposed-dev-homelab-AGENTS.md`
- Existing: `docs/proposed-internal-smoketest-AGENTS.md`
- Existing: `docs/proposed-services-authentik-AGENTS.md`
- New: `docs/proposed-services-homarr-AGENTS.md`

Primary current recommendation: keep the existing root, homelab, smoke-test, and Authentik guidance proposals, and add a nested `services/homarr/AGENTS.md` because Homarr has enough live-state, OIDC, dashboard-seeding, and Docker/SQLite-specific workflow to justify local instructions.

## Current Guidance Inventory

- `CLAUDE.md`: Canonical repository guidance for Claude Code. It covers Go commands, homelab workflows, Authentik state contracts, service manifests, docs checks, and PR hygiene.
- `.github/copilot-instructions.md`: Compatibility pointer that tells Copilot to follow `CLAUDE.md`.
- `AGENTS.md`: Not present at repo root.
- Nested guidance files: none currently checked in.
- Generated proposal docs:
  - `docs/proposed-AGENTS.md`
  - `docs/proposed-dev-homelab-AGENTS.md`
  - `docs/proposed-internal-smoketest-AGENTS.md`
  - `docs/proposed-services-authentik-AGENTS.md`
  - `docs/proposed-services-homarr-AGENTS.md`

## Repository Shape

The repo is a single Go module:

- `cmd/homelab`: User-facing homelab CLI entrypoint.
- `internal/*`: Shared installer, Authentik, service registry, compose, smoke-test, and support packages.
- `services/<slug>`: First-class service workspaces with a `service.yaml`, implementation, tests, and usually README/docs.
- `dev/homelab`: Docker Compose, Authentik bootstrap scripts, smoke-test scripts, and local homelab runtime artifacts.
- `internal/smoketest`: Playwright-backed live SSO validation and service smoke coverage.
- `docs`: Operational documentation and generated guidance proposals.

The repo already has a strong `CLAUDE.md`, so root `AGENTS.md` should mostly translate and condense that guidance for Codex-style agents rather than invent new policy.

## Important Current Service Surface

Active compose/services observed in this refresh:

- Authentik
- Forgejo
- Grafana
- Homarr
- Mattermost
- Open WebUI
- Paperclip
- Portainer
- SonarQube

n8n is intentionally absent from current compose, manifests, installer prompts, service links, smoke targets, and docs. Avoid reintroducing n8n instructions unless the service is restored intentionally.

## Recommended Root AGENTS.md

Use `docs/proposed-AGENTS.md` as the starting point for a root `AGENTS.md`.

Why:

- It aligns with the existing `CLAUDE.md` without being as long.
- It keeps branch/PR hygiene explicit.
- It names the most important validation commands.
- It preserves the repo-specific Authentik, 1Password, and homelab smoke-test expectations.

Suggested next action:

```bash
cp docs/proposed-AGENTS.md AGENTS.md
```

Then run:

```bash
go test ./...
```

No code changes are required for the guidance file itself.

## Recommended Nested AGENTS.md Files

### `dev/homelab/AGENTS.md`

Use `docs/proposed-dev-homelab-AGENTS.md`.

Reason:

- `dev/homelab` mixes live compose state, bootstrap scripts, generated env files, Authentik runtime files, and smoke-test helper scripts.
- Agents need stronger guardrails here because accidental edits can affect live local services or secrets.

Suggested next action:

```bash
cp docs/proposed-dev-homelab-AGENTS.md dev/homelab/AGENTS.md
```

### `internal/smoketest/AGENTS.md`

Use `docs/proposed-internal-smoketest-AGENTS.md`.

Reason:

- Smoke tests depend on Playwright behavior, Authentik flows, service-specific success states, Shadow DOM handling, and live environment assumptions.
- A nested file can keep browser automation guidance close to the tests.

Suggested next action:

```bash
cp docs/proposed-internal-smoketest-AGENTS.md internal/smoketest/AGENTS.md
```

### `services/authentik/AGENTS.md`

Use `docs/proposed-services-authentik-AGENTS.md`.

Reason:

- Authentik provider/source/outpost provisioning is a central contract for all service SSO.
- The package is high-impact and has specific failure modes around missing providers, source flow fields, and state reconciliation.

Suggested next action:

```bash
cp docs/proposed-services-authentik-AGENTS.md services/authentik/AGENTS.md
```

### `services/homarr/AGENTS.md`

Use `docs/proposed-services-homarr-AGENTS.md`.

Reason:

- Homarr now owns dashboard setup, OIDC login wiring, default board seeding, app filtering, and Docker-backed SQLite mutation.
- The live repair path is unusual enough to justify local rules: installer-owned rows use `homelab_app_%` and `homelab_item_%`, seed code should stay idempotent, and dashboard curation belongs in `internal/servicebuilder`.

Suggested next action:

```bash
cp docs/proposed-services-homarr-AGENTS.md services/homarr/AGENTS.md
```

## Candidate Areas That Do Not Need Nested Guidance Yet

### Other `services/<slug>` Packages

Most current service packages are small and follow the common manifest/configurator pattern. A nested AGENTS file for every service would add maintenance overhead before it adds clarity.

Add more service-local guidance later if one of these services gains:

- Live database mutation.
- Multi-step bootstrap or repair scripts.
- Complex SSO-specific behavior.
- Service-owned smoke-test conventions that differ from the rest of the repo.

### `internal/servicebuilder`

This package is now important because it curates service metadata and dashboard app inclusion. It does not yet need a standalone AGENTS file, but future Homarr/dashboard changes should mention it from `services/homarr/AGENTS.md`.

If service planning grows more complex, consider either:

- A nested `internal/servicebuilder/AGENTS.md`.
- A short root guidance note that dashboard inclusion and service metadata should be changed in `internal/servicebuilder`, not duplicated in service packages.

## Documentation Gaps

The previous `2026-05-08` report was stale after n8n removal. This report replaces it and reflects the post-PR #33 service surface.

The main remaining gaps are:

- No checked-in root `AGENTS.md` yet.
- No nested guidance files checked in yet.
- Existing docs still duplicate some guidance across `CLAUDE.md`, `.github/copilot-instructions.md`, and proposed AGENTS docs. If the proposed files are adopted, keep `CLAUDE.md` as the canonical long-form source and make compatibility files concise pointers where possible.

## Validation Performed

This guidance refresh was read-only with respect to source code. The inventory pass inspected file names, directory structure, and non-secret docs/source context. Secret-like file paths such as `.env` were identified as sensitive and were not opened.

Recommended before committing guidance adoption:

```bash
git diff --check
go test ./...
```

For Homarr-specific adoption, also run:

```bash
go test ./internal/servicebuilder ./services/homarr -v
```

## Open Questions

- Whether to adopt all proposed nested guidance files now, or land only root plus Homarr first.
- Whether `docs/proposed-*.md` should remain after guidance files are copied into place, or be removed once the checked-in guidance exists.
- Whether `internal/servicebuilder` should get local guidance if dashboard/service metadata keeps expanding.
