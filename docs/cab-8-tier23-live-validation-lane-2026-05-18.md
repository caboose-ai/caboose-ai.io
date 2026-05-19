# CAB-8 Follow-up: Tier 2/3 Live Validation Lane With Evidence Capture (May 18, 2026)

Issue: [CAB-8](/CAB/issues/CAB-8)
Parent: [CAB-6](/CAB/issues/CAB-6)

## Goal

Operationalize a single command lane for Tier 2/Tier 3 live checks and produce
artifacts that can be attached directly to issue evidence.

## Lane Command

From repo root:

```bash
# No Tier 3 services changed (Tier 2 + paperclip control-plane smoke)
mise run cab:tier23-live

# Tier 3 services changed (comma-separated)
dev/cab-tier23-live-validation.sh "forgejo,paperclip"
```

## CI Workflow Lane

Workflow: `.github/workflows/cab-tier23-live-validation.yml`

- Scheduled trigger: Mondays at `06:35 UTC`.
- Manual trigger: `workflow_dispatch` with:
  - `tier2_mode`: `quick` or `full`
  - `changed_services`: comma-separated service list for Tier 3 targeted smokes
- Ownership: Delivery/CI maintainers for workflow runtime; issue assignee for
  rerun and follow-up when FAIL occurs.
- GitHub-hosted runners do not have the homelab compose directory by default;
  the workflow records Tier 3 service checks as `SKIP` when
  `HOMELAB_COMPOSE_DIR/docker-compose.yml` is absent. Run the local command on
  the homelab host for authoritative live service evidence.

## What The Lane Runs

1. Tier 2 quick identity contract check:
   - `mise run sso:check-quick`
2. Tier 3 targeted service smokes (if provided):
   - `mise run service:smoke -- <slug>` for each changed service
3. Tier 3 mandatory control-plane smoke:
   - `mise run paperclip:smoke`

## Evidence Capture

Each lane run writes:

- log: `docs/evidence/cab-8/tier23-live-<timestamp>.log`
- summary: `docs/evidence/cab-8/tier23-live-<timestamp>-summary.md`

Summary includes UTC timestamp, changed-service input, and PASS/FAIL/SKIP per
command so CAB issue comments can attach a durable, normalized result set.

Workflow runs upload the same artifacts as a GitHub Actions artifact bundle so
the CAB-6 evidence template can be filled directly from run output.

## Operator Notes

- If live prerequisites are missing (for example both `AUTHENTIK_TOKEN` and
  `AUTHENTIK_BOOTSTRAP_TOKEN`, or the homelab compose directory on a generic CI
  runner), commands may skip according to existing smoketest behavior; keep the
  generated summary as the evidence source of record.
- For CAB evidence comments, link the summary file and include follow-up owner
  when any command returns FAIL.
