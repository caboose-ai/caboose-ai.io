# Service Workspace Guidelines

## Scope

These instructions apply to every `services/<slug>/` workspace. Nested guidance
may add stricter rules for service-specific risk.

## Patterns

- Every service workspace should have `service.yaml` and `README.md`.
- The manifest `slug` must match the directory name and service slug.
- Keep manifest fields aligned with compose, URLs, secrets, health checks,
  configurator registration, smoke tests, and docs.
- Use `dashboard.show` for Homarr inclusion and `sso.mode` for the service auth
  contract. Do not duplicate dashboard visibility in Go allow/deny lists.
- Use `runtime: external` only when a service is deliberately managed outside
  local compose; external services should not list fake compose services.
- Use `configurator` only when the service has Go setup logic registered in
  `internal/servicebuilder`.
- Use `smoke_flow` only when `internal/smoketest` can execute that named flow;
  health-only services should rely on health checks instead.
- Keep service READMEs operational: what the service is, how it is exposed,
  what Authentik mode it uses, secrets it needs, and how to verify it.
- Configurator packages should implement `service.ServiceConfigurator` and use
  injected clients/stores for external effects.

## Good And Bad Examples

Good:

```yaml
slug: portainer
display_name: Portainer
url_key: portainer
compose_services:
  - portainer
secrets:
  - PORTAINER_ADMIN_PASSWORD
configurator: portainer
smoke_flow: portainer
dashboard:
  show: true
sso:
  mode: oidc
health:
  url_key: portainer
  path: /
  kind: http
docs:
  - README.md
```

Bad:

```yaml
slug: docker-ui
display_name: Portainer
compose_services:
  - portainer
smoke_flow: portainer
# slug does not match the directory, dashboard/SSO ownership is missing, and
# docs/health/secrets are incomplete.
```

Good:

```yaml
slug: paperclip
display_name: Paperclip
url_key: paperclip
compose_services:
  - paperclip
  - paperclip-db
configurator: paperclip
smoke_flow: paperclip
dashboard:
  show: true
sso:
  mode: proxy
health:
  url_key: paperclip
  path: /api/health
  kind: http
```

Bad:

```yaml
slug: paperclip
display_name: Paperclip
smoke_flow: paperclip
health:
  path: /
# The flow must resolve in internal/smoketest, dashboard visibility should not
# be hidden in servicebuilder, and health should use the app health endpoint.
```

Good:

```go
func (c *Configurator) CheckConfigured(ctx context.Context) (bool, error) {
    secret, err := c.Secrets.Get(ctx, "SERVICE_CLIENT_SECRET")
    if err != nil {
        return false, err
    }
    return secret != "", nil
}
```

Bad:

```go
func (c *Configurator) CheckConfigured(context.Context) (bool, error) {
    return os.Getenv("SERVICE_CLIENT_SECRET") != "", nil
}
```

## Validation

- `go test ./services/...`
- `go test ./internal/service ./internal/servicebuilder`
- If `smoke_flow` changes: `go test ./internal/smoketest`
- If compose or live setup changes: run the relevant `mise run service:*` or
  SSO smoke command from the root guidance.
