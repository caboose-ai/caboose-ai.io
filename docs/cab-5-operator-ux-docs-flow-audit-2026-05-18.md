# CAB-5 Operator UX And Docs Flow Audit (2026-05-18)

## Scope

Audit the current operator journey for the homelab software shop, focusing on:

- first-run setup and day-2 operations
- MCP access setup and live validation
- where operators discover commands, risks, and troubleshooting paths

## Current Journey Review

### 1) First-run setup discovery

Operators currently start in `README.md`, then jump to:

- quick start commands
- safety notes around reset and secrets
- service-specific docs under `services/<slug>/README.md`

What works:

- command coverage is broad
- safety guidance for destructive commands is explicit
- service registry structure is clear

Friction:

- no explicit "start here" path for operators versus contributors
- dense command blocks mix install, operations, migration, and release workflows

### 2) Day-2 operations and verification

Operators use `mise run service:*`, `mise run sso:check*`, and Paperclip/Telegram/MCP helpers.

What works:

- strong command surface for status/configure/smoke
- clear validation split between quick and full SSO checks

Friction:

- verification commands are discoverable, but not grouped by operator intent
- troubleshooting path is implicit instead of a documented escalation sequence

### 3) MCP operator flow

README includes both manual access commands and wrapper scripts:

- `homelab-mcp access request/import/token`
- `homelab mcp access setup/approve`
- `dev/homelab/mcp-access-live.sh`
- `dev/homelab/mcp-test-live.sh`

What works:

- complete end-to-end flow is documented
- wrapper scripts reduce repeated manual steps

Friction:

- manual and wrapper paths are mixed in one block with little decision guidance
- no short decision rule for "when to use manual vs wrapper"

## Priority Findings

1. High: README lacks an explicit operator docs map and role-based path.
2. Medium: command reference is rich but not segmented by common operational intents.
3. Medium: MCP flow needs a quick selector for wrapper-driven vs manual path.
4. Low: troubleshooting/escalation sequencing should be easier to scan.

## Changes Shipped In This CAB-5 Pass

1. Added this audit artifact for durable issue history and future doc iteration.
2. Updated `README.md` docs section to expose an operator-focused docs entry point.

## Recommended Next Changes

1. Add `docs/operator-runbook.md` with intent-based sections:
   - install/reset
   - validate
   - MCP setup/access
   - troubleshoot/escalate
2. Add a compact "operator first hour" block near the top of `README.md`.
3. Add an MCP "path selector" table:
   - wrapper (`mise run mcp:access-live`) for most operators
   - manual commands for debugging or partial flows
4. Add a troubleshooting ladder:
   - `service:status` -> `service:logs` -> `service:smoke` -> `sso:check-quick` -> `sso:check`

## Exit Criteria For CAB-5 Completion

- operator docs map is present in README
- runbook exists with intent-based flow
- MCP selector is documented
- at least one new operator path is validated by a fresh-reader pass
