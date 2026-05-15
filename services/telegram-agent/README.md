# Telegram Agent Bridge

Telegram Agent Bridge is an optional host-run bot for controlling the homelab's
AI agents from Telegram. It uses Telegram long polling, restricts access to
`TELEGRAM_ALLOWED_USER_IDS`, and invokes the local OpenClaw model gateway via
the host `openclaw` CLI.

The service is deliberately tracked as `runtime: external`: it needs the host's
OpenClaw credentials and should not be exposed through Caddy, Homarr, or
Docker Compose.

## Configuration

Required:

```bash
export TELEGRAM_BOT_TOKEN=...
export TELEGRAM_ALLOWED_USER_IDS=123456789
```

Optional:

```bash
export TELEGRAM_DEFAULT_MODEL=github-copilot/claude-opus-4.6
export TELEGRAM_ALLOWED_MODELS=github-copilot/claude-opus-4.6,github-copilot/gpt-5.3-codex,ollama/qwen3:14b,ollama/qwen3:8b
export TELEGRAM_REQUIRE_CONFIRMATION=true
export HOMELAB_MCP_CONFIG=/home/caboose/dev/caboose-ai.io/homelab.yml
export HOMELAB_MCP_BINARY=/path/to/homelab-mcp
export HOMELAB_MCP_WORKDIR=/home/caboose/dev/caboose-ai.io
export HOMELAB_MCP_TIMEOUT=2m
```

`TELEGRAM_REQUIRE_CONFIRMATION` is retained for compatibility with existing
host environments. Agent prompts draft plans by default and do not execute
Paperclip or homelab changes directly from Telegram.

`TELEGRAM_NOTIFY_CHAT_ID` can narrow one-shot notifications to a specific chat
ID. If it is unset, `telegram-agent notify` sends to all allowed user IDs.

## Operations

```bash
mise run telegram-agent:run
mise run telegram-agent:build
mise run telegram-agent:notify -- "PR is ready for review."
mise run pr:watch-ready -- --repo caboose-ai/caboose-ai.io --pr 48 --poll 1m --timeout 10m
```

Supported Telegram commands:

```text
/ask <prompt>
/agent <role> <task>
/agent confirm <role> <task>
/model
/model <provider/model>
/status
/lab status
/lab docker
/lab service <slug>
/lab ask <prompt>
```

`/agent <role> <task>` drafts a role-scoped plan. The prompt requires intended
commands/tools, affected files/services, rollback notes, verification, and the
required confirmation phrase before any execution.

`/agent confirm <role> <task>` is only a short remote confirmation signal. It is
not durable approval for broad Paperclip execution. The agent prompt still
requires human approval before restarts, deploys, destructive operations, or
other runtime mutations.

Agent plans should prefer self-hosted delivery first: Forgejo/Gitea branches or
PRs where applicable, Woodpecker CI for validation, and Docker inspection
through Portainer, MCP, or CLI before changing runtime state.

`/lab` commands use the local Homelab MCP server over stdio. By default the bot
generates a temporary MCP config from `HOMELAB_*` environment values and runs
`go run -buildvcs=false ./cmd/mcp --config <tempfile>`. Set
`HOMELAB_MCP_CONFIG`, `HOMELAB_MCP_BINARY`, or `HOMELAB_MCP_WORKDIR` to pin the
MCP runtime path. The supported MCP surface is intentionally narrow: full-stack
health, Docker listing, per-service status resources, and `agent_invoke`.

## PR Readiness Notifications

`cmd/pr-ready-watch` polls GitHub through the local `gh` CLI and sends a
Telegram notification when a PR has a completed Codex review, no failing or
pending checks, no requested changes, and no merge conflict. Draft PRs are not
blocked: the notification labels them as ready for final human review so the
human remains last in the loop. If an open PR becomes blocked by failing
checks, requested changes, or a merge conflict, the watcher sends the blocked
findings instead of waiting silently.

The watcher can read the Telegram bot token from 1Password if
`TELEGRAM_BOT_TOKEN` is unset:

```bash
TELEGRAM_BOT_TOKEN_OP_ITEM="Telegram Homelab Bot Token" \
TELEGRAM_BOT_TOKEN_OP_VAULT=Personal \
TELEGRAM_ALLOWED_USER_IDS=123456789 \
TELEGRAM_NOTIFY_CHAT_ID=123456789 \
mise run pr:watch-ready -- --repo caboose-ai/caboose-ai.io --pr 48 --poll 1m --timeout 10m
```

Use `--once --dry-run` to print the current assessment without notifying. For a
local timer or cron job, run the same command from the repository root. A hosted
webhook is only needed if GitHub should call a public callback instead of this
host polling with local credentials.

## Service Contract

Good:

```yaml
runtime: external
compose_services: []
dashboard:
  show: false
sso:
  mode: telegram-allowlist
```

Bad:

```yaml
compose_services:
  - telegram-agent
# The bot needs host OpenClaw CLI credentials; a fake compose service would not
# verify the same runtime path.
```
