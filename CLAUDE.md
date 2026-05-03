# CLAUDE.md — caboose-ai.io

## Project

Go monorepo for a homelab SSO infrastructure stack. Two binaries:
- `cmd/homelab` — Bubbletea TUI installer that bootstraps Authentik SSO + all services
- `cmd/mcp` — MCP server exposing homelab tools to AI assistants

All packages under `internal/`. No public API.

## Build & Test

```bash
go build ./...                    # build everything
go test ./internal/... -v         # run all tests
go test ./internal/install/ -v    # test a specific package
```

Go binary is at `/home/caboose/.local/go/bin/go`. If `go` isn't on PATH, use `PATH="/home/caboose/.local/go/bin:$PATH"`.

## Architecture

- `internal/config/` — Config structs, YAML parsing, URL derivation
- `internal/secrets/` — SecretStore interface with 1Password + .env backends
- `internal/install/` — Installer orchestration, one file per concern
- `internal/services/<name>/` — Per-service configurators implementing `ServiceConfigurator`
- `internal/services/authentik/` — Authentik API client, one file per resource type
- `internal/tui/` — Bubbletea TUI with message-driven phase transitions
- `internal/mcp/` — MCP server, tools, resources
- `internal/runner/` — CommandRunner + HTTPClient interfaces for testability
- `dev/homelab/` — Docker Compose, Prometheus config, Authentik patches

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

### Git
- Never commit to main. Always branch first.
- Commit format: `type(scope): description`
- Types: feat, fix, test, docs, chore, refactor

### Docker Compose
- `dev/homelab/docker-compose.yml` — all services
- Secrets via `${VAR}` from `.env`
- DB networks: `*-internal` with `internal: true`
- App networks: `apps`
- Ports: `127.0.0.1` only — Caddy handles external

## Adding a New Service

1. Compose entry in `dev/homelab/docker-compose.yml`
2. Caddy site in `/etc/caddy/Caddyfile`
3. Authentik provider (proxy for SSO gate, OAuth2 for native OIDC)
4. Service configurator in `internal/services/<name>/`
5. Register in `install/install.go` `BuildServices()`
6. Secrets in `secrets/store.go` if needed

## Live Environment

- Domain: `caboose-ai.io`
- Auth: `auth.caboose-ai.io` (Authentik)
- Services: git, ci, docker, grafana, ai, n8n, sonar, chat, openclaw, blog
- Caddy reverse proxy on the host
- Cloudflare tunnel for `chat` and `sonar` subdomains
- Ollama on host for local LLM inference
