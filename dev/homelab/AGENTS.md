# Homelab Stack Guidelines

## Scope

These instructions apply to `dev/homelab/`.

## Patterns

- `docker-compose.yml` is the canonical local stack definition.
- Use `${VAR}` substitution for secrets and runtime values; do not hardcode real
  values into compose, scripts, examples, or docs.
- Keep database-only networks marked `internal: true`.
- App-facing services that need Authentik token exchange should join the `apps`
  network.
- Use `HOMELAB_BIND_ADDRESS` and `serve_mode` behavior for host port exposure
  instead of hardcoding `0.0.0.0` or `127.0.0.1`.
- Optional services that should not block default startup should use Compose
  profiles.
- Prefer Go installer paths in `cmd/homelab` and `internal/install` for new
  behavior. Keep shell scripts here for legacy compatibility or diagnosis.
- Direct reset flows must be explicit: `homelab reset` requires `--yes` unless
  it is run with `--dry-run`.

## Good And Bad Examples

Good:

```yaml
ports:
  - "${HOMELAB_BIND_ADDRESS:-127.0.0.1}:3000:3000"
networks:
  - apps
  - service-internal
```

Bad:

```yaml
ports:
  - "0.0.0.0:3000:3000"
environment:
  ADMIN_PASSWORD: "real-password"
```

Good:

```yaml
networks:
  service-internal:
    internal: true
```

Bad:

```yaml
networks:
  service-internal: {}
```

Good:

```yaml
profiles:
  - paperclip
environment:
  PAPERCLIP_DEPLOYMENT_MODE: authenticated
  PAPERCLIP_DEPLOYMENT_EXPOSURE: public
  PAPERCLIP_AUTH_PUBLIC_BASE_URL: "${PAPERCLIP_PUBLIC_URL:-https://paperclip.caboose-ai.io}"
  PAPERCLIP_ALLOWED_HOSTNAMES: "${PAPERCLIP_ALLOWED_HOSTNAMES:-paperclip.caboose-ai.io}"
```

Bad:

```yaml
environment:
  PAPERCLIP_DEPLOYMENT_EXPOSURE: private
  BETTER_AUTH_SECRET: "plaintext-secret"
# Public reverse-proxied Paperclip needs explicit public auth URL settings, and
# secrets must come from env substitution or the managed secret store.
```

## Validation

- `docker compose -f dev/homelab/docker-compose.yml config --services`
- `docker compose --profile paperclip -f dev/homelab/docker-compose.yml config --services`
- `HOMELAB_COMPOSE_DIR=dev/homelab mise run install`
- `mise run sso:check-quick`
