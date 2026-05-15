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
| SonarQube | `sonar.caboose-ai.io` | Code quality & security |
| Mattermost | `chat.caboose-ai.io` | Team chat |
| OpenClaw | `openclaw.caboose-ai.io` | OpenClaw app behind Authentik forward auth |
| Ghost | `blog.caboose-ai.io` | Blog |
| Paperclip | `paperclip.caboose-ai.io` | AI-labor control plane for the homelab software shop |
| Homelab MCP | `mcp.caboose-ai.io` | Bearer-token MCP HTTP endpoint for homelab automation |
| Telegram Agent Bridge | — | Private Telegram bot for OpenClaw-backed agent control |
| Homarr | `caboose-ai.io` | Native Authentik/OIDC dashboard homepage |
| Prometheus | — | Metrics collection |
| Loki + Promtail | — | Log aggregation |

Browser-facing services authenticate through Authentik via OAuth2/OIDC or forward-auth proxy. The installer keeps Authentik applications open (`policy_engine_mode=all`) so provider-level SSO decisions control access consistently. The social login configurator keeps Google as the enabled source with email-link matching and hides local email/password fields on the default authentication flow; GitHub credentials can remain stored, but the managed GitHub source is disabled and unpromoted by default. Generic source upserts preserve the existing promotion state unless explicitly changed. Telegram Agent Bridge is not public web surface; it is a host-run bot protected by a Telegram user-ID allowlist.

Paperclip-driven agent control uses the self-hosted delivery loop first:
Forgejo hosts branches and PRs, Woodpecker reports CI verification, and
Portainer gives Docker visibility for approved operational handoffs. Paperclip
is the planning and audit ledger; confirmed execution still routes through
approved OpenClaw, Telegram, MCP, Homelab CLI, PR, or Portainer handoffs, and
Docker mutations remain confirmation-gated.

The installer also completes first-run setup for services that expose a product
setup or local-login boundary before they can participate in the SSO smoke
suite. SonarQube and Mattermost are configured with managed admin
credentials from 1Password or the compose `.env` fallback, and Mattermost local
mode is enabled in compose for repeatable bootstrap checks. Woodpecker keeps its
server data under `/var/lib/woodpecker` so OAuth/session setup survives
container recreation.

## Binaries

- **`cmd/homelab`** — Bubbletea TUI installer that bootstraps the entire stack: generates secrets, starts containers, provisions OAuth providers, configures each service.
  - Includes `homelab service <slug> <status|configure|logs|smoke|open>` for per-service operations backed by `services/<slug>/service.yaml`.
  - Loads config through shared `internal/config` helpers before applying command-specific domain, compose-dir, and serve-mode overrides.
- **`cmd/mcp`** — MCP server exposing homelab tools to AI assistants.
  - The `diagnose-service` prompt uses service manifests for runtime and compose-service lookup, including external-runtime services with no local compose logs.
  - Includes `agent_invoke` provider fallback across Ollama, Claude Code, Copilot CLI, and Emberfall.
  - Includes `homelab-mcp access <request|import|token|status>` for admin-approved external client access.
- **`cmd/telegram-agent`** — Private Telegram bot that runs local OpenClaw gateway prompts, role-scoped agent prompts, and narrow `/lab` Homelab MCP queries for allowlisted Telegram users.
- **`cmd/pr-ready-watch`** — Local GitHub PR watcher that polls Codex review, checks, and review state, then notifies Telegram when the PR is ready for final human review.
  - Treats Codex review signals as current only when they occur after the latest PR head commit, so stale review comments cannot mark new changes ready.

## Homebrew

The first public distribution path is source-built Homebrew formulae from tagged
GitHub releases:

```bash
brew tap caboose-ai/tap
brew install caboose-homelab

# Optional MCP server binary
brew install caboose-homelab-mcp
```

The formulae install `homelab` and `homelab-mcp`. Runtime prerequisites are
intentionally documented instead of enforced as Homebrew dependencies: Docker
with Compose for the stack itself, plus either the 1Password CLI or a populated
compose `.env` fallback for secrets. Caddy and Cloudflare are only needed for
deployment modes that use the host reverse proxy, TLS, tunnels, or Turnstile
automation.

## Release automation

Pull requests to `main` should use Conventional Commit titles such as
`feat(homelab): add installer check` or `fix(mcp): repair config loading`. CI
accepts legacy bracketed titles such as `[chore] tighten setup` during the
transition, then runs unit tests and builds both release binaries on every PR
and push to `main`.

Releases are managed by Release Please from conventional commits after merges
to `main`. The current release manifest starts at `v0.1.0`; future `fix:`
commits produce patch releases, `feat:` commits produce minor releases, and
breaking changes produce major releases. Release Please opens the version bump
PR and publishes the GitHub release when that PR is merged. Published releases
then trigger the Homebrew tap update workflow, which requires the
`HOMEBREW_TAP_TOKEN` Actions secret, computes the tagged source archive SHA,
updates both tap formulae, installs and `brew test`s both updated formulae, and
opens or reuses a pull request in
`caboose-ai/homebrew-tap`. The tap update workflow can also be rerun manually
with a release tag.

## Planning docs

- [Homelab Service Documentation and Agent Control Plan](docs/homelab-agent-control-plan.md) — roadmap for documenting service capabilities, routing OpenClaw and Telegram agent workflows, minimizing outside token use, and supporting subagentic development.

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

# Optional Paperclip profile
mise run paperclip:up
mise run paperclip:status
mise run paperclip:smoke
mise run paperclip:seed

# Telegram agent bridge
mise run telegram-agent:run
mise run telegram-agent:notify -- "Homelab task finished."

# Homelab MCP server
mise run mcp:status
mise run mcp:probe
mise run mcp:resolve
mise run mcp:probe-external
mise run mcp:setup-external

# Homelab MCP access workflow
homelab-mcp access request --name "codex on laptop" --out mcp-request.json
homelab mcp access setup
homelab mcp access approve mcp-request.json --out mcp-release.json
homelab-mcp access import mcp-release.json
homelab-mcp access token
dev/homelab/mcp-access-live.sh --name "codex on laptop"
dev/homelab/mcp-test-live.sh

# PR readiness watcher
mise run pr:watch-ready -- --repo caboose-ai/caboose-ai.io --pr 48 --poll 1m --timeout 10m

# Migrate host Mattermost to Docker
go run ./cmd/homelab migrate
```

The homelab tasks default to `HOMELAB_DOMAIN=caboose-ai.io`, `HOMELAB_COMPOSE_DIR=/opt/homelab`, and `HOMELAB_SERVE_MODE=public`.
`mise run mcp:setup-external` additionally requires `CLOUDFLARE_API_TOKEN` and
`CLOUDFLARE_ZONE_ID` from the environment or `fnox`, plus sudo access to
install and reload the Caddy route.
`homelab mcp access setup` requires Authentik admin credentials through the
configured secret store. Access requests are client-generated JSON blobs;
approved releases are encrypted to the requester's key and imported into a
0600 credential file under the user's config directory.
`dev/homelab/mcp-access-live.sh` wraps setup, request, approval, import, token
minting, and an authenticated initialize probe for live operator testing.
`dev/homelab/mcp-test-live.sh` probes the live endpoint with an existing
credential or `HOMELAB_MCP_TOKEN` without creating new access.
Override variables inline when needed, for example:

```bash
HOMELAB_COMPOSE_DIR=dev/homelab mise run install
HOMELAB_SERVE_MODE=local mise run install
```

`serve_mode` controls host port exposure. `public` binds compose ports to
`127.0.0.1` for Caddy/TLS reverse proxying. `local` binds them to `0.0.0.0`
for LAN access while keeping the same service URLs and Authentik callback
configuration.

Direct `homelab reset` is destructive and requires `--yes` unless `--dry-run`
is used. The recurring `mise` reset/reinstall tasks pass `--yes` explicitly so
automation remains intentional and auditable. Homarr SQLite board seeding and
other live Docker, SQLite, secret, reset, or destructive filesystem mutations
require explicit human approval before being run against live state.

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
Homarr validates the native Authentik/OIDC dashboard login, while
proxy-gated services such as Woodpecker, OpenClaw, and Paperclip are validated
by reaching their protected landing URLs. Per-service smoke commands run the
manifest-owned flow, for example `mise run paperclip:smoke`.

## Infrastructure

- **Caddy** reverse proxy on the host — handles TLS and routes to containers in `public` serve mode
- **Docker Compose** at `dev/homelab/docker-compose.yml` — all services
- **Homarr** homepage — pinned to `ghcr.io/homarr-labs/homarr:v1.61.0`, stores dashboard state in `homarr_data:/appdata`, and uses native Authentik OIDC for `caboose-ai.io`
- **Authentik** state — `/data` is persisted in the `authentik_data` volume for uploaded media and runtime-managed files
- **Paperclip** profile (`docker compose --profile paperclip ...`) — built from upstream tag `v2026.428.0`, backed by `paperclip-db`, bind-mounts `PAPERCLIP_WORKSPACE_ROOT` for local agent workspaces, publishes the DB on loopback for the host-network app, binds the app to host loopback in `local_trusted` mode, and is Authentik-gated publicly through `paperclip-proxy`
- **Forgejo + Woodpecker + Portainer** self-hosted delivery loop — Forgejo is
  the branch/PR source, Woodpecker is the CI verification surface, and
  Portainer is Docker visibility plus approved operational handoff rather than
  an unchecked executor
- **OpenClaw** external runtime — tracked for URL, Authentik proxy, dashboard,
  smoke, and health metadata without claiming a local compose service
- **Telegram Agent Bridge** external runtime — host-run long-polling bot that
  uses `TELEGRAM_BOT_TOKEN`, `TELEGRAM_ALLOWED_USER_IDS`, and the local OpenClaw
  CLI instead of a compose service or public route
- **PR readiness watcher** external runtime — host-run `gh`/Telegram poller
  for Codex review completion, check state, and final human-review handoff
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
