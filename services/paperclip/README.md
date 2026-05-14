# Paperclip

Paperclip is an optional multi-agent workspace service behind the compose
`paperclip` profile. It is exposed at `paperclip.caboose-ai.io`, protected by
the Authentik forward-auth provider `paperclip-proxy`, and shown on Homarr.
Paperclip itself runs in `local_trusted` mode on host loopback so Authentik is
the only browser login prompt.

## Operations

```bash
mise run paperclip:up
mise run paperclip:status
mise run paperclip:smoke
mise run paperclip:seed
```

`paperclip:seed` uses `homelab paperclip seed-company --profile software-shop`
and reads `PAPERCLIP_API_KEY` from the environment or configured secret store
when the API requires bearer auth.

## Agent Planning and Execution

Paperclip can trigger the homelab agent-control workflow as the task intake,
planning, and audit surface. Use it to capture the goal, affected services,
token budget, approval requirements, execution checklist, evidence, PR links,
and follow-up work.

The safe default is for Paperclip to plan first and execute only through
approved control paths: OpenClaw for interactive supervision, Telegram Agent
Bridge for remote confirmations, Homelab MCP for typed automation tools, and
the Homelab CLI for deterministic service operations. Write, deploy, restart,
secret, destructive, and external-token-spending tasks require explicit human
approval before execution.

### Trigger Runbook

1. Start and verify Paperclip:

   ```bash
   mise run paperclip:up
   mise run paperclip:status
   mise run paperclip:smoke
   mise run paperclip:seed
   ```

2. Open `https://paperclip.caboose-ai.io` and use the seeded Caboose AI
   Software Shop workspace.
3. Create a task with the goal, service slugs, token budget, approval needs,
   verification checklist, and desired output. Use `Mode: plan-only` until the
   task classifier, context index, confirmation gates, and MCP tools are ready.
4. Assign the task to the seeded role that best matches the work, such as
   CEO/PM for triage, Architect for design, DevOps/SRE for service operations,
   Backend Engineer for Go/MCP changes, QA Engineer for verification, or
   Security Engineer for secrets and SSO risk review.
5. Have the assigned role produce a plan in Paperclip. If the plan needs writes
   or token-spending, approve it explicitly and hand execution to OpenClaw,
   Telegram Agent Bridge, Homelab MCP, or a deterministic `mise`/`homelab`
   command.
6. Attach the branch or PR link, command output, smoke evidence, rollback notes,
   and follow-up tasks back to the Paperclip task.

If `PAPERCLIP_PUBLIC_URL` is overridden for a non-default domain, update the
matching Authentik proxy provider and Caddy route.

## Service Contract

Good:

```yaml
smoke_flow: paperclip
dashboard:
  show: true
sso:
  mode: proxy
health:
  url_key: paperclip
  path: /api/health
```

Bad:

```yaml
smoke_flow: paperclip
# No executable smoke flow, dashboard visibility is hidden in Go code,
# and health points at a UI page instead of the app health endpoint.
health:
  path: /
```
