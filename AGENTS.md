# Repository Guidelines

## Scope

These instructions apply to the whole `caboose-ai.io` repository. Nested
`AGENTS.md` files override this file for their directory.

## Project Map

- `cmd/`: CLI and MCP binary entrypoints.
- `internal/`: shared Go packages for config, install, service registry, CLI,
  MCP, runner interfaces, orchestration, and smoke tests.
- `services/<slug>/`: per-service workspaces with `service.yaml`, `README.md`,
  and optional Go configurator code.
- `dev/homelab/`: Docker Compose stack, legacy shell automation, observability
  config, Grafana dashboards, and Authentik patches.
- `docs/`: generated and proposed architecture or agent-guidance reports.

## Build, Test, and Validation

- `go test ./...`: run unit tests before claiming Go changes are complete.
- `go build ./...`: build all packages and binaries.
- `mise run lint`: run repository golangci-lint checks.
- `mise run vulncheck`: run govulncheck across repository packages.
- `mise run test`: repo wrapper for verbose Go tests.
- `mise run sso:check-quick`: live Authentik API smoke checks.
- `mise run sso:check`: full live SSO smoke suite.
- `git diff --check`: verify whitespace before committing.

If `go` is missing from `PATH`, check `/home/caboose/.local/go/bin/go` before
suggesting a reinstall.

## Repository Patterns

- Keep behavior behind package contracts. CLI and TUI code should delegate to
  `internal/*` packages instead of embedding Docker, Authentik, or filesystem
  logic directly.
- Put service facts in the service workspace first: `services/<slug>/service.yaml`
  for registry metadata, service README for operator notes, configurator code
  only when the service needs active setup.
- Use existing interfaces such as `SecretStore`, `CommandRunner`, and
  `HTTPClient` for external effects so tests can mock them.
- Keep generated evidence, local tokens, agent settings, and built binaries out
  of commits.
- Update `CLAUDE.md`, `.github/copilot-instructions.md`, and `README.md` when a
  PR changes user-visible architecture, commands, services, or infrastructure.

## Good And Bad Examples

Good:

```go
func (c *Configurator) Configure(ctx context.Context, opts service.ConfigureOpts) (*service.ConfigureResult, error) {
    value, err := c.Secrets.Get(ctx, "SERVICE_CLIENT_ID")
    if err != nil {
        return nil, err
    }
    // configure through injected clients
}
```

Bad:

```go
func configure() {
    _ = os.Getenv("SERVICE_CLIENT_ID")
    _ = exec.Command("docker", "exec", "service", "rewrite-live-state").Run()
}
```

Good:

```yaml
slug: forgejo
display_name: Forgejo
compose_services:
  - forgejo
smoke_flow: forgejo
docs:
  - README.md
```

Bad:

```yaml
slug: forgejo-prod
compose_services:
  - forgejo
# README and smoke flow omitted, with duplicate service data hard-coded elsewhere.
```

## Git Workflow

- Never commit directly to `main`; branch first.
- Preserve unrelated user changes and local state.
- Use Conventional Commit messages such as `fix(install): ...`,
  `feat(homelab): ...`, `test(service): ...`, or `docs: ...`.
- Do not commit `.env`, `.claude/`, `.playwright-mcp/`, `.remember/`, smoke
  evidence, screenshots, logs, or generated binaries unless explicitly asked.
