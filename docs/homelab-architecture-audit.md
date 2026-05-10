# Homelab Architecture Audit

Repository: `/home/caboose/dev/caboose-ai.io`
Generated: `2026-05-10`

## Executive Summary

This read-only audit checks the current homelab service surface across service manifests, Docker Compose, servicebuilder registration, Authentik provider/proxy specs, dashboard inclusion, smoke-test flows, and docs.

Active service manifests: Authentik, cAdvisor, Forgejo, Ghost, Grafana, Homarr, Loki, Mattermost, Open WebUI, OpenClaw, Paperclip, Portainer, Prometheus, Promtail, Social Login, SonarQube, Woodpecker.

Findings: 0 high, 0 medium, 0 low.

## Service Matrix

| Service | Compose | Configurator | URL key | SSO | Smoke | Dashboard | Docs |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `authentik` | ok | - | authentik | identity provider | browser proxy | shown | ok |
| `cadvisor` | ok | - | - | - | - | - | ok |
| `forgejo` | ok | built | forgejo | oidc | browser oauth | shown | ok |
| `ghost` | ok | - | ghost | proxy | browser proxy | hidden | ok |
| `grafana` | ok | built | grafana | oidc | browser oauth | shown | ok |
| `homarr` | ok | built | dashboard | oidc | browser oauth | shown | ok |
| `loki` | ok | - | - | - | - | - | ok |
| `mattermost` | ok | built | mattermost | local/service auth | browser oauth | hidden | ok |
| `open-webui` | ok | built | open_webui | oidc | browser oauth | shown (/oauth/oidc/login) | ok |
| `openclaw` | external | - | openclaw | proxy | browser proxy | shown | ok |
| `paperclip` | ok | built | paperclip | proxy | browser proxy | shown | ok |
| `portainer` | ok | built | portainer | oidc | browser oauth | shown | ok |
| `prometheus` | ok | - | - | - | - | - | ok |
| `promtail` | ok | - | - | - | - | - | ok |
| `social` | ok | built | authentik | social sources | browser proxy | - | ok |
| `sonarqube` | ok | built | sonarqube | local/service auth | - | hidden | ok |
| `woodpecker` | ok | built | woodpecker | proxy | browser proxy | shown | ok |

## Findings

## Automation Recommendations

### Hooks

- Add a stop-point branch/scope hook for edits touching `services/*/service.yaml`, `dev/homelab/docker-compose.yml`, `internal/servicebuilder`, `internal/install/providers.go`, `internal/install/outpost.go`, or `internal/smoketest/flows.go`.
- Add a pre-tool safety hook that blocks `.env`, generated smoke evidence, screenshots, logs, JSONL, and live-state mutation commands unless the user explicitly asks for them.

### Skills

- Use `homelab-architecture-auditor` before and after service additions/removals or SSO/dashboard changes.
- Keep `repo-agent-guidance-generator` for broader AGENTS/CLAUDE guidance refreshes; use this audit when the question is service architecture consistency.

### Subagents

- Service-contract reviewer: check manifest, compose, URL, provider/proxy, servicebuilder, smoke, and docs changes as one unit.
- SSO-smoke reviewer: inspect Authentik provider/proxy flows and browser smoke coverage after login-path changes.

### MCPs

- GitHub MCP for PR/check triage and review-thread resolution on architecture changes.
- Playwright MCP for browser verification when a service login or dashboard route changes.

## Scanner Safety

- Files read: 56
- Scope: whitelisted repo source, docs, manifests, and compose files only.
- Excluded: `.env` files, generated smoke evidence, screenshots, logs, JSONL evidence, local agent/editor state, and live Docker/Authentik state.

## Validation Commands

```bash
env UV_CACHE_DIR=/tmp/uv-cache uv run python /home/caboose/.agents/skills/homelab-architecture-auditor/scripts/audit_homelab_architecture.py . --check-safety
go test ./internal/servicebuilder ./services/homarr
git diff --check
```
