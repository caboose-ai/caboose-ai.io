# CLAUDE.md — caboose-ai.io

## Project

Go monorepo for a homelab SSO infrastructure stack. Four command entrypoints:
- `cmd/homelab` — Bubbletea TUI installer that bootstraps Authentik SSO + all services
- `cmd/mcp` — MCP server exposing homelab tools to AI assistants
  - `agent_invoke` supports provider fallback: Ollama, Claude Code, Copilot CLI, Emberfall
- `cmd/telegram-agent` — private Telegram bot for allowlisted OpenClaw-backed agent control and narrow `/lab` Homelab MCP queries
- `cmd/pr-ready-watch` — local GitHub PR watcher that notifies Telegram when Codex review and checks are ready for final human review

Service implementation packages live under `services/<slug>/`; shared internal
packages remain under `internal/`. No public API.

## Build & Test

```bash
go build ./...                    # build everything
go test ./...                     # run all tests
go test ./internal/install/ -v    # test a specific package
go run ./cmd/homelab oauth-setup --domain caboose-ai.io  # print external OAuth/Turnstile setup
go run ./cmd/homelab service --domain caboose-ai.io forgejo status  # inspect one service
go run ./cmd/telegram-agent notify "task finished"  # send a Telegram completion notice
go run ./cmd/mcp access request --name "codex on laptop" --out mcp-request.json  # create MCP access request
go run ./cmd/homelab mcp access setup  # create MCP OAuth provider/scope
go run ./cmd/homelab mcp access approve mcp-request.json --out mcp-release.json  # approve MCP client access
mise run mcp:probe-external       # verify the public Homelab MCP endpoint
mise run mcp:setup-external       # upsert MCP DNS, install Caddy route, then verify
dev/homelab/mcp-access-live.sh --name "codex on laptop"  # create/import/smoke-test live MCP access
dev/homelab/mcp-test-live.sh     # smoke-test the live MCP endpoint with existing access
go run ./cmd/pr-ready-watch --repo caboose-ai/caboose-ai.io --pr 48 --poll 1m --timeout 10m  # watch a PR for human-review readiness
CLOUDFLARE_ACCOUNT_ID=... CLOUDFLARE_API_TOKEN=... go run ./cmd/homelab oauth-setup --domain caboose-ai.io --create-turnstile  # create Turnstile via API
mise run sso:check                # full SSO smoke tests (requires live stack)
mise run sso:check-quick          # API config checks only
```

Go binary is at `/home/caboose/.local/go/bin/go`. If `go` isn't on PATH, use `PATH="/home/caboose/.local/go/bin:$PATH"`.

## Architecture

- `internal/config/` — Config structs, YAML parsing, URL derivation
- `internal/secrets/` — SecretStore interface with 1Password + .env backends
- `internal/install/` — Installer orchestration, one file per concern
- `internal/service/` — Shared service manifest, registry, and `ServiceConfigurator` types
- `internal/servicebuilder/` — Central service configurator construction for installer and MCP
- `internal/paperclip/` — Paperclip bootstrap client and software-shop seed profile
- `internal/prwatch/` — PR readiness classifier for Codex review, checks, requested changes, and merge state
- `internal/telegrambot/` — Telegram long-poll bot, allowlist checks, model selection, and OpenClaw command invocation
- `services/<slug>/` — Per-service workspaces with `service.yaml`, `README.md`, and optional Go configurator package
- `services/authentik/` — Authentik API client, one file per resource type
- `internal/tui/` — Bubbletea TUI with message-driven phase transitions
- `internal/mcp/` — MCP server, tools, resources
- `internal/runner/` — CommandRunner + HTTPClient interfaces for testability
- `internal/migrate/` — Host-to-Docker migration orchestrators (e.g. Mattermost)
- `internal/smoketest/` — Integration smoke tests (API config checks + headless browser login flows)
- `dev/homelab/` — Docker Compose, Prometheus/Loki/Promtail config, Grafana dashboards, Authentik patches

The installer includes first-run configurators for services that otherwise stop
at setup or product-local login screens. SonarQube and Mattermost use
managed admin credentials from the split 1Password store or `.env` fallback;
Mattermost compose enables local mode so the configurator can verify the
bootstrap state without interactive browser setup. Woodpecker stores server
state at `/var/lib/woodpecker` to preserve OAuth/session configuration across
container recreation.

## Conventions

### Code Style
- Interfaces for testability: `SecretStore`, `CommandRunner`, `HTTPClient`
- `ServiceConfigurator` interface for all services: `Name()`, `Slug()`, `CheckConfigured()`, `Configure()`
- Service manifests live at `services/<slug>/service.yaml` and back CLI/MCP service discovery.
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
  local admin login, Homarr native Authentik/OIDC, and proxy-gated landing
  checks for Woodpecker, OpenClaw, and Paperclip.
- `homelab service <slug> smoke` runs the manifest-owned `smoke_flow`; do not
  declare a `smoke_flow` unless `internal/smoketest` has an executable flow with
  that exact name.

### Git
- Never commit to main. Always branch first.
- Commit format: `type(scope): description`
- Types: feat, fix, test, docs, chore, refactor
- PR titles to `main` should follow Conventional Commits because CI validates
  the title and Release Please uses conventional commits for semver releases;
  legacy `[type] description` titles are accepted during transition.
- Release Please starts from `v0.1.0`: `fix:` creates patch releases, `feat:`
  creates minor releases, and `!` or `BREAKING CHANGE:` creates major releases.
- Published releases trigger `.github/workflows/update-homebrew-tap.yml`, which
  uses the `HOMEBREW_TAP_TOKEN` Actions secret to update
  `caboose-ai/homebrew-tap` formulae, install and `brew test` both updated
  formulae, and open or reuse a tap PR.
- CI builds both release binaries with `go build -buildvcs=false` to avoid
  Go VCS stamping failures in linked worktrees or source archive contexts.
- Direct `homelab reset` requires `--yes` unless `--dry-run`; live Docker,
  SQLite, secret, reset, and destructive filesystem changes require explicit
  human approval.

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
- DB networks: `*-internal` with `internal: true`, except Paperclip's DB bridge
  which stays private but not `internal` so Docker publishes the loopback DB port
  used by the host-network Paperclip app.
- App networks: `apps`
- Homarr is pinned to `ghcr.io/homarr-labs/homarr:v1.61.0`, persists `/appdata`
  in `homarr_data`, and uses native Authentik OIDC for the root homepage.
- Authentik mounts `authentik_data:/data` in both server and worker containers
  so uploaded media and runtime-managed files survive container recreation.
- Port exposure is configurable with `serve_mode` / `--serve-mode`:
  `public` binds host ports to `127.0.0.1` for Caddy, `local` binds to
  `0.0.0.0` for LAN access.

### Authentik
- OAuth2 and proxy providers must have matching Authentik applications.
- Authentik application lookups must exact-match slugs; API search responses can include broader matches.
- Installer-created or repaired applications should use `policy_engine_mode=all` so provider-level SSO policy gates access.
- Managed social login is Google-only in the browser: Google is enabled with `email_link` matching, local email/password fields are hidden on the default authentication flow, and the GitHub OAuth source is kept disabled/unpromoted even when credentials exist.
- Shared OAuth source upserts should leave `promoted` unset unless the caller intentionally owns promotion/demotion.
- Use `homelab oauth-setup --domain <domain>` to print GitHub/Google OAuth callback URLs, Turnstile hostname, and expected secret keys. Add `--create-turnstile` with `CLOUDFLARE_ACCOUNT_ID` and `CLOUDFLARE_API_TOKEN` to create the Turnstile widget through Cloudflare's API.

## Adding a New Service

1. Compose entry in `dev/homelab/docker-compose.yml`
2. Caddy site in `/etc/caddy/Caddyfile`
3. Authentik provider (proxy for SSO gate, OAuth2 for native OIDC)
4. Service workspace in `services/<slug>/` with `service.yaml` and `README.md`
5. Optional service configurator in `services/<slug>/`
6. Register configurator construction in `internal/servicebuilder`
7. Secrets in `secrets/store.go` if needed
8. Add or update `internal/smoketest/flows.go` when the service has native
   login, first-run setup, or proxy-gated landing behavior that must be proven
   by the live SSO browser test.
9. Set manifest `dashboard` and `sso` metadata; Homarr dashboard inclusion comes
   from `dashboard.show`, not a hard-coded service list.
10. For Paperclip-style control-plane services, add or update `services/<slug>/service.yaml` and any seed/context document.

## Live Environment

- Domain: `caboose-ai.io`
- Auth: `auth.caboose-ai.io` (Authentik)
- Services: git, ci, docker, grafana, ai, sonar, chat, openclaw, blog, paperclip, telegram-agent, dash
- Observability: Prometheus (metrics), Loki + Promtail (logs), Grafana (dashboards)
- Caddy reverse proxy on the host
- Cloudflare tunnel for `chat` and `sonar` subdomains
- Ollama on host for local LLM inference
- Paperclip runs behind the compose `paperclip` profile with Authentik forward-auth provider `paperclip-proxy`; Paperclip itself uses `local_trusted` mode on host loopback so Authentik is the only browser login. Its container bind-mounts `PAPERCLIP_WORKSPACE_ROOT` at the same path inside the container for repo-aware agent runs, its DB publishes a loopback-only port for the host-network app, and agents use `PAPERCLIP_API_URL=http://127.0.0.1:3100` instead of the Authentik-gated public URL for API calls. Start it with `mise run paperclip:up`, verify it with `mise run paperclip:smoke`, and seed it with `mise run paperclip:seed` so the repo path follows the compose `.env` `PAPERCLIP_WORKSPACE_ROOT`.
- Paperclip-driven agent control starts as a planner/audit ledger. Prefer the
  self-hosted delivery loop for implementation work: Forgejo/Gitea branches and
  PRs, Woodpecker CI, and Portainer Docker visibility. Confirmed execution must
  route through approved OpenClaw, Telegram, MCP, Homelab CLI, PR, or Portainer
  handoffs; Docker mutations, deploys, restarts, and destructive operations
  require explicit human approval.
- Paperclip seed data includes the internal delivery route: push `paperclip/*`
  branches to the `forgejo` remote, open Forgejo PRs, attach Woodpecker evidence
  from `https://ci.caboose-ai.io`, and treat `https://docker.caboose-ai.io`
  Portainer access as inspection-only unless the human approves a runtime
  mutation.
- OpenClaw is an external runtime tracked by manifest, URL, Authentik proxy,
  dashboard, smoke, and health metadata; do not add a fake compose service for it.
- Telegram Agent Bridge is an external runtime tracked by manifest and run on
  the host with `mise run telegram-agent:run`; it uses
  `TELEGRAM_BOT_TOKEN`, `TELEGRAM_ALLOWED_USER_IDS`, and the local OpenClaw CLI
  rather than Caddy, Homarr, Authentik, or Docker Compose.
- PR readiness notifications are host-run through `cmd/pr-ready-watch` or
  `mise run pr:watch-ready`; the watcher uses local `gh`, optional 1Password
  token lookup through `TELEGRAM_BOT_TOKEN_OP_ITEM`, and Telegram allowlist env
  vars to keep the human as the final PR review handoff.

## Security

- Do not commit `.env` or live secrets. Prefer 1Password or local untracked files.
- Example variables live in `dev/homelab/.env.example`.
- CI scans new push and PR commit ranges with Gitleaks.
- If any secret lands in git history, rotate it immediately.
