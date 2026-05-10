# OpenClaw

OpenClaw is externally managed on this host. The manifest tracks its URL,
Authentik forward-auth proxy, dashboard visibility, smoke flow, health check,
and docs without claiming a local Docker Compose service.

Good:

```yaml
runtime: external
url_key: openclaw
smoke_flow: openclaw
sso:
  mode: proxy
```

Bad:

```yaml
compose_services:
  - openclaw
# No matching service exists in dev/homelab/docker-compose.yml.
```
