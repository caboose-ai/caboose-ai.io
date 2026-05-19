# CAB-4 Technical Baseline and Top-5 Improvement Backlog

Generated: 2026-05-18 (UTC)
Repository: `caboose-ai.io`

## Baseline Snapshot

- Language/runtime: Go monorepo with `go.mod` root, `cmd/*` binaries, and `internal/*` shared packages.
- Entry points discovered: 4 (`cmd/homelab`, `cmd/mcp`, `cmd/pr-ready-watch`, `cmd/telegram-agent`).
- Service workspaces discovered: 18 `services/*/service.yaml` manifests.
- Go test files discovered: 67 (`*_test.go`).
- Documentation files in `docs/`: 14.
- CI workflows discovered: 7 (`ci`, `docs-check`, `golangci-lint`, `secret-scan`, `release-please`, `release-please-automerge`, `update-homebrew-tap`).
- Prior architecture audit in repo (`docs/homelab-architecture-audit.md`) reports no manifest/compose/servicebuilder drift as of 2026-05-10.

## Verification Notes

- Could not execute `go test ./...` or `go build ./...` inside the Paperclip heartbeat because the containerized runner did not have `go` or the host mise shims available.
- Baseline therefore relies on static repository inspection plus existing committed reports/workflows. Host-side validation should be run before promoting any implementation branch.

## Strengths

- Service model is normalized: service metadata is mostly centralized under `services/<slug>/service.yaml` + per-service README.
- CI enforces core quality gates on PRs to `main`: PR-title convention, test/build, lint, docs check, and secret scan.
- Release flow is automated end-to-end with Release Please plus guarded auto-merge.
- Operator workflows are well surfaced via `mise.toml` task catalog.

## Primary Gaps

- Toolchain pinning is now in place (`go.mod`, `mise.toml`, and CI all enforce `1.26.3`), but compatibility policy is still implicit and should be documented for future upgrades.
- Baseline metrics are ad hoc: no single command/report generates ongoing architecture + quality scorecard.
- Test layering is broad but undifferentiated: no explicit fast-vs-slow suite policy in CI lanes.
- Operational scripts in `dev/homelab/` are numerous and lightly centralized for ownership/criticality tracking.
- Documentation freshness is manual; audits/reports can become stale without automation.

## Top-5 Improvement Backlog (Ranked)

1. Finalize Go toolchain contract policy now that pinning is enforced.
   - Impact: preserves reliability gains and avoids accidental drift during upgrades.
   - Scope: document upgrade policy (owner, cadence, minimum support window) and keep `go.mod` as the single source of truth with automated checks.
   - Done when: version bump procedure is documented and referenced from contributor docs.

2. Add an automated baseline report task (architecture + quality metrics).
   - Impact: continuous visibility into service coverage, test counts, workflow health, and drift.
   - Scope: script that emits a markdown/json scorecard into `docs/` (or CI artifact) and can run in CI nightly/on-demand.
   - Done when: one command produces CAB-4-style snapshot without manual grep/count steps.

3. Split CI test strategy into explicit tiers (fast unit, integration/live smoke, security).
   - Impact: faster PR feedback and clearer signal ownership.
   - Scope: keep mandatory fast lane on PRs; move live smoke (`sso:check*`) and heavier checks to scheduled/manual + required gates for release branches.
   - Done when: each lane has documented SLA/owner and failing lane points to concrete remediation runbooks.

4. Create service-operability ownership matrix for `dev/homelab` scripts and smoke flows.
   - Impact: reduces operational bus factor and maintenance ambiguity.
   - Scope: add machine-readable ownership/criticality metadata for scripts and tie to service manifests or docs.
   - Done when: each high-impact script/smoke flow has an owner, criticality, and last-verified timestamp.

5. Add automated docs-freshness checks for dated audit documents.
   - Impact: prevents stale architecture/security guidance from being treated as current.
   - Scope: CI check to flag docs older than threshold unless explicitly marked historical.
   - Done when: dated audit docs include freshness metadata and CI warns/fails on stale "current-state" reports.

## Suggested Execution Order

1. Toolchain contract pinning (Backlog #1).
2. Baseline report automation (Backlog #2).
3. CI tiering policy (Backlog #3).
4. Operability ownership matrix (Backlog #4).
5. Docs freshness automation (Backlog #5).
