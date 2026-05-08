# CLAUDE.md — caboose-ai.io

## Project

Go monorepo for a homelab SSO infrastructure stack. Two binaries:
- `cmd/homelab` — Bubbletea TUI installer that bootstraps Authentik SSO + all services
- `cmd/mcp` — MCP server exposing homelab tools to AI assistants
  - `agent_invoke` supports provider fallback: Ollama, Claude Code, Copilot CLI, Emberfall

All packages under `internal/`. No public API.

## Build & Test

```bash
go build ./...                    # build everything
go test ./internal/... -v         # run all tests
go test ./internal/install/ -v    # test a specific package
go run ./cmd/homelab oauth-setup --domain caboose-ai.io  # print external OAuth/Turnstile setup
CLOUDFLARE_ACCOUNT_ID=... CLOUDFLARE_API_TOKEN=... go run ./cmd/homelab oauth-setup --domain caboose-ai.io --create-turnstile  # create Turnstile via API
mise run sso:check                # full SSO smoke tests (requires live stack)
mise run sso:check-quick          # API config checks only
```

Go binary is at `/home/caboose/.local/go/bin/go`. If `go` isn't on PATH, use `PATH="/home/caboose/.local/go/bin:$PATH"`.

## Architecture

- `internal/config/` — Config structs, YAML parsing, URL derivation
- `internal/secrets/` — SecretStore interface with 1Password + .env backends
- `internal/install/` — Installer orchestration, one file per concern
- `internal/services/<name>/` — Per-service configurators implementing `ServiceConfigurator`
- `internal/services/authentik/` — Authentik API client, one file per resource type
- `internal/paperclip/` — Paperclip bootstrap client and software-shop seed profile
- `internal/tui/` — Bubbletea TUI with message-driven phase transitions
- `internal/mcp/` — MCP server, tools, resources
- `internal/runner/` — CommandRunner + HTTPClient interfaces for testability
- `internal/migrate/` — Host-to-Docker migration orchestrators (e.g. Mattermost)
- `internal/smoketest/` — Integration smoke tests (API config checks + headless browser login flows)
- `dev/homelab/` — Docker Compose, Prometheus/Loki/Promtail config, Grafana dashboards, Authentik patches

The installer includes first-run configurators for services that otherwise stop
at setup or product-local login screens. SonarQube, n8n, and Mattermost use
managed admin credentials from the split 1Password store or `.env` fallback;
Mattermost compose enables local mode so the configurator can verify the
bootstrap state without interactive browser setup. Woodpecker stores server
state at `/var/lib/woodpecker` to preserve OAuth/session configuration across
container recreation.

## Conventions

### Code Style
- Interfaces for testability: `SecretStore`, `CommandRunner`, `HTTPClient`
- `ServiceConfigurator` interface for all services: `Name()`, `Slug()`, `CheckConfigured()`, `Configure()`
- Return errors up the stack. TUI/CLI handles display.
- No global state — everything flows through `Installer` struct or Bubbletea model.
- One responsibility per file. Split by domain, not layer.

### Naming
- Secret keys: `SCREAMING_SNAKE_CASE`
- Service slugs: kebab-case matching Authentik slugs
- Config YAML: snake_case
- Go types: standard Go conventions

### Testing
- Tests live next to code (`foo_test.go`)
- Table-driven tests for multiple cases
- Mock via interfaces (see `runner/mock.go`)
- Integration smoke tests in `internal/smoketest/` use build tag `integration` and require a live stack
- Browser smoke tests exercise both Authentik/OIDC login and service-specific
  handoffs, including Portainer's OAuth button, Mattermost's browser handoff and
  local admin login, and proxy-gated landing checks for Woodpecker, n8n,
  Homarr, OpenClaw, and Paperclip.

### Git
- Never commit to main. Always branch first.
- Commit format: `type(scope): description`
- Types: feat, fix, test, docs, chore, refactor

### Documentation
- Every PR to main that changes code or infrastructure MUST update docs.
- Files to keep in sync: `CLAUDE.md`, `.github/copilot-instructions.md`, `README.md`
- `CLAUDE.md` — architecture map, conventions, service list, "Adding a New Service" checklist
- `.github/copilot-instructions.md` — mirrors CLAUDE.md for GitHub Copilot
- `README.md` — project overview, service table, setup instructions for humans
- CI enforces this via `.github/workflows/docs-check.yml`. Add `docs-exempt` label to skip.
- When adding a service: update the Architecture section, Live Environment services list, and README service table.
- When adding a package: update the Architecture section in all three files.

### Docker Compose
- `dev/homelab/docker-compose.yml` — all services
- Secrets via `${VAR}` from `.env`
- DB networks: `*-internal` with `internal: true`
- App networks: `apps`
- Ports: `127.0.0.1` only — Caddy handles external

### Authentik
- OAuth2 and proxy providers must have matching Authentik applications.
- Authentik application lookups must exact-match slugs; API search responses can include broader matches.
- Installer-created or repaired applications should use `policy_engine_mode=all` so provider-level SSO policy gates access.
- Shared OAuth source upserts should leave `promoted` unset unless the caller intentionally owns promotion/demotion.
- Use `homelab oauth-setup --domain <domain>` to print GitHub/Google OAuth callback URLs, Turnstile hostname, and expected secret keys. Add `--create-turnstile` with `CLOUDFLARE_ACCOUNT_ID` and `CLOUDFLARE_API_TOKEN` to create the Turnstile widget through Cloudflare's API.

## Adding a New Service

1. Compose entry in `dev/homelab/docker-compose.yml`
2. Caddy site in `/etc/caddy/Caddyfile`
3. Authentik provider (proxy for SSO gate, OAuth2 for native OIDC)
4. Service configurator in `internal/services/<name>/`
5. Register in `install/install.go` `BuildServices()`
6. Secrets in `secrets/store.go` if needed
7. Add or update `internal/smoketest/flows.go` when the service has native
   login, first-run setup, or proxy-gated landing behavior that must be proven
   by the live SSO browser test.
8. For Paperclip-style control-plane services, add or update `services/<slug>/service.yaml` and any seed/context document.

## Live Environment

- Domain: `caboose-ai.io`
- Auth: `auth.caboose-ai.io` (Authentik)
- Services: git, ci, docker, grafana, ai, n8n, sonar, chat, openclaw, blog, paperclip, dash
- Observability: Prometheus (metrics), Loki + Promtail (logs), Grafana (dashboards)
- Caddy reverse proxy on the host
- Cloudflare tunnel for `chat` and `sonar` subdomains
- Ollama on host for local LLM inference
- Paperclip runs behind the compose `paperclip` profile with Authentik forward-auth provider `paperclip-proxy`; seed it with `homelab paperclip seed-company --profile software-shop --repo /home/caboose/dev/caboose-ai.io`.
