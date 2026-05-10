# Homarr Service Guidelines

## Scope

These instructions apply to `services/homarr/`.

## Patterns

- Keep Homarr setup idempotent. `Configure` should be safe to rerun and should
  repair installer-owned dashboard state when credentials already exist.
- Treat app IDs prefixed with `homelab_app_` and item IDs prefixed with
  `homelab_item_` as installer-owned records.
- Keep dashboard app selection in `internal/servicebuilder` and seed rendering
  in `services/homarr`; do not duplicate dashboard app lists elsewhere.
- Preserve the Open WebUI dashboard URL override to `/oauth/oidc/login` unless
  a test proves the root URL no longer emits unauthenticated noise.
- Use structured JSON and prepared SQLite statements in embedded Node scripts.
- Docker-backed SQLite changes mutate live Homarr state; require explicit human
  approval before doing them outside tests or planned repair work.

## Good And Bad Examples

Good:

```go
apps := servicebuilder.DashboardApps(cfg)
homarr.New(ak, secrets, url, httpClient, dockerExec, "homarr", apps...)
```

Bad:

```go
apps := []homarr.App{{Name: "Forgejo"}, {Name: "Grafana"}, {Name: "Old Service"}}
```

Good:

```sql
DELETE FROM items WHERE id LIKE 'homelab_item_%';
```

Bad:

```sql
DELETE FROM items;
DELETE FROM apps;
```

## Validation

- `go test ./services/homarr -v`
- `go test ./internal/servicebuilder ./services/homarr -v`
- For live repairs, verify both the SQLite rows and browser-visible dashboard
  behavior when feasible.
