# caboose-ai.io

The manifests for everything served under `caboose-ai.io`: a single-node
**k3s** cluster in a home office, reached through a **Cloudflare Tunnel**.
No forwarded ports, no origin IP in DNS.

This checkout *is* the deploy directory, `~/apps/k3s` on the node. What is
committed here is what the cluster should be running.

> **History:** everything before the `pre-k3s-archive` tag is the previous,
> retired Docker Compose homelab (Authentik, Forgejo, Woodpecker, Homarr). It
> was wiped in the 2026-07-15 factory reset and none of it runs anymore.

## Traffic

```
you → Cloudflare edge → tunnel (cloudflared ×2) → traefik → Service → pod
```

The tunnel is remotely managed in Cloudflare. Its ingress rules route
`caboose-ai.io` and `*.caboose-ai.io` to traefik, and the zone holds a matching
proxied wildcard CNAME. **So a new app needs no Cloudflare change**: apply an
Ingress with a single-level host and it serves.

**Hostnames must be single-level** (`app.caboose-ai.io`). Free Universal SSL
does not cover a second label, so `x.y.caboose-ai.io` fails the TLS handshake.

Game servers are the exception to the tunnel: Cloudflare's free plan won't
proxy arbitrary TCP/UDP, so Minecraft and Palworld go through a playit.gg
relay (`minecraft/playit.yaml`) with DNS-only records.

## What's here

| Path | Host | Notes |
|---|---|---|
| `landing/` | `caboose-ai.io` | nginx serving static HTML from a ConfigMap |
| `moto-edit/` | `edit.`, `s.caboose-ai.io` | app source: `caboose-ai/moto-edit` |
| `caboose-fit/` | `fit.caboose-ai.io` | app source: `caboose-ai/caboose-fit` |
| `minecraft/` | `mc.caboose-ai.io` (SRV) | Fabric; idles at 0 replicas |
| `palworld/` | `pw.caboose-ai.io:58417` | idles at 0 replicas |
| `monitoring/` | `grafana.caboose-ai.io` | Prometheus, Loki, Alloy, Grafana |
| `machine-mcp/` | `machine.caboose-ai.io` | |
| `mcp-bridge/` | `mcp.caboose-ai.io` | app source: `mcp-grpc-bridge` |
| `caboose-health.yaml` | `health.caboose-ai.io` | host service fronted by an Ingress |
| `whoami.yaml` | `labs.caboose-ai.io/whoami` | ingress smoke test |
| `cloudflared.yaml` | — | tunnel connectors |

App manifests are **copied here** by each app repo's `deploy/build.sh`. The app
repo is the source of truth for its own manifest; edit it there, rebuild, and
the copy here follows. Commit the copy afterwards so this repo matches the
cluster.

## Secrets

**None are in this repo, and none may be.** Every credential is a Kubernetes
Secret created out-of-band with `kubectl create secret`; manifests only
reference them by name through `secretKeyRef`. The comment at the top of each
manifest gives the exact command. Values live in 1Password.

## Conventions

- Images come from the node's local registry, `localhost:5000/<app>:<tag>`.
  A bare `<app>:<tag>` makes k3s try Docker Hub and fail with `ImagePullBackOff`.
- The `image:` line must match what is deployed. `kubectl set image` does not
  write back here, and a later `kubectl apply` would roll production back.
- Apps expose `/metrics` on a **separate port** named in the
  `prometheus.io/port` annotation, not the app port. The Ingress fronts every
  path on the app port, so a metrics route there is public.
- Stateful apps use a ReadWriteOnce PVC with `strategy: Recreate`.

## Landing page

```sh
cd landing
kubectl create configmap landing-html \
  --from-file=index.html --from-file=moto-edit.html \
  --from-file=privacy.html --from-file=tos.html \
  -o yaml --dry-run=client | kubectl apply -f -
kubectl rollout restart deploy/landing
```

Include **every** HTML file. Recreating the ConfigMap from a partial list
silently deletes the pages you left out.
