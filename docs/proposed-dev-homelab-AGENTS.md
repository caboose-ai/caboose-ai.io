# Repository Guidelines

## Scope

These instructions apply to `dev/homelab/`.

## Project Map

- `docker-compose.yml`: Main homelab compose stack.
- `.env.example`: Example environment surface. Do not mirror real secret values into it.
- `prometheus.yml`, `promtail.yml`, `grafana/`: Observability configuration.
- `patches/authentik/`: Mounted Authentik migration patches.
- `lib/*.sh` and `sso.sh`: Older shell automation for SSO and service setup.
- `tests/`: Shell/Bats validation for the legacy scripts.

## Build, Test, and Validation

- `docker compose -f dev/homelab/docker-compose.yml config --services`: Validate compose syntax and default service inclusion.
- `docker compose --profile paperclip -f dev/homelab/docker-compose.yml config --services`: Validate profile-gated Paperclip services.
- `HOMELAB_COMPOSE_DIR=dev/homelab mise run install`: Run the non-interactive installer against the repo compose directory.
- `mise run sso:check-quick`: Validate Authentik configuration against a live stack.
- `mise run sso:check`: Validate full live SSO behavior.

## Conventions

- Use `${VAR}` env substitution for secrets and runtime values.
- Keep database-only networks marked `internal: true`.
- App-facing services that need Authentik token exchange should join the `apps` network.
- Use `HOMELAB_BIND_ADDRESS` and `serve_mode` behavior for host port exposure instead of hardcoding `0.0.0.0` or `127.0.0.1`.
- Optional services that should not block default startup should use Compose profiles.

## Security and Risk

- Do not read, print, or commit `dev/homelab/.env`.
- Treat Docker socket mounts, privileged containers, host networking, Cloudflare tunnel config, and Authentik patch mounts as high-risk changes.
- Review compose changes for accidental LAN exposure, secret defaults, public host ports, and loss of persistent volumes.

## Agent Workflow

- Prefer Go installer paths in `cmd/homelab` and `internal/install` for new behavior. Use shell scripts here mainly for legacy compatibility or diagnosis.
- After compose changes, run a compose config validation command before claiming the file is valid.
- If a compose change affects code or infrastructure, update `CLAUDE.md`, `.github/copilot-instructions.md`, and `README.md` unless the PR is intentionally docs-exempt.
