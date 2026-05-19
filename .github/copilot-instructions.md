# Copilot Instructions — caboose-ai.io

## Project Overview

Go monorepo for a homelab SSO infrastructure stack. Four command entrypoints:
- `cmd/homelab` — TUI installer that bootstraps Authentik SSO + all services
- `cmd/mcp` — MCP server exposing homelab tools to AI assistants
  - `agent_invoke` provider fallback supports Ollama, Claude Code, Copilot CLI, and Emberfall
- `cmd/telegram-agent` — private Telegram bot for allowlisted OpenClaw-backed agent control and narrow `/lab` Homelab MCP queries
- `cmd/pr-ready-watch` — local GitHub PR watcher that notifies Telegram when Codex review and checks are ready for final human review

Service implementation packages live under `services/<slug>/`; shared internal
packages live under `internal/`. No public API surface.

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
  service/              Shared service manifest, registry, and configurator types
  servicebuilder/       Central service configurator construction
  paperclip/            Paperclip bootstrap client, local_trusted proxy mode, and software-shop seed profile
  prwatch/              PR readiness classifier for Codex review, checks, requested changes, and merge state
  telegrambot/          Telegram allowlist bot and OpenClaw model command bridge
  mcp/                  MCP server, tools, resources
  cli/                  Non-interactive CLI runner
  migrate/              Host-to-Docker migration orchestrators
  smoketest/            Integration smoke tests (API + browser, build tag: integration)
  orchestrator/         Backend abstraction (compose, kubernetes)

services/
  <slug>/               Service workspace with service.yaml, README.md, and optional Go configurator package
  authentik/            Authentik API client (providers, sources, outpost)
```

The installer performs first-run setup for services that would otherwise block
SSO validation behind setup or local product login screens. SonarQube and
Mattermost use managed admin credentials from the split 1Password store or
`.env` fallback; Mattermost local mode is enabled in compose for repeatable
bootstrap verification. Woodpecker persists server data at
`/var/lib/woodpecker` so OAuth/session state survives container recreation.

Paperclip-driven agent control starts as a planner/audit ledger. Implementation
work should prefer the self-hosted delivery loop: Forgejo/Gitea branches and
PRs, Woodpecker CI, and Portainer Docker visibility. Confirmed execution routes
through approved OpenClaw, Telegram, MCP, Homelab CLI, PR, or Portainer
handoffs; Docker mutations, deploys, restarts, and destructive operations
require explicit human approval.
Seeded Paperclip workspaces carry concrete delivery metadata: `paperclip/*`
branches go to the `forgejo` remote, review happens as Forgejo pull requests,
Woodpecker at `https://ci.caboose-ai.io` is the CI evidence source, and
Portainer at `https://docker.caboose-ai.io` is inspection-only unless a human
approves a runtime mutation.

## Conventions

### Go Patterns

- **Interfaces for testability**: `SecretStore`, `CommandRunner`, `HTTPClient` — inject via struct fields, mock in tests.
- **`ServiceConfigurator` interface**: Every service implements `Name()`, `Slug()`, `CheckConfigured()`, `Configure()`. New services follow this pattern.
- **Service manifest registry**: `services/<slug>/service.yaml` backs per-service CLI and MCP resources.
- **Installer state machine**: Phases defined in `install/state.go`. TUI drives phase transitions via Bubbletea messages in `tui/app.go`.
- **Error handling**: Return errors up the stack, don't log-and-continue. The TUI/CLI handles display.
- **No global state**: All state flows through the `Installer` struct or Bubbletea model.

### File Organization

- One responsibility per file. Split by domain, not by technical layer.
- Service configurators go in `services/<slug>/<name>.go`.
- Authentik API methods go in `services/authentik/` — one file per resource type (providers.go, sources.go, outpost.go).
- Install orchestration steps go in `internal/install/` — one file per concern (providers.go, outpost.go, social.go, forgejo.go).

### Testing

- Tests live next to the code they test (`foo_test.go` alongside `foo.go`).
- Use table-driven tests where there are multiple cases.
- Mock external dependencies via interfaces (see `runner/mock.go`, test mocks in `install/social_test.go`).
- Integration smoke tests in `internal/smoketest/` use build tag `integration` and require a live stack.
- Browser smoke tests cover native Authentik/OIDC login, service-specific
  handoffs such as Portainer's OAuth button and Mattermost's browser handoff
  plus local admin login, Homarr native Authentik/OIDC login, and proxy-gated
  landing checks for Woodpecker, OpenClaw, and Paperclip.
- `homelab service <slug> smoke` runs the manifest-owned `smoke_flow`; leave
  health-only services without a flow.
- Run tests: `go test ./...`
- Run format checks: `mise run fmt`
- Run lint checks: `mise run lint`
- Run vulnerability checks: `mise run vulncheck`
- Run smoke tests: `mise run sso:check` (full) or `mise run sso:check-quick` (API only)
- Run CAB Tier 2/3 lane: `mise run cab:tier23-live` (writes evidence under `docs/evidence/cab-8/`)
- Run MCP endpoint checks: `mise run mcp:probe` locally and `mise run mcp:probe-external` publicly
- Run MCP access setup via `homelab mcp access setup`, approve client blobs with `homelab mcp access approve`, and request/import/token from `homelab-mcp access`.
- Diagnose Portainer admin-password drift with `mise run portainer:recover-access`;
  only run `dev/homelab/portainer-recover-access.sh --yes` after explicit human
  approval for the local Portainer reset path.

### Naming

- Secret keys: `SCREAMING_SNAKE_CASE` (e.g., `GITHUB_OAUTH_CLIENT_ID`)
- Go types: standard Go naming (CamelCase exported, camelCase unexported)
- Service slugs: kebab-case matching Authentik application slugs
- Config YAML keys: snake_case

### Docker Compose

- Services in `dev/homelab/docker-compose.yml`
- Secrets via `${VAR}` env var substitution from `.env` file
- Internal-only DB networks (`*-internal` with `internal: true`), except
  Paperclip's private DB bridge which must allow Docker to publish the
  loopback-only DB port used by the host-network Paperclip app.
- Paperclip's public URL remains Authentik-gated; host-network Paperclip agents
  use `PAPERCLIP_API_URL=http://127.0.0.1:3100` for local API calls.
- App-facing services join the `apps` network
- Mattermost Team Edition must not be published or scraped as a Prometheus
  metrics target; its profiling listener does not expose HTTP `/metrics`.
- Homarr is pinned to `ghcr.io/homarr-labs/homarr:v1.61.0`, persists `/appdata`
  in `homarr_data`, and uses native Authentik OIDC for the root homepage.
- Authentik mounts `authentik_data:/data` in both server and worker containers
  so uploaded media and runtime-managed files survive container recreation.
- Port exposure is configurable with `serve_mode` / `--serve-mode`:
  `public` binds host ports to `127.0.0.1` for Caddy reverse proxying,
  `local` binds to `0.0.0.0` for LAN access.

### Authentik

- OAuth2 and proxy providers must have matching Authentik applications.
- Exact-match application slugs after Authentik API searches; broad search results can include unrelated apps.
- Installer-created or repaired applications should use `policy_engine_mode=all` so provider-level SSO policy gates access.
- Managed social login is Google-only in the browser: Google is enabled with `email_link` matching, local email/password fields are hidden on the default authentication flow, and the GitHub OAuth source is kept disabled/unpromoted even when credentials exist.
- Shared OAuth source upserts should leave `promoted` unset unless the caller intentionally owns promotion/demotion.
- Use `homelab oauth-setup --domain <domain>` to print GitHub/Google OAuth callback URLs, Turnstile hostname, and expected secret keys. Add `--create-turnstile` with `CLOUDFLARE_ACCOUNT_ID` and `CLOUDFLARE_API_TOKEN` to create the Turnstile widget through Cloudflare's API.

### Git Workflow

- Never commit directly to main. Always create a feature branch.
- Commit messages: `type(scope): description` (e.g., `feat(install):`, `fix(homelab):`, `test(install):`)
- Types: feat, fix, test, docs, chore, refactor
- PR titles targeting `main` should follow Conventional Commits. CI validates
  the title before merge and accepts legacy `[type] description` titles during
  transition.
- Release Please uses conventional commits from `main` to open release PRs and
  publish GitHub releases. Starting version is `v0.1.0`; `fix:` is patch,
  `feat:` is minor, and `!` or `BREAKING CHANGE:` is major.
- `.github/workflows/release-please-automerge.yml` approves and squash-merges
  Release Please PRs after `Conventional PR title`, `Test and build`,
  `docs-check`, `Gitleaks Secret Scan`, and `lint` pass. Use
  `RELEASE_PLEASE_TOKEN` for the merge so the release publication workflow is
  triggered by a normal token-backed push.
- Published releases trigger `.github/workflows/update-homebrew-tap.yml`, which
  uses the `HOMEBREW_TAP_TOKEN` Actions secret to update
  `caboose-ai/homebrew-tap` formulae, install and `brew test` both updated
  formulae, rerun formula validation before deploy, and push the verified
  formula commit directly to the tap.
- CI builds `cmd/homelab` and `cmd/mcp` with `-buildvcs=false` so release
  checks do not depend on Go VCS stamping.
- Direct `homelab reset` requires `--yes` unless `--dry-run`; live Docker,
  SQLite, secret, reset, and destructive filesystem changes require explicit
  human approval.
- `homelab reset --store-static-env` copies static external `.env` values
  (GitHub, Google, Turnstile, Cloudflare) to the static 1Password vault before
  teardown, then removes `.env`; install restores those static values from the
  secret store into a fresh `.env`.

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
5. Create `services/<slug>/service.yaml` and `services/<slug>/README.md`
6. Create optional service configurator in `services/<slug>/`
7. Register configurator construction in `internal/servicebuilder`
8. Add any secrets to `secrets/store.go` `BootstrapSecrets()`
9. Update `internal/smoketest/flows.go` when the service needs a browser proof
   for native login, first-run setup, or proxy-gated landing behavior.
10. Add manifest `dashboard` and `sso` metadata so Homarr and docs follow the
    service contract instead of a hard-coded inclusion list.
11. Add or update `services/<slug>/service.yaml` when the service participates in the service-workspace surface.

### Adding a new Authentik API method

1. Add to appropriate file in `services/authentik/` (or create new file for new resource type)
2. Follow existing patterns: define request/response structs, use `c.Get()`/`c.Post()`/`c.Patch()`
3. The `Client` struct handles auth headers and error checking

### Running the installer

```bash
go run ./cmd/homelab oauth-setup --domain caboose-ai.io
CLOUDFLARE_ACCOUNT_ID=... CLOUDFLARE_API_TOKEN=... go run ./cmd/homelab oauth-setup --domain caboose-ai.io --create-turnstile
go run ./cmd/homelab reset --yes --store-static-env --domain caboose-ai.io --compose-dir /opt/homelab
go run ./cmd/homelab install --domain caboose-ai.io --compose-dir dev/homelab
go run ./cmd/homelab service --domain caboose-ai.io --compose-dir dev/homelab forgejo status
mise run paperclip:seed
mise run portainer:recover-access
go run ./cmd/telegram-agent notify "task finished"
go run ./cmd/mcp access request --name "codex on laptop" --out mcp-request.json
go run ./cmd/homelab mcp access setup
go run ./cmd/homelab mcp access approve mcp-request.json --out mcp-release.json
mise run mcp:setup-external
dev/homelab/mcp-access-live.sh --name "codex on laptop"
dev/homelab/mcp-test-live.sh
go run ./cmd/pr-ready-watch --repo caboose-ai/caboose-ai.io --pr 48 --poll 1m --timeout 10m
```

### Building

```bash
go build ./...          # build all
go test ./...           # test all
```

## Security

- Do not commit `.env` or live secrets. Prefer 1Password or local untracked files.
- Example variables live in `dev/homelab/.env.example`.
- CI scans new push and PR commit ranges with Gitleaks.
- If any secret lands in git history, rotate it immediately.
