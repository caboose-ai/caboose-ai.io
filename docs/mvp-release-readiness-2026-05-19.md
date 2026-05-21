# MVP Release Readiness Pass (2026-05-19)

## Objective

Ship the MVP when the **local profile passes** and the **Homebrew binary
install/update path is documented and guarded**.

Public tunnel exposure and external MCP lifecycle management are post-MVP
profiles. They stay documented in the appendix so the next release lane has a
clear path, but they are not required to declare this MVP ready.

## MVP Scope

- Host OS: Linux
- Runtime: Docker Engine plus Docker Compose v2
- Stack source: a compose directory from a source checkout such as
  `dev/homelab`, or a deployed directory such as `/opt/homelab`
- Secrets: either 1Password CLI or a pre-populated compose `.env`
- Serve mode: forced to `local`
- Local smoke set: `authentik`, `forgejo`, `woodpecker`, `homarr`
- Recovery reset: optional and explicit with `--with-recovery`

Homebrew formulae install the `homelab` and `homelab-mcp` binaries. They do not
install a packaged compose stack, so Homebrew readiness proves binary delivery,
not standalone stack provisioning.

## Primary Acceptance Command

Preview the local gate:

```bash
mise run release:mvp-local -- --dry-run
```

Run the gate with 1Password-backed secrets:

```bash
mise run release:mvp-local
```

Run the gate with a compose `.env` only:

```bash
mise run release:mvp-local -- --secrets-env-only
```

Run the destructive recovery pass only when intentionally requested:

```bash
mise run release:mvp-local -- --with-recovery
```

The task calls `dev/homelab/mvp-local-readiness.sh`. The script defaults to:

```bash
HOMELAB_DOMAIN=caboose-ai.io
HOMELAB_COMPOSE_DIR=dev/homelab
HOMELAB_BIN=<unset, uses go run ./cmd/homelab>
```

The `mise` task intentionally defaults to the source checkout's compose
directory and source-built CLI even though other homelab tasks default to
`/opt/homelab` and installed binaries. Override the compose directory when
validating a deployed host:

```bash
HOMELAB_MVP_COMPOSE_DIR=/opt/homelab mise run release:mvp-local
```

Override `HOMELAB_BIN=homelab` only when the goal is to validate an installed
Homebrew binary against a deployed compose directory.

## Local Prerequisites

| Capability | Verification | Pass criteria |
| --- | --- | --- |
| Docker Engine | `docker version` | Client and Server sections render without daemon error |
| Docker Compose v2 | `docker compose version` | Version prints and exits 0 |
| Homelab binary | `homelab install --help` | Help output prints usage and exits 0 |
| 1Password mode | `op --version` | Version prints and operator can sign in |
| `.env` mode | `test -f dev/homelab/.env` | The compose env file exists |

The local gate runs these checks before install. With `--secrets-env-only`, it
checks the compose `.env` path instead of requiring `op`.

## Local Gate Details

The gate runs install in local mode:

```bash
homelab install --non-interactive \
  --domain caboose-ai.io \
  --compose-dir dev/homelab \
  --serve-mode local
```

Then it validates the MVP service set with slug-first service commands:

```bash
for slug in authentik forgejo woodpecker homarr; do
  homelab service --domain caboose-ai.io --compose-dir dev/homelab --serve-mode local "$slug" status
  homelab service --domain caboose-ai.io --compose-dir dev/homelab --serve-mode local "$slug" smoke
done
```

With `--with-recovery`, it also runs:

```bash
homelab reset --keep-env --yes \
  --domain caboose-ai.io \
  --compose-dir dev/homelab \
  --serve-mode local
```

Then it reinstalls and repeats the status/smoke loop.

## Homebrew Readiness

Verify a Homebrew-installed operator path with:

```bash
brew tap caboose-ai/tap
brew install caboose-homelab
brew test caboose-ai/tap/caboose-homelab
homelab install --help

brew install caboose-homelab-mcp
brew test caboose-ai/tap/caboose-homelab-mcp
homelab-mcp -help
```

The tap deploy workflow must keep these guardrails before pushing formula
updates:

- resolve and validate the release tag
- compute the tagged source archive SHA
- update both formulae
- run Ruby syntax checks
- run Homebrew style and audit checks
- install and `brew test` both formulae
- pause on the `homebrew-tap-deploy` environment before direct tap push
- re-check out the tap, re-apply the tested patch, and rerun validation before
  pushing

## Release Decision

Declare the MVP ready only when:

- `mise run release:mvp-local` passes on at least one clean local host
- `mise run release:mvp-local -- --dry-run` shows the expected command order
- Homebrew binary install and `brew test` verification are documented and
  guarded in the tap workflow
- any failure has a tracked fix or explicit release note

Do not claim public DNS, Cloudflare Tunnel, Caddy exposure, or external MCP as
MVP-ready unless their appendix checks have also passed separately.

## Supported Post-MVP Public Profile

The public profile adds Caddy and Cloudflare Tunnel to the local profile. It is
not required for the local MVP decision, but it has a supported acceptance gate:

```bash
mise run release:public-profile -- --dry-run
mise run release:public-profile
```

The task calls `dev/homelab/public-readiness.sh`. It forces `serve-mode public`,
validates the configured Caddy file, generates and validates the Cloudflare
Tunnel ingress config through `dev/homelab/setup-cloudflare-tunnel.sh`, and runs
public-mode status and smoke checks for the MVP browser services.

Equivalent manual checks:

```bash
mise run tunnel:config

for slug in authentik forgejo woodpecker homarr; do
  homelab service --domain caboose-ai.io --compose-dir dev/homelab --serve-mode public "$slug" status
  homelab service --domain caboose-ai.io --compose-dir dev/homelab --serve-mode public "$slug" smoke
done
```

Pass criteria: the tunnel config validates, public hostnames route through the
intended Caddy origin, and browser-facing authentication succeeds end to end.
No static public IP or inbound WAN port forwarding is required.

## Appendix B: Public Plus External MCP (Post-MVP)

The external MCP profile adds client request, admin approval, credential import,
and authenticated endpoint probing.

Suggested checks:

```bash
env | rg 'CLOUDFLARE_(API_TOKEN|ZONE_ID)'
homelab mcp access setup --compose-dir dev/homelab
homelab-mcp access request --name "$(hostname)" --out mcp-request.json
homelab mcp access approve mcp-request.json --out mcp-release.json
homelab-mcp access import mcp-release.json
homelab-mcp access status
mise run mcp:test-live
```

Pass criteria: setup, approval, import, token lookup, and authenticated endpoint
probe all complete without manual undocumented edits.
