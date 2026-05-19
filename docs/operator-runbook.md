# Operator Runbook

This runbook is the fastest path for day-2 operators who need to verify stack
health, run MCP access checks, and troubleshoot safely.

## Operator First Hour

1. Verify service health:

   ```bash
   mise run service:status -- forgejo
   mise run service:status -- authentik
   mise run service:status -- paperclip
   ```

2. Run quick SSO validation:

   ```bash
   mise run sso:check-quick
   ```

3. Validate MCP endpoint with the wrapper flow:

   ```bash
   mise run mcp:access-live
   mise run mcp:test-live
   ```

4. If anything fails, follow the troubleshooting ladder below before making
   changes.

## Intent-Based Operations

### Check Current Runtime State

```bash
mise run service:status -- <slug>
mise run service:logs -- <slug>
mise run service:smoke -- <slug>
```

### Repair Or Reconfigure A Service

```bash
mise run service:configure -- <slug>
mise run service:configure -- <slug> --dry-run
```

### Validate SSO Boundary

```bash
mise run sso:check-quick
mise run sso:check
```

## MCP Path Selector

Use this selector to choose the right MCP access path.

| Situation | Recommended path | Why |
| --- | --- | --- |
| You want a full live operator check | `mise run mcp:access-live` | Covers setup, request/approve/import, token mint, and initialize probe in one flow |
| You already have credentials and only need endpoint validation | `mise run mcp:test-live` | Reuses existing credential or `HOMELAB_MCP_TOKEN` without creating new access |
| You need to debug or inspect one access step | Manual commands (`homelab-mcp access ...`, `homelab mcp access ...`) | Exposes each stage separately for targeted troubleshooting |
| You need host-level direct script control | `dev/homelab/mcp-access-live.sh` / `dev/homelab/mcp-test-live.sh` | Script wrappers mirror the same flow outside `mise` task indirection |

Manual flow reference:

```bash
homelab-mcp access request --name "codex on laptop" --out mcp-request.json
homelab mcp access setup
homelab mcp access approve mcp-request.json --out mcp-release.json
homelab-mcp access import mcp-release.json
homelab-mcp access token
```

## Troubleshooting Ladder

Escalate in this order, and stop when a step identifies the cause.

1. Service status:

   ```bash
   mise run service:status -- <slug>
   ```

2. Service logs:

   ```bash
   mise run service:logs -- <slug>
   ```

3. Service smoke:

   ```bash
   mise run service:smoke -- <slug>
   ```

4. Quick SSO checks:

   ```bash
   mise run sso:check-quick
   ```

5. Full SSO checks:

   ```bash
   mise run sso:check
   ```

6. MCP-specific checks when access or tool calls fail:

   ```bash
   mise run mcp:test-live
   mise run mcp:access-live
   ```

7. Escalate with evidence:
   - failing command output
   - affected service slug(s)
   - timestamp in UTC
   - whether failure is reproducible after a second run

## Safety Notes

- Treat `homelab reset --yes` as destructive; use it only with explicit intent.
- Prefer `--dry-run` variants where available before write-capable operations.
- Leave `SMOKETEST_RECOVER_AUTHENTIK_TOKEN` unset for read-only smoke checks;
  setting it to `1` lets smoke tests recover a bootstrap API token from the live
  Authentik container.
- Keep secrets in the configured secret store; do not commit credentials or
  request/release artifacts.
