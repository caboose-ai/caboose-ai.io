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
```

`TELEGRAM_NOTIFY_CHAT_ID` can narrow one-shot notifications to a specific chat
ID. If it is unset, `telegram-agent notify` sends to all allowed user IDs.

## Operations

```bash
mise run telegram-agent:run
mise run telegram-agent:build
mise run telegram-agent:notify -- "PR is ready for review."
```

Supported Telegram commands:

```text
/ask <prompt>
/agent <role> <task>
/agent confirm <role> <task>
/model
/model <provider/model>
/status
```

Write, deploy, restart, commit, push, merge, reset, and delete-style agent
tasks require `/agent confirm ...` when `TELEGRAM_REQUIRE_CONFIRMATION` is true.

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
