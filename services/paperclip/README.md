# Paperclip

Paperclip is an optional multi-agent workspace service behind the compose
`paperclip` profile. It is exposed at `paperclip.caboose-ai.io`, protected by
the Authentik forward-auth provider `paperclip-proxy`, and shown on Homarr once
the targeted `paperclip` smoke flow is part of the repo.

## Operations

```bash
mise run paperclip:up
mise run paperclip:status
mise run paperclip:smoke
mise run paperclip:seed
```

`paperclip:seed` uses `homelab paperclip seed-company --profile software-shop`
and reads `PAPERCLIP_API_KEY` from the environment or configured secret store
when the API requires bearer auth.

If `PAPERCLIP_PUBLIC_URL` is overridden for a non-default domain, set
`PAPERCLIP_ALLOWED_HOSTNAMES` to the matching hostname.

## Service Contract

Good:

```yaml
smoke_flow: paperclip
dashboard:
  show: true
sso:
  mode: proxy
health:
  url_key: paperclip
  path: /api/health
```

Bad:

```yaml
smoke_flow: paperclip
# No executable smoke flow, dashboard visibility is hidden in Go code,
# and health points at a UI page instead of the app health endpoint.
health:
  path: /
```
