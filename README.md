# caboose-ai.io

Go monorepo for a self-hosted homelab infrastructure stack with SSO via Authentik.

## What's in the stack

| Service | Subdomain | Description |
|---------|-----------|-------------|
| Authentik | `auth.caboose-ai.io` | SSO identity provider |
| Forgejo | `git.caboose-ai.io` | Git hosting (Gitea fork) |
| Woodpecker CI | `ci.caboose-ai.io` | CI/CD pipelines |
| Portainer | `docker.caboose-ai.io` | Docker management UI |
| Grafana | `grafana.caboose-ai.io` | Dashboards & observability |
| Open WebUI | `ai.caboose-ai.io` | LLM chat interface (Ollama) |
| n8n | `n8n.caboose-ai.io` | Workflow automation |
| SonarQube | `sonar.caboose-ai.io` | Code quality & security |
| Mattermost | `chat.caboose-ai.io` | Team chat |
| OpenClaw | `openclaw.caboose-ai.io` | OpenClaw app behind Authentik forward auth |
| Ghost | `blog.caboose-ai.io` | Blog |
| Paperclip | `paperclip.caboose-ai.io` | AI-labor control plane for the homelab software shop |
| Homarr | `caboose-ai.io` | Dashboard / homepage |
| Prometheus | — | Metrics collection |
| Loki + Promtail | — | Log aggregation |

All services authenticate through Authentik via OAuth2/OIDC or forward-auth proxy. The installer keeps Authentik applications open (`policy_engine_mode=all`) so provider-level SSO decisions control access consistently. The social login configurator promotes managed GitHub and Google sources, while generic source upserts preserve the existing promotion state unless explicitly changed.

The installer also completes first-run setup for services that expose a product
setup or local-login boundary before they can participate in the SSO smoke
suite. SonarQube, n8n, and Mattermost are configured with managed admin
credentials from 1Password or the compose `.env` fallback, and Mattermost local
mode is enabled in compose for repeatable bootstrap checks. Woodpecker keeps its
server data under `/var/lib/woodpecker` so OAuth/session setup survives
container recreation.

## Binaries

- **`cmd/homelab`** — Bubbletea TUI installer that bootstraps the entire stack: generates secrets, starts containers, provisions OAuth providers, configures each service.
  - Includes `homelab service <slug> <status|configure|logs|smoke|open>` for per-service operations backed by `services/<slug>/service.yaml`.
- **`cmd/mcp`** — MCP server exposing homelab tools to AI assistants.
  - Includes `agent_invoke` provider fallback across Ollama, Claude Code, Copilot CLI, and Emberfall.

## Service workspaces

Each service has a root-level workspace under `services/<slug>/`. Configured
services keep their Go configurator package there, and operational-only
services keep a manifest and README so CLI, MCP, docs, and smoke flows share
one registry. The shared `ServiceConfigurator` contract lives in
`internal/service`.

## Quick start

```bash
# Build
mise run build

# Build the homelab CLI
mise run homelab:build

# Run the verified non-interactive installer
mise run install

# Run the interactive TUI installer
mise run homelab

# Reset everything while preserving static external credentials
mise run reset

# Reset, then run the verified non-interactive installer
mise run reinstall

# Print GitHub, Google, and Turnstile setup URLs/callbacks
mise run homelab:oauth-setup

# Create a Cloudflare Turnstile widget via API, then print all setup values
mise run homelab:create-turnstile

# Per-service operations
mise run service:status -- forgejo
mise run service:configure -- mattermost --dry-run
mise run service:smoke -- forgejo

# Seed the Paperclip software-shop company
mise run paperclip:seed

# Migrate host Mattermost to Docker
go run ./cmd/homelab migrate
```

The homelab tasks default to `HOMELAB_DOMAIN=caboose-ai.io`, `HOMELAB_COMPOSE_DIR=/opt/homelab`, and `HOMELAB_SERVE_MODE=public`.
Override variables inline when needed, for example:

```bash
HOMELAB_COMPOSE_DIR=dev/homelab mise run install
HOMELAB_SERVE_MODE=local mise run install
```

`serve_mode` controls host port exposure. `public` binds compose ports to
`127.0.0.1` for Caddy/TLS reverse proxying. `local` binds them to `0.0.0.0`
for LAN access while keeping the same service URLs and Authentik callback
configuration.

## Testing

```bash
go test ./...                       # unit tests

# SSO smoke tests (requires live stack running)
mise run sso:check                  # full suite: config + endpoints + browser login
mise run sso:check-quick            # API config checks only
mise run sso:e2e                    # browser SSO with click/input screenshot evidence
mise run homelab:e2e-reset          # destructive reset + install + E2E evidence
```

Browser E2E runs write an action log and screenshots under
`internal/smoketest/testdata/evidence/`. Each action records the page URL and
whether the test opened a page, clicked a control, entered text, or reached a
service. Password values are redacted from the evidence log.

The browser flow covers both native Authentik/OIDC redirects and service
specific first-run paths. Portainer clicks its visible OAuth login control,
Mattermost follows the browser handoff and uses the managed local admin account,
and proxy-gated services such as Woodpecker, n8n, Homarr, and OpenClaw are
validated by reaching their protected landing URLs.

## Infrastructure

- **Caddy** reverse proxy on the host — handles TLS and routes to containers in `public` serve mode
- **Docker Compose** at `dev/homelab/docker-compose.yml` — all services
- **Paperclip** profile (`docker compose --profile paperclip ...`) — built from upstream tag `v2026.428.0`, backed by `paperclip-db`, and Authentik-gated through `paperclip-proxy`
- **Cloudflare tunnel** for `chat` and `sonar` subdomains
- **1Password** for secret storage (with `.env` fallback)
- **Prometheus + Loki** for metrics and logs, visualized in Grafana

## Documentation

Three files are kept in sync and must be updated on every PR to main:

| File | Audience |
|------|----------|
| `CLAUDE.md` | Claude Code / AI assistants |
| `.github/copilot-instructions.md` | GitHub Copilot |
| `README.md` | Humans |

CI checks this via `.github/workflows/docs-check.yml`. Add the `docs-exempt` label to skip.
