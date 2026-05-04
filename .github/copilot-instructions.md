# Copilot Instructions — caboose-ai.io

## Project Overview

Go monorepo for a homelab SSO infrastructure stack. Two binaries:
- `cmd/homelab` — TUI installer that bootstraps Authentik SSO + all services
- `cmd/mcp` — MCP server exposing homelab tools to AI assistants

All packages live under `internal/`. No public API surface.

## Architecture

```
cmd/homelab/main.go     CLI entry point, flag parsing, subcommands
cmd/mcp/main.go         MCP server entry point

internal/
  config/               Config structs, YAML parsing, URL derivation
  secrets/              SecretStore interface, 1Password + .env backends
  prereq/               Prerequisite checks (docker, op, etc.)
  install/              Installer orchestration, service provisioning
  docker/               Docker Compose and exec wrappers
  runner/               CommandRunner + HTTPClient interfaces (for testability)
  health/               Health polling for services
  tui/                  Bubbletea TUI (app model, views, components, styles)
  services/             Per-service configurators (forgejo, grafana, etc.)
    authentik/          Authentik API client (providers, sources, outpost)
  mcp/                  MCP server, tools, resources
  cli/                  Non-interactive CLI runner
  migrate/              Host-to-Docker migration orchestrators
  smoketest/            Integration smoke tests (API + browser, build tag: integration)
  orchestrator/         Backend abstraction (compose, kubernetes)
```

## Conventions

### Go Patterns

- **Interfaces for testability**: `SecretStore`, `CommandRunner`, `HTTPClient` — inject via struct fields, mock in tests.
- **`ServiceConfigurator` interface**: Every service implements `Name()`, `Slug()`, `CheckConfigured()`, `Configure()`. New services follow this pattern.
- **Installer state machine**: Phases defined in `install/state.go`. TUI drives phase transitions via Bubbletea messages in `tui/app.go`.
- **Error handling**: Return errors up the stack, don't log-and-continue. The TUI/CLI handles display.
- **No global state**: All state flows through the `Installer` struct or Bubbletea model.

### File Organization

- One responsibility per file. Split by domain, not by technical layer.
- Service configurators go in `internal/services/<name>/<name>.go`.
- Authentik API methods go in `internal/services/authentik/` — one file per resource type (providers.go, sources.go, outpost.go).
- Install orchestration steps go in `internal/install/` — one file per concern (providers.go, outpost.go, social.go, forgejo.go).

### Testing

- Tests live next to the code they test (`foo_test.go` alongside `foo.go`).
- Use table-driven tests where there are multiple cases.
- Mock external dependencies via interfaces (see `runner/mock.go`, test mocks in `install/social_test.go`).
- Integration smoke tests in `internal/smoketest/` use build tag `integration` and require a live stack.
- Run tests: `go test ./internal/... -v`
- Run smoke tests: `mise run sso:check` (full) or `mise run sso:check-quick` (API only)

### Naming

- Secret keys: `SCREAMING_SNAKE_CASE` (e.g., `GITHUB_OAUTH_CLIENT_ID`)
- Go types: standard Go naming (CamelCase exported, camelCase unexported)
- Service slugs: kebab-case matching Authentik application slugs
- Config YAML keys: snake_case

### Docker Compose

- Services in `dev/homelab/docker-compose.yml`
- Secrets via `${VAR}` env var substitution from `.env` file
- Internal-only DB networks (`*-internal` with `internal: true`)
- App-facing services join the `apps` network
- Ports bind to `127.0.0.1` only — Caddy reverse proxies externally

### Git Workflow

- Never commit directly to main. Always create a feature branch.
- Commit messages: `type(scope): description` (e.g., `feat(install):`, `fix(homelab):`, `test(install):`)
- Types: feat, fix, test, docs, chore, refactor

## Documentation

- Every PR to main that changes code or infrastructure MUST update docs.
- Files to keep in sync: `CLAUDE.md`, `.github/copilot-instructions.md`, `README.md`
- CI enforces this via `.github/workflows/docs-check.yml`. Add `docs-exempt` label to skip.
- When modifying a service configurator, verify the corresponding compose entry still matches.
- When adding a new service: update Architecture sections, service lists, and README service table.
- When adding a new package: update the Architecture tree in all three doc files.

## Common Tasks

### Adding a new service to the stack

1. Add Docker service to `dev/homelab/docker-compose.yml`
2. Add Caddy reverse proxy entry to `/etc/caddy/Caddyfile`
3. If SSO-gated (no native OIDC): create proxy provider + bind to outpost
4. If app-level OIDC: create OAuth2 provider + application in Authentik
5. Create service configurator in `internal/services/<name>/`
6. Register in `install/install.go` `BuildServices()`
7. Add any secrets to `secrets/store.go` `BootstrapSecrets()`

### Adding a new Authentik API method

1. Add to appropriate file in `internal/services/authentik/` (or create new file for new resource type)
2. Follow existing patterns: define request/response structs, use `c.Get()`/`c.Post()`/`c.Patch()`
3. The `Client` struct handles auth headers and error checking

### Running the installer

```bash
go run ./cmd/homelab install --domain caboose-ai.io --compose-dir dev/homelab
```

### Building

```bash
go build ./...          # build all
go test ./internal/...  # test all
```
