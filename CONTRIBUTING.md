# Contributing

## Development Bootstrap

1. Clone the repo and enter it.
2. Install toolchain dependencies:

```bash
mise install
```

3. If `go` is not on `PATH`, use the local fallback binary path before reinstalling:

```bash
export PATH="/home/caboose/.local/go/bin:$PATH"
```

4. Verify core checks:

```bash
go build ./...
go test ./...
```

## Common `mise` Tasks

Use `mise run <task>` for repeatable workflows:

- `build`: build repo packages.
- `test`: run verbose Go unit tests.
- `fmt`: format all Go packages.
- `lint`: run golangci-lint with repository rules.
- `vulncheck`: run govulncheck across all packages.
- `go:check-toolchain`: assert local Go matches the `go.mod` contract.
- `install`: run the verified non-interactive installer.
- `homelab`: run the interactive TUI installer.
- `reinstall`: reset then reinstall non-interactively.
- `sso:check-quick`: live Authentik API smoke checks.
- `sso:check`: full live SSO smoke suite.

## Workflow Expectations

- Branch from `main`; do not commit directly to `main`.
- Use Conventional Commits, for example `fix(install): ...` or `docs: ...`.
- Preserve unrelated local changes and state.
- Keep generated artifacts, secrets, evidence logs, and local tokens out of commits.

## Documentation Rules

PRs that change code or infrastructure must keep these files aligned:

- `README.md`
- `CLAUDE.md`
- `.github/copilot-instructions.md`

CI enforces this via `.github/workflows/docs-check.yml`. If the change is intentionally docs-neutral, apply the `docs-exempt` label.

## Validation Before PR

Run the smallest relevant set for your change, then include what you ran in the PR description:

```bash
go test ./...
go build ./...
mise run lint
mise run vulncheck
git diff --check
```

Add smoke coverage when your change affects live SSO flows:

```bash
mise run sso:check-quick
mise run sso:check
```
