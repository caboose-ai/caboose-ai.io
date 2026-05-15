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

The Paperclip container bind-mounts `PAPERCLIP_WORKSPACE_ROOT`, defaulting to
`/home/caboose/dev/caboose-ai.io`, at the same path inside the container. Keep
this value aligned with the repo path passed to `paperclip:seed`; otherwise
Paperclip-managed agent runs will skip the workspace because the local path is
not available inside the container.

The Paperclip app runs with `network_mode: host` and reaches Postgres through
`${PAPERCLIP_DB_BIND_ADDRESS:-127.0.0.1}:5433`. Keep that bind address on
loopback. The Paperclip DB bridge is intentionally not marked `internal: true`
so Docker publishes the loopback DB port for the host-network app.

Paperclip-managed agents use `PAPERCLIP_API_URL=http://127.0.0.1:3100` for
internal API calls. Keep this separate from `PAPERCLIP_PUBLIC_URL`; the
public URL is Authentik-gated and returns browser redirects to local agents.

The seed path creates missing company, goal, project, agent, and routine
records. It does not update existing Paperclip records in place; when seed
guidance changes, reseeding adds any new Agent Control Plan project or routine
records, and existing agent instructions should be reviewed from the Paperclip
workspace until Paperclip exposes a stable update endpoint for this client.

## Agent Control Plan v1

Paperclip is the plan-only v1 intake, planning, approval, evidence, and
follow-up surface for the homelab agent-control workflow. Use it to capture the
goal, affected services, token budget, approval requirements, execution
checklist, evidence, PR links, and follow-up work.

The safe default is `Mode: plan-only`. Agents may inspect, branch, test,
commit, open PRs, query monitoring, and propose deploy actions. Direct infra
execution remains approval-gated. Write, deploy, restart, secret, destructive,
external-token-spending, Portainer, and Docker mutation tasks require explicit
human approval before execution.

Use the self-hosted delivery loop as the control surface: Forgejo/Gitea for
branches and PRs, Woodpecker for CI evidence, and Portainer for container state
review. Portainer and Docker mutations are review inputs until a human approves
execution through OpenClaw, Telegram Agent Bridge, Homelab MCP, or deterministic
`mise`/`homelab` commands.

### Internal Delivery Wiring

`mise run paperclip:seed` writes concrete delivery metadata into each seeded
Paperclip project workspace and agent adapter config:

- Forgejo repository: `https://git.caboose-ai.io/caboose-ai/caboose-ai.io.git`
- Forgejo remote name: `forgejo`
- Branch prefix: `paperclip/`
- CI evidence source: Woodpecker at `https://ci.caboose-ai.io`
- Pipeline file: `.woodpecker.yml`
- Runtime inspection surface: Portainer at `https://docker.caboose-ai.io`
- Runtime mutation policy: `human_approval_required`

If the internal repo path differs, reseed with:

```bash
mise run paperclip:seed -- --forgejo-repo-url https://git.caboose-ai.io/<owner>/<repo>.git
```

Before assigning implementation work, make sure the local checkout has a
matching remote:

```bash
mise run paperclip:forgejo-remote
```

Override `PAPERCLIP_FORGEJO_REPO_URL` when the internal repository lives at a
different owner/path. Agents should link Forgejo PRs and Woodpecker runs back to
the Paperclip task before asking for final human review.

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
   verification checklist, desired output, Forgejo/Gitea branch or PR target,
   Woodpecker evidence needs, and any Portainer/Docker review notes. Keep
   `Mode: plan-only` for v1.
4. Assign the task to the seeded role that best matches the work, such as
   CEO/PM for triage, Architect for design, DevOps/SRE for service operations,
   Backend Engineer for Go/MCP changes, QA Engineer for verification, or
   Security Engineer for secrets and SSO risk review.
5. Have the assigned role produce a plan in Paperclip. If the plan needs writes
   or token-spending, approve it explicitly and hand execution to OpenClaw,
   Telegram Agent Bridge, Homelab MCP, or a deterministic `mise`/`homelab`
   command. Keep Portainer and Docker mutations blocked until this approval is
   recorded.
6. Attach the Forgejo/Gitea branch or PR link, Woodpecker status, command
   output, smoke evidence, rollback notes, and follow-up tasks back to the
   Paperclip task.

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
