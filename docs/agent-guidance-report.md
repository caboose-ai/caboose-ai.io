# Agent Guidance Report

Repository: `/home/caboose/dev/caboose-ai.io`
Generated: `2026-05-08`

## Executive Summary

This repository is a Go homelab infrastructure monorepo centered on an Authentik-backed SSO stack. The current guidance surface is useful but split across `CLAUDE.md`, `.github/copilot-instructions.md`, and `README.md`; there is no root `AGENTS.md`.

I generated proposed agent guidance under `docs/` only:

- `docs/proposed-AGENTS.md`
- `docs/proposed-dev-homelab-AGENTS.md`
- `docs/proposed-internal-smoketest-AGENTS.md`
- `docs/proposed-services-authentik-AGENTS.md`

No source code or existing guidance file was modified.

## Existing Guidance

Observed:

- `CLAUDE.md` documents the project map, commands, conventions, Git workflow, docs policy, Authentik notes, and adding-service checklist.
- `.github/copilot-instructions.md` mirrors the same architecture and coding expectations for GitHub Copilot.
- `README.md` gives the human-facing stack overview, service table, quick start, testing commands, and docs policy.
- `.github/workflows/docs-check.yml` enforces docs updates for PRs that change `cmd/*`, `internal/*`, or `dev/homelab/docker-compose.yml`, unless the PR has `docs-exempt`.
- There is no root `AGENTS.md`, `GEMINI.md`, or nested `AGENTS.md`.

Recommendation:

- Add a root `AGENTS.md` based on `docs/proposed-AGENTS.md`.
- Keep `CLAUDE.md`, `.github/copilot-instructions.md`, and `README.md` as the required docs-sync set.
- Add nested `AGENTS.md` only for high-risk directories where local instructions materially differ from root guidance.

## Architecture and Repo Map

Observed:

- `go.mod` declares module `github.com/caboose-ai/caboose-ai.io` with Go `1.26.2`.
- `cmd/homelab/main.go` contains the homelab CLI entrypoint and subcommands: `install`, `reset`, `migrate`, `oauth-setup`, `service`, `recovery`, and `paperclip`.
- `cmd/mcp/main.go` starts the MCP server, requiring a config file and supporting stdio or HTTP mode.
- `internal/config/config.go` defines defaults, YAML config, `serve_mode`, compose directory, orchestrator selection, and split 1Password vault names.
- `internal/install/install.go` owns installer state, prereq checks, secret generation, compose apply, Authentik initialization, and service configuration.
- `internal/service/service.go` defines the shared `ServiceConfigurator` interface.
- `internal/service/registry.go` loads `services/<slug>/service.yaml` and validates that manifest slugs match directory names.
- `internal/servicebuilder/builder.go` is the central configurator construction point.
- `services/<slug>/` contains service manifests and optional configurator packages.
- `services/authentik/` is a resource-oriented Authentik API client.
- `dev/homelab/docker-compose.yml` defines the compose stack, internal DB networks, app network, Authentik containers, observability, and the profile-gated Paperclip services.

Confidence: High. These are directly observed from the current tree.

## Commands

Observed:

- `mise.toml` defines repo tasks for build, install, reset, reinstall, service operations, OAuth setup, Turnstile creation, Paperclip seed, MCP, tests, SSO checks, browser E2E, and destructive E2E reset.
- `go build ./...` and `go test ./...` are the baseline local validation commands.
- `mise run sso:check-quick` runs `go test -tags integration ./internal/smoketest/ -run TestSSO_Config -v`.
- `mise run sso:check` runs the full live SSO smoke suite with a five-minute timeout.
- `mise run sso:e2e` enables screenshot/action evidence for browser flows.
- `mise run homelab:e2e-reset` is destructive: reset, install, and browser SSO evidence tests.

Recommendation:

- Root guidance should separate cheap unit validation from live-stack validation.
- Destructive reset commands should be explicitly labeled so agents do not run them casually.

Confidence: High.

## Code Style and Dependencies

Observed:

- Go code uses small packages under `internal/` and per-service packages under `services/`.
- External effects are abstracted behind interfaces such as `SecretStore`, `CommandRunner`, and `HTTPClient`.
- Service setup follows the `ServiceConfigurator` interface and central servicebuilder registration.
- Manifests use YAML and are loaded through structured parsing, not string scanning.
- Authentik helpers are split by resource type.
- Dependencies include Bubble Tea/Lip Gloss for TUI, Rod for browser tests, the Go MCP SDK, and `yaml.v3`.

Recommendation:

- Agent guidance should require `gofmt`, interface-backed tests, manifest updates for service discovery, and servicebuilder registration for configurable services.

Confidence: High.

## Testing and Quality Gates

Observed:

- Unit tests live next to production code across `internal/` and `services/`.
- `internal/smoketest/` contains build-tagged integration tests, endpoint tests, browser flows, evidence recording, and live Authentik token discovery/recovery.
- Browser flow definitions include native OAuth/OIDC, service-specific login controls, local-admin handoffs, and proxy-gated landing checks.
- Evidence is written under `internal/smoketest/testdata/evidence/` when enabled.
- The docs check workflow blocks relevant PRs without documentation updates unless labeled `docs-exempt`.

Recommendation:

- Root guidance should require focused unit tests before live smoke tests.
- Nested smoke-test guidance should warn that tests touch live Authentik state and can change Turnstile test keys.

Confidence: High.

## Security, Data, and Risk

Observed:

- The inventory flagged `.env` and `dev/homelab/.env` as secret-like paths.
- `internal/secrets/store.go` defines generated bootstrap secrets, derived OAuth secret keys, static external credentials, social OAuth credentials, and Turnstile keys.
- `dev/homelab/docker-compose.yml` mounts Docker socket access for some services, uses privileged cAdvisor access, has internal database networks, and profile-gates Paperclip.
- `internal/smoketest/suite.go` can recover Authentik bootstrap API token state from the running container but does not need to print token values.

Recommendation:

- Agent guidance should explicitly forbid reading, printing, or committing real `.env`, 1Password, OAuth, Turnstile, Cloudflare, or bootstrap-token values.
- Compose guidance should call out Docker socket mounts, privileged containers, host networking, and port exposure.
- Authentik guidance should treat recovery URLs, bearer tokens, OAuth client secrets, and bootstrap tokens as sensitive.

Confidence: High.

## Documentation Gaps and Proposed Guidance Files

Observed gaps:

- No root `AGENTS.md` exists, so Codex-style repo guidance is currently inherited from `CLAUDE.md` and user-provided session instructions.
- No nested guidance exists for high-risk directories.
- Existing docs are strong but optimized for humans, Claude, and Copilot rather than repo-local Codex operating rules.

Recommended nested guidance:

- `dev/homelab/AGENTS.md`: Compose, secret, host-access, profile, and validation rules are materially different from Go source defaults.
- `internal/smoketest/AGENTS.md`: Live integration tests have stateful Authentik, browser, evidence, and secret-redaction concerns.
- `services/authentik/AGENTS.md`: Auth-critical API helpers need exact-match slug semantics, token hygiene, and focused tests.

Not recommended right now:

- Per-service nested guidance for every `services/<slug>/` directory. The root guidance plus service README/manifests should be enough unless a service gets distinct ownership or test commands.
- Separate guidance for `cmd/` or most `internal/` packages. Current patterns are consistent enough for root guidance.

Confidence: Medium-high. The nested candidates are recommendations, but the underlying risks are directly observed.

## Proposed Files

- `docs/proposed-AGENTS.md`: Root Codex/agent guidance candidate.
- `docs/proposed-dev-homelab-AGENTS.md`: Candidate content for `dev/homelab/AGENTS.md`.
- `docs/proposed-internal-smoketest-AGENTS.md`: Candidate content for `internal/smoketest/AGENTS.md`.
- `docs/proposed-services-authentik-AGENTS.md`: Candidate content for `services/authentik/AGENTS.md`.

## Confidence and Open Questions

Confidence:

- Architecture map: High.
- Commands: High.
- Testing and docs policy: High.
- Security/risk guidance: High.
- Nested guidance placement: Medium-high.

Open questions:

- Whether the team wants to actually add root and nested `AGENTS.md` files now, or keep the generated candidates under `docs/` for review first.
- Whether `GEMINI.md` should be added as a compatibility pointer after root `AGENTS.md` lands.
- Whether the docs-check workflow should include `services/*` changes in addition to `cmd/*`, `internal/*`, and `dev/homelab/docker-compose.yml`.
