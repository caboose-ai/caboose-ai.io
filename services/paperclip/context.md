# Caboose AI Software Shop Context

Mission: Operate, improve, deploy, monitor, and secure caboose-ai.io and its homelab services as a high-skill software development shop.

Primary repo: `/home/caboose/dev/caboose-ai.io`

Read first:
- `README.md`
- `CLAUDE.md`
- `.github/copilot-instructions.md`
- `services/paperclip/service.yaml`
- `services/paperclip/README.md`
- `docs/homelab-agent-control-plan.md`
- `dev/homelab/docker-compose.yml`
- `mise.toml`

Connection contract:
- SSO mode: Authentik forward-auth through `paperclip-proxy`
- Public URL: `https://paperclip.caboose-ai.io`
- Compose profile: `paperclip`
- Health path: `/api/health`
- Smoke flow: `paperclip`
- Dashboard: manifest-owned `dashboard.show: true`

Project areas:
- Homelab Core: installer, reset, compose, Caddy, Authentik
- SSO and Identity: OAuth/OIDC, proxy apps, smoke flows
- Observability: Prometheus, Loki, Grafana, health checks
- Service Workspaces: per-service manifests, configuration, docs
- Delivery: Forgejo, Woodpecker, PRs, release verification
- Agent Control Plan: plan-only v1 task intake, approval gates, execution
  evidence, and follow-up review using Forgejo/Gitea, Woodpecker, and
  Portainer as self-hosted delivery/control surfaces

Authority:
- Default mode is plan-only: agents may inspect, branch, test, commit, open PRs, query monitoring, and propose deploy actions.
- Direct infra execution remains approval-gated.
- Docker and Portainer mutations, installer reset, production deploy, secret changes, firewall changes, and destructive filesystem/data mutations require explicit human approval.
- Recurring work must stay within budgets and leave an audit trail in Paperclip.
