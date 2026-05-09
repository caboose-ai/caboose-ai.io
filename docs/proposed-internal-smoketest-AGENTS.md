# Repository Guidelines

## Scope

These instructions apply to `internal/smoketest/`.

## Project Map

- `suite.go`: Live test setup, Authentik token discovery/recovery, browser launch, and test captcha key handling.
- `flows.go`: Service flow definitions for OAuth/OIDC and proxy-gated browser checks.
- `smoketest_config_test.go`: Authentik/provider configuration checks.
- `smoketest_endpoints_test.go`: Live endpoint checks.
- `smoketest_browser_test.go`: Browser SSO and service handoff checks.
- `evidence.go`: Optional screenshot/action-log recording.

## Build, Test, and Validation

- `go test -tags integration ./internal/smoketest/ -run TestSSO_Config -v`: API config checks.
- `go test -tags integration ./internal/smoketest/ -run TestSSO_Endpoints -v`: Endpoint checks.
- `SMOKETEST_EVIDENCE=1 go test -tags integration ./internal/smoketest/ -run TestSSO_BrowserFlows -v -timeout 10m`: Browser flow with evidence.
- `mise run sso:check-quick`: Repo shortcut for config checks.
- `mise run sso:check`: Repo shortcut for the full suite.

## Conventions

- Keep live tests behind the `integration` build tag.
- Prefer explicit selectors and service-specific success conditions over broad URL-only checks for native login flows.
- Add new services to `flows.go` only when there is a real SSO, first-run, or proxy-gated behavior to prove.
- Redact passwords and tokens from evidence logs.

## Security and Risk

- These tests use live Authentik state and may recover or repair bootstrap token state inside the `authentik-server` container.
- Browser tests can swap Turnstile captcha stages to test keys.
- Do not print recovered tokens, admin passwords, or values loaded from `.env`.

## Agent Workflow

- If smoke tests fail, identify whether the failure is config, endpoint reachability, browser selector drift, service first-run state, or product-edition limits.
- For browser failures, inspect the action log and screenshot evidence path before changing selectors or installer code.
- Keep unit tests for helper behavior separate from live integration assertions.
