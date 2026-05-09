# Repository Guidelines

## Scope

These instructions apply to `services/authentik/`.

## Project Map

- `client.go`: Authentik HTTP client, request helpers, and auth/error handling.
- `providers.go`: OAuth2 and proxy provider operations.
- `sources.go`: Social source operations.
- `stages.go`, `bindings.go`, `brands.go`, `outpost.go`, `users.go`, `recovery.go`: Resource-specific API helpers.

## Build, Test, and Validation

- `go test ./services/authentik -v`: Authentik client and helper unit tests.
- `go test ./internal/install -run 'Provider|Application|Flow|Social|Outpost' -v`: Installer tests that often cover Authentik helper contracts.
- `mise run sso:check-quick`: Live config verification after Authentik provisioning changes.

## Conventions

- Keep one Authentik resource type per file where practical.
- Define request and response structs instead of ad hoc map/string handling.
- Use the existing `Client` helpers for HTTP verbs and error handling.
- Exact-match application and provider slugs after search results; Authentik API search can return broader matches.
- Installer-created or repaired applications should use `policy_engine_mode=all` so provider-level SSO policy controls access.
- Shared OAuth source upserts should not change `promoted` unless the caller explicitly owns promotion or demotion.

## Security and Risk

- Authentik helpers touch identity, OAuth, recovery, outpost, and social-login behavior. Treat changes here as auth-critical.
- Do not log bearer tokens, bootstrap tokens, OAuth client secrets, recovery URLs, or raw `.env` values.
- Prefer tests with fake HTTP clients for request shape and response parsing.

## Agent Workflow

- When fixing live SSO, verify whether the bug is in Authentik helper semantics, installer provisioning order, service configurator setup, or the smoke test expectation.
- Add or update focused unit tests before running live smoke checks.
- Use `homelab oauth-setup --domain <domain>` for callback URL and secret-key expectations.
