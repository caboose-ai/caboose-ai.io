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
| Ghost | `blog.caboose-ai.io` | Blog |
| Homarr | `caboose-ai.io` | Dashboard / homepage |
| Prometheus | — | Metrics collection |
| Loki + Promtail | — | Log aggregation |

All services authenticate through Authentik via OAuth2/OIDC or forward-auth proxy.

## Binaries

- **`cmd/homelab`** — Bubbletea TUI installer that bootstraps the entire stack: generates secrets, starts containers, provisions OAuth providers, configures each service.
- **`cmd/mcp`** — MCP server exposing homelab tools to AI assistants.

## Quick start

```bash
# Build
go build ./...

# Run the installer (interactive TUI)
go run ./cmd/homelab install --domain caboose-ai.io --compose-dir dev/homelab

# Or non-interactive
go run ./cmd/homelab install --non-interactive --config homelab.yml

# Reset everything
go run ./cmd/homelab reset          # full reset
go run ./cmd/homelab reset --keep-env  # keep .env file

# Migrate host Mattermost to Docker
go run ./cmd/homelab migrate
```

## Testing

```bash
go test ./internal/... -v           # unit tests

# SSO smoke tests (requires live stack running)
mise run sso:check                  # full suite: config + endpoints + browser login
mise run sso:check-quick            # API config checks only
```

## Infrastructure

- **Caddy** reverse proxy on the host — handles TLS and routes to containers
- **Docker Compose** at `dev/homelab/docker-compose.yml` — all services
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
