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
The `paperclip:seed` task stores that route in each Paperclip project
workspace: local agents should push `paperclip/*` branches to the `forgejo`
remote, open Forgejo pull requests, attach Woodpecker evidence from
`https://ci.caboose-ai.io`, and keep Portainer at `https://docker.caboose-ai.io`
inspection-only unless a human approves an ops mutation.
It also seeds the `Improve caboose-ai.io itself with the homelab software shop`
kickoff issue in the Agent Control Plan project, assigned to the seeded CEO/PM,
so a clean install has an immediately runnable first task.

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

The public binary distribution path is source-built Homebrew formulae from
tagged GitHub releases:

```bash
brew tap caboose-ai/tap
brew install caboose-homelab
brew test caboose-ai/tap/caboose-homelab
homelab install --help

# Optional MCP server binary
brew install caboose-homelab-mcp
brew test caboose-ai/tap/caboose-homelab-mcp
homelab-mcp -help
```

After a new `caboose-homelab` release reaches the tap, the update/reset helper
upgrades only that formula and then runs `homelab reset --yes --keep-env`:

```bash
mise run homelab:update-reset

# Poll until an update appears, then upgrade and reset
mise run homelab:update-reset:wait

# Preview the direct script path
dev/homelab/update-homelab-and-reset.sh --dry-run

# After signing in to 1Password, store static .env values before reset
dev/homelab/update-homelab-and-reset.sh --yes --store-static-env
```

The formulae install binaries only: `homelab` and `homelab-mcp`. They do not
install a packaged Docker Compose stack. Local stack install/update still needs
a valid compose directory such as a source checkout's `dev/homelab` directory
or a deployed `/opt/homelab` directory. Runtime prerequisites are intentionally
documented instead of enforced as Homebrew dependencies: Docker with Compose for
the stack itself, plus either the 1Password CLI or a populated compose `.env`
fallback for secrets.

Caddy and Cloudflare Tunnel are the default public exposure path: `public` mode
keeps services on host loopback for Caddy, and `cloudflared` publishes the
stable `*.caboose-ai.io` hostnames without requiring a static IP or inbound WAN
port forwarding. Cloudflare API credentials are only needed for Turnstile or
DNS/tunnel automation.

## Release automation

Pull requests to `main` should use Conventional Commit titles such as
`feat(homelab): add installer check` or `fix(mcp): repair config loading`. CI
accepts legacy bracketed titles such as `[chore] tighten setup` during the
transition, then runs unit tests and builds both release binaries on every PR
and push to `main`.

Releases are managed by Release Please from conventional commits after merges
to `main`. The current root package version is tracked in
`.release-please-manifest.json`; future `fix:` commits produce patch releases,
`feat:` commits produce minor releases, and breaking changes produce major
releases. Release Please opens the version bump PR. Once that PR's CI checks
pass, `.github/workflows/release-please-automerge.yml` approves and
squash-merges the Release Please PR with `RELEASE_PLEASE_TOKEN`, so the original
feature PR merge is the only human interaction in the normal release path.
Published releases then trigger the Homebrew tap update workflow, which
requires the `HOMEBREW_TAP_TOKEN` Actions secret, computes the tagged source
archive SHA, updates both tap formulae, and runs formula syntax, Homebrew
style/audit, install, and `brew test` validation. If formulae changed, it
re-applies the tested formula patch to the latest tap checkout after the
`homebrew-tap-deploy` environment approval, reruns the same validation, then
pushes the verified formula commit directly to `caboose-ai/homebrew-tap`. The
tap update workflow can also be rerun manually with a release tag.

## Operator docs

- [Operator Runbook](docs/operator-runbook.md) — first-hour operator flow, MCP path selector, and troubleshooting ladder.
- [CAB-5 Operator UX and docs-flow audit (2026-05-18)](docs/cab-5-operator-ux-docs-flow-audit-2026-05-18.md) — operator journey audit across install, validation, MCP access, and troubleshooting navigation.
- [CAB Technical Baseline Scorecard (Latest)](docs/cab-technical-baseline-latest.md) — generated architecture/quality snapshot; refresh with `mise run baseline:scorecard`.
- [Homelab Service Documentation and Agent Control Plan](docs/homelab-agent-control-plan.md) — roadmap for documenting service capabilities, routing OpenClaw and Telegram agent workflows, minimizing outside token use, and supporting subagentic development.
- [MVP Release Readiness Pass (2026-05-19)](docs/mvp-release-readiness-2026-05-19.md) — local MVP acceptance gate plus Homebrew binary-readiness guardrails; public and external-MCP profiles are post-MVP appendices.

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

# Format Go packages
mise run fmt

# Lint Go packages
mise run lint

# Check for known vulnerabilities
mise run vulncheck

# Build the homelab CLI
mise run homelab:build

# Run the verified non-interactive installer
mise run install

# Run the interactive TUI installer
mise run homelab

# Reset everything while preserving static external credentials
mise run reset

# Store static .env credentials in 1Password, remove .env, then reset
mise run homelab:vault-reset

# Reset, then run the verified non-interactive installer
mise run reinstall

# Store static .env credentials in 1Password, remove .env, then reinstall
mise run homelab:vault-reinstall

# Upgrade only caboose-homelab from Homebrew when available, then reset
mise run homelab:update-reset

# Run the local MVP release-readiness gate
mise run release:mvp-local
mise run release:mvp-local -- --dry-run

# Print GitHub, Google, and Turnstile setup URLs/callbacks
mise run homelab:oauth-setup

# Create a Cloudflare Turnstile widget via API, then print all setup values
mise run homelab:create-turnstile

# Per-service operations
mise run service:status -- forgejo
mise run service:configure -- mattermost --dry-run
mise run service:logs -- forgejo
mise run service:smoke -- forgejo

# Portainer environment access recovery when the stored admin password drifted
mise run portainer:recover-access
dev/homelab/portainer-recover-access.sh --yes

# Paperclip is provisioned as an optional example, but install leaves it stopped.
# Start and seed the full software-shop example only when intentionally requested.
mise run paperclip:example
mise run paperclip:status
mise run paperclip:smoke
mise run paperclip:forgejo-remote
mise run paperclip:seed
mise run paperclip:up
# Optional override for a differently named internal Forgejo repository
mise run paperclip:seed -- --forgejo-repo-url https://git.caboose-ai.io/<owner>/<repo>.git

# Telegram agent bridge
mise run telegram-agent:run
mise run telegram-agent:notify -- "Homelab task finished."

# Homelab MCP server
mise run mcp:status
mise run mcp:probe-local
mise run mcp:resolve
mise run mcp:probe-external
mise run tunnel:print
mise run tunnel:config
mise run mcp:external-readiness
mise run mcp:setup-external
mise run mcp:access-live
mise run mcp:test-live

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
`mise run tunnel:print` prints a locally managed Cloudflare Tunnel ingress
config for the browser-facing service hostnames. `mise run tunnel:config`
writes that config to `HOMELAB_TUNNEL_CONFIG` when set, otherwise
`${XDG_CONFIG_HOME:-$HOME/.config}/cloudflared/homelab.yml`, and validates it
with `cloudflared`; set `HOMELAB_TUNNEL_ID` and
`HOMELAB_TUNNEL_CREDENTIALS_FILE` to make the generated config runnable. The
credentials file must already exist for validation; this proves the local config
is syntactically valid, while the endpoint smoke checks prove public
reachability.
`mise run mcp:external-readiness` is the supported external MCP acceptance
command. It validates the Cloudflare Tunnel ingress config, then runs the
request, approval, import, token, and authenticated external probe workflow.
`mise run mcp:setup-external` remains available only as the legacy/static-IP
path for the MCP A record and Caddy route. It requires `CLOUDFLARE_API_TOKEN`
and `CLOUDFLARE_ZONE_ID` from the environment or `fnox`, plus sudo access to
install and reload the Caddy route.
`homelab mcp access setup` requires Authentik admin credentials through the
configured secret store. Access requests are client-generated JSON blobs;
approved releases are encrypted to the requester's key and imported into a
0600 credential file under the user's config directory.
`dev/homelab/mcp-access-live.sh` wraps setup, request, approval, import, token
minting, and an authenticated initialize probe for live operator testing.
`dev/homelab/mcp-test-live.sh` probes the live endpoint with an existing
credential or `HOMELAB_MCP_TOKEN` without creating new access.
The matching `mise run mcp:access-live` and `mise run mcp:test-live` tasks use
the installed `homelab` and `homelab-mcp` binaries by default, so they exercise
the same path a Homebrew-installed operator will use.
Override variables inline when needed, for example:

```bash
HOMELAB_COMPOSE_DIR=dev/homelab mise run install
HOMELAB_MVP_COMPOSE_DIR=/opt/homelab mise run release:mvp-local
HOMELAB_SERVE_MODE=local mise run install
```

`serve_mode` controls host port exposure. `public` binds compose ports to
`127.0.0.1` for Caddy/TLS reverse proxying; the default public deployment
publishes those Caddy routes through Cloudflare Tunnel instead of a static IP.
`local` binds them to `0.0.0.0` for LAN access while keeping the same service
URLs and Authentik callback configuration.

Direct `homelab reset` is destructive and requires `--yes` unless `--dry-run`
is used. The recurring `mise` reset/reinstall tasks pass `--yes` explicitly so
automation remains intentional and auditable. Reset uses the compose file's
declared profiles when tearing down volumes, so optional services such as
Paperclip are included in a full reset. `homelab reset --yes --store-static-env`
copies static external credentials such as GitHub, Google, Turnstile, and
Cloudflare values from `.env` into the static 1Password vault before teardown;
if that copy fails, reset stops before destroying runtime state. The following
install restores those static values from the secret store into a new `.env`, so
the compose env file can be treated as generated runtime state when 1Password is
available. The `.env` fallback secret generator honors the configured character
recipe; symbol-enabled recipes require at least one symbol for services such as
SonarQube. Homarr SQLite board seeding and other live Docker, SQLite, secret,
reset, or destructive filesystem mutations require explicit human approval
before being run against live state.

## Testing

```bash
go test ./...                       # unit tests
mise run go:check-toolchain         # enforce Go version matches go.mod
mise run lint                       # golangci-lint
mise run vulncheck                  # govulncheck

# SSO smoke tests (requires live stack running)
mise run sso:check                  # full suite: config + endpoints + browser login
mise run sso:check-quick            # API config checks only
mise run sso:e2e                    # browser SSO with click/input screenshot evidence
mise run cab:tier23-live            # CAB Tier 2/3 lane with timestamped evidence capture
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
For CAB follow-up evidence capture, use `mise run cab:tier23-live` (or
`dev/cab-tier23-live-validation.sh "<svc1,svc2>"`) and attach the generated
`docs/evidence/cab-8/*-summary.md` artifact.

## Infrastructure

- **Caddy** reverse proxy on the host — handles TLS and routes to containers in `public` serve mode
- **Cloudflare Tunnel** — default public exposure path for stable service
  hostnames without a static IP or inbound WAN port forwarding
- **Docker Compose** at `dev/homelab/docker-compose.yml` — all services
- **Homarr** homepage — pinned to `ghcr.io/homarr-labs/homarr:v1.61.0`, stores dashboard state in `homarr_data:/appdata`, and uses native Authentik OIDC for `caboose-ai.io`
- **Authentik** state — `/data` is persisted in the `authentik_data` volume for uploaded media and runtime-managed files
- **Paperclip** profile (`docker compose --profile paperclip ...`) — built from upstream tag `v2026.428.0`, backed by `paperclip-db`, bind-mounts `PAPERCLIP_WORKSPACE_ROOT` for local agent workspaces, publishes the DB and runtime API on loopback for host-network agents, binds the app to host loopback in `local_trusted` mode, and is Authentik-gated publicly through `paperclip-proxy`
- **Forgejo + Woodpecker + Portainer** self-hosted delivery loop — Forgejo is
  the branch/PR source, Woodpecker is the CI verification surface, and
  Portainer is Docker visibility plus approved operational handoff rather than
  an unchecked executor. `.woodpecker.yml` mirrors the GitHub docs, secret,
  release-helper, Go test, and build gates for repositories enabled in
  Woodpecker. Use `mise run portainer:recover-access` to diagnose Portainer
  admin password drift; the reset repair path requires explicit `--yes`.
- **OpenClaw** external runtime — tracked for URL, Authentik proxy, dashboard,
  smoke, and health metadata without claiming a local compose service
- **Telegram Agent Bridge** external runtime — host-run long-polling bot that
  uses `TELEGRAM_BOT_TOKEN`, `TELEGRAM_ALLOWED_USER_IDS`, and the local OpenClaw
  CLI instead of a compose service or public route
- **PR readiness watcher** external runtime — host-run `gh`/Telegram poller
  for Codex review completion, check state, and final human-review handoff
- **Legacy direct DNS/Caddy exposure** — available for the MCP A-record path
  through `mise run mcp:setup-external` when a stable public IP is intentional
- **1Password** for secret storage (with `.env` fallback)
- **Prometheus + Loki** for metrics and logs, visualized in Grafana. Mattermost
  Team Edition is omitted from Prometheus scrapes because it does not expose the
  HTTP `/metrics` endpoint.

## Documentation

Three files are kept in sync and must be updated on every PR to main:

| File | Audience |
|------|----------|
| `CLAUDE.md` | Claude Code / AI assistants |
| `.github/copilot-instructions.md` | GitHub Copilot |
| `README.md` | Humans |

CI checks this via `.github/workflows/docs-check.yml`. Add the `docs-exempt` label to skip.

## Security

- Do not commit `.env` or live secrets. Use 1Password or local untracked files.
- Example variables live in `dev/homelab/.env.example`.
- CI runs Gitleaks secret scanning on new push and PR commit ranges.
- If any secret is ever committed, treat it as compromised and rotate it.
