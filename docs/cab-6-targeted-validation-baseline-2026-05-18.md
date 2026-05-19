# CAB-6 Targeted Validation Baseline (May 18, 2026)

Issue: [CAB-6](/CAB/issues/CAB-6)
Parent context: [CAB-1](/CAB/issues/CAB-1)

## Goal

Establish a minimal, repeatable validation baseline for the highest-risk homelab workflows after credential/security-impacting changes.

## Top Workflows Covered

1. Authentik SSO contract integrity (provider/app/source/outpost URL + config).
2. Protected service login handoff via manifest-owned smoke flows.
3. Paperclip private runtime health behind Authentik proxy.
4. Service-manifest to smoke-flow resolution contract.

## Baseline Command Set

Run from repo root.

### A. Fast offline contract checks (required per PR touching auth/service wiring)

```bash
go test ./internal/smoketest -run 'TestPaperclipSmokeFlowResolvesToProxyFlow|TestServiceManifestSmokeFlowsResolve|TestHealthOnlyServicesDoNotDeclareSmokeFlows' -v
```

Purpose:
- Ensures declared `smoke_flow` values resolve to executable smoke targets.
- Prevents drift between `services/*/service.yaml` and `internal/smoketest/flows.go`.

### B. Live quick SSO config check (required when stack is available)

```bash
mise run sso:check-quick
```

Purpose:
- Verifies Authentik/provider/application/source/outpost contracts without browser-flow overhead.

### C. Targeted high-value service smokes (run on changed services + Paperclip)

```bash
# Replace <slug> with changed service(s)
mise run service:smoke -- <slug>

# Always include paperclip for control-plane reliability
mise run paperclip:smoke
```

Purpose:
- Confirms end-to-end login/access paths for modified surface area.
- Confirms Paperclip app health and proxy/runtime integration remain intact.

## Passing Criteria

- Offline contract checks pass with no unknown smoke flows.
- `sso:check-quick` passes on live stack.
- Targeted service smokes pass for changed services and Paperclip.
- Any failure yields a ticketed follow-up with owner and rerun evidence.

## Evidence Format For CAB-6

Attach to [CAB-6](/CAB/issues/CAB-6):

- Commit SHA under test.
- Exact commands executed.
- Pass/fail per command.
- If failed: first failing assertion/log snippet and linked follow-up issue.

## Notes From This Heartbeat

- The original Paperclip runner lacked `go` and `mise`, but the host workspace has both.
- Host verification on 2026-05-18 passed the offline smoke-flow contract check.
- `mise run sso:check-quick` and `mise run paperclip:smoke` completed with skipped integration tests because `AUTHENTIK_TOKEN` / `AUTHENTIK_BOOTSTRAP_TOKEN` was not present in env or `.env`.
- Live stack checks are environment-dependent; offline contract checks are the minimum always-on gate when live Authentik credentials are unavailable.
