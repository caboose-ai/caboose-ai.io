# Repository Guidelines

## Scope

These instructions apply to the whole `caboose-ai.io` repository. More specific nested guidance should override this file only for directories with different commands, risks, or ownership.

## Project Map

- `cmd/homelab/`: Homelab CLI and Bubble Tea installer entrypoint.
- `cmd/mcp/`: MCP server entrypoint for homelab tools.
- `internal/config/`: YAML config, defaults, validation, and derived service URLs.
- `internal/install/`: Installer orchestration, phases, Authentik provisioning, service configuration, and reset flow.
- `internal/service/`: Shared service manifest registry and `ServiceConfigurator` contract.
- `internal/servicebuilder/`: Central construction point for service configurators used by installer and MCP paths.
- `internal/smoketest/`: Live integration and browser smoke tests using the `integration` build tag.
- `services/<slug>/`: Per-service workspace with `service.yaml`, `README.md`, and optional Go configurator code.
- `services/authentik/`: Authentik API client and resource-specific helpers.
- `dev/homelab/`: Docker Compose stack, scripts, observability config, Grafana dashboards, and Authentik patches.

## Build, Test, and Validation

- `go build ./...`: Build all Go packages and binaries.
- `go test ./...`: Run unit tests. Use this before claiming general Go changes are complete.
- `mise run build`: Repo task wrapper for `go build ./...`.
- `mise run test`: Verbose repo task wrapper for `go test ./... -v`.
- `go run ./cmd/homelab oauth-setup --domain caboose-ai.io`: Print external OAuth and Turnstile setup values without changing the stack.
- `go run ./cmd/homelab install --non-interactive --domain caboose-ai.io --compose-dir dev/homelab`: Exercise the installer against the repo compose directory.
- `mise run sso:check-quick`: Run live Authentik API config smoke checks only.
- `mise run sso:check`: Run the full live SSO smoke suite.
- `mise run sso:e2e`: Run browser SSO tests with screenshot and action-log evidence.

If `go` is not on `PATH`, check `/home/caboose/.local/go/bin/go` before suggesting a reinstall.

## Coding Conventions

- Follow standard Go formatting with `gofmt`.
- Keep external effects behind interfaces such as `SecretStore`, `CommandRunner`, and `HTTPClient`; tests should mock those interfaces instead of shelling out or making live requests.
- Implement service setup through `internal/service.ServiceConfigurator` with `Name()`, `Slug()`, `CheckConfigured()`, and `Configure()`.
- Add new service manifests at `services/<slug>/service.yaml`; the `slug` field must match the directory name.
- Register configurable services in `internal/servicebuilder`; do not duplicate configurator construction across CLI, installer, and MCP code.
- Return errors up the stack. CLI and TUI layers own user-facing display.
- Avoid package-level mutable state. Installer state should flow through `Installer`, config structs, or Bubble Tea models.
- Use `SCREAMING_SNAKE_CASE` for secret keys, kebab-case for service slugs, and snake_case for YAML config keys.

## Testing Expectations

- Put tests beside the code under test as `*_test.go`.
- Prefer table-driven tests when behavior has multiple cases.
- For changes touching installer phases, secret handling, servicebuilder wiring, service manifests, or Authentik resources, add focused unit coverage before relying on live smoke tests.
- Integration tests in `internal/smoketest/` require a live stack and use `-tags integration`; do not run them as a substitute for unit tests.
- Browser smoke tests can mutate Authentik test state such as Turnstile keys and write evidence under `internal/smoketest/testdata/evidence/`.

## Security and Data Handling

- Do not read, print, or commit secret values from `.env`, `dev/homelab/.env`, 1Password, `fnox`, or shell history.
- Treat `dev/homelab/.env`, repo-root `.env`, OAuth credentials, bootstrap tokens, Turnstile keys, and Cloudflare API values as sensitive.
- Prefer `homelab oauth-setup` for callback URLs and expected secret names instead of exposing credential values.
- The split 1Password model uses dynamic generated secrets and static external credentials; reset code must preserve static external credentials unless the user explicitly asks for a factory wipe.
- Be careful with destructive tasks such as `mise run factory-reset`, `mise run homelab:e2e-reset`, and `homelab reset` without `--keep-env`.
- Docker Compose mounts `/var/run/docker.sock` for some services and uses internal-only database networks. Review compose changes for host access, privileged mode, and unintended port exposure.

## Documentation and Git Workflow

- Never commit directly to `main`; create a feature branch first.
- Use conventional commit messages such as `fix(install): ...`, `feat(homelab): ...`, `test(service): ...`, or `docs: ...`.
- Every PR to `main` that changes `cmd/*`, `internal/*`, or `dev/homelab/docker-compose.yml` must update the documentation set unless the PR has the `docs-exempt` label.
- Keep these files aligned when behavior, architecture, services, or commands change: `CLAUDE.md`, `.github/copilot-instructions.md`, and `README.md`.
- When adding a service, update compose, Authentik provider/application setup, `services/<slug>/service.yaml`, service README, servicebuilder registration if configurable, smoke flows when needed, and the docs set.

## Agent Workflow

- Start by checking `git status --short`; preserve unrelated user changes.
- Before editing an existing file, read the current file contents.
- Keep edits scoped to the requested task and existing package boundaries.
- For homelab installer work, verify both code-level tests and the relevant repo-native command path.
- For live SSO or Authentik failures, chase the exact failing contract in the installer, Authentik client, service configurator, or smoke test rather than stopping at wrapper errors.
- When a task changes service auth, first-run bootstrap, compose env, or smoke behavior, include a concrete validation command in the final response.
