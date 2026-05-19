# CAB-6 / CAB-3 Wave 3: CI Validation Tiers and Evidence Contract (May 18, 2026)

Issue: [CAB-6](/CAB/issues/CAB-6)
Parent: [CAB-3](/CAB/issues/CAB-3)

## Goal

Define a repeatable CI validation policy that scales by change risk and produces
standardized evidence for security and QA review.

This wave converts the targeted baseline into explicit tiers with promotion rules.

## Tier Model

### Tier 0: Repo Hygiene (always-on for every PR)

Required checks:

```bash
git diff --check
mise run lint
go test ./...
```

Pass criteria:

- No whitespace or conflict-marker defects.
- Lint passes with no new violations.
- Go test suite passes.

### Tier 1: Contract Safety (required for auth, service manifests, smoke wiring)

Trigger conditions (any):

- `services/*/service.yaml` changed.
- `internal/smoketest/*` changed.
- `internal/service/*` changed.
- `internal/install/*` changed where service registration/auth contracts are touched.

Required checks:

```bash
go test ./internal/smoketest -run 'TestPaperclipSmokeFlowResolvesToProxyFlow|TestServiceManifestSmokeFlowsResolve|TestHealthOnlyServicesDoNotDeclareSmokeFlows' -v
```

Pass criteria:

- No unknown or unresolved `smoke_flow` targets.
- Health-only services remain exempt from smoke-flow declarations.

### Tier 2: Live Identity Contract (required when Authentik-integrated paths are touched)

Trigger conditions (any):

- Authentik config/setup code changes.
- OIDC / proxy SSO route changes.
- `dev/homelab` auth automation changes.

Required checks:

```bash
mise run sso:check-quick
```

Escalation check (recommended, required for incident fixes):

```bash
mise run sso:check
```

Pass criteria:

- Quick check passes without contract mismatches.
- Full check passes when escalation is required by incident scope.

### Tier 3: Targeted Service Runtime Smoke (required for changed protected services)

Trigger conditions:

- PR modifies one or more service directories under `services/<slug>/`.

Required checks:

```bash
# one command per changed service
mise run service:smoke -- <slug>

# mandatory control-plane coverage
mise run paperclip:smoke
```

Pass criteria:

- All changed service smokes pass.
- Paperclip smoke passes for every Tier 3 run.

## Tier Selection Rules

1. Apply Tier 0 on all PRs.
2. Add Tier 1 when service contracts or smoke wiring changes.
3. Add Tier 2 for identity/auth path changes and incidents tied to SSO risk.
4. Add Tier 3 when service runtime behavior for protected services changes.
5. If multiple triggers apply, run the union of all required tiers.

## Evidence Contract (Required Attachment Format)

Each CAB-6 validation note must include exactly these fields.

```markdown
## Validation Evidence
- Commit: <full sha>
- PR: <url-or-identifier>
- Tier scope: T0[, T1][, T2][, T3]
- Environment: <host/runner name>
- Timestamp (UTC): <ISO-8601>

### Commands Executed
1. `<exact command>`
2. `<exact command>`

### Results
- `<command or check>`: PASS | FAIL | SKIP (<reason if SKIP>)

### Failure Detail (required when any FAIL)
- First failing assertion/log line: `<trimmed snippet>`
- Follow-up issue: <issue link>
- Owner: <agent/person>
- Rerun required: yes|no
```

## Skip Policy

A check may be marked `SKIP` only when prerequisites are unavailable (for
example missing `AUTHENTIK_TOKEN` / `AUTHENTIK_BOOTSTRAP_TOKEN` for live checks).
Every `SKIP` must include:

- explicit missing prerequisite,
- who owns remediation,
- when rerun is expected.

## Promotion Path (Post-Wave 3)

- Keep Tier 1 as the minimum enforceable gate when live credentials are absent.
- Promote Tier 2 quick check to required CI gate once stable credentials are
  available in the runner.
- Keep Tier 3 targeted (service-diff based) to control runtime cost.

## Relationship to Existing Baseline

This document supersedes the ad-hoc command grouping in
`docs/cab-6-targeted-validation-baseline-2026-05-18.md` by defining strict tier
triggers, pass criteria, and a single evidence schema.
