# Homelab Architecture Audit

Repository: `/home/caboose/dev/caboose-ai.io`
Generated: `2026-05-10`

## Executive Summary

This read-only audit checks the current homelab service surface across service manifests, Docker Compose, servicebuilder registration, Authentik provider/proxy specs, dashboard inclusion, smoke-test flows, and docs.

Active service manifests: Authentik, cAdvisor, Forgejo, Ghost, Grafana, Homarr, Loki, Mattermost, Open WebUI, OpenClaw, Paperclip, Portainer, Prometheus, Promtail, Social Login, SonarQube, Woodpecker.

Findings: 2 high, 4 medium, 0 low.

## Service Matrix

| Service | Compose | Configurator | URL key | SSO | Smoke | Dashboard | Docs |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `authentik` | ok | - | authentik | identity provider | unresolved | unset | ok |
| `cadvisor` | ok | - | - | - | unresolved | - | ok |
| `forgejo` | ok | built | forgejo | oidc provider: forgejo | browser oauth | unset | ok |
| `ghost` | ok | - | ghost | proxy: ghost-proxy | unresolved | unset | ok |
| `grafana` | ok | built | grafana | oidc provider: grafana | browser oauth | unset | ok |
| `homarr` | ok | built | dashboard | oidc provider: homarr | browser oauth | unset | ok |
| `loki` | ok | - | - | - | unresolved | - | ok |
| `mattermost` | ok | built | mattermost | oidc provider: mattermost | browser oauth | unset | ok |
| `open-webui` | ok | built | open_webui | oidc provider: open-webui | browser oauth | unset | ok |
| `openclaw` | missing openclaw | - | openclaw | proxy: openclaw-proxy | browser proxy | unset | ok |
| `paperclip` | ok | built | paperclip | proxy: paperclip-proxy | unresolved | unset | ok |
| `portainer` | ok | built | portainer | oidc provider: portainer | browser oauth | unset | ok |
| `prometheus` | ok | - | - | - | unresolved | - | ok |
| `promtail` | ok | - | - | - | unresolved | - | ok |
| `social` | ok | built | authentik | social sources | unresolved | - | ok |
| `sonarqube` | ok | built | sonarqube | local/service auth | unresolved | unset | ok |
| `woodpecker` | ok | built | woodpecker | proxy: ci-proxy | unresolved | unset | ok |

## Findings

### 1. High: Manifest compose services are missing from Compose

- References: `services/openclaw/service.yaml`, `dev/homelab/docker-compose.yml`
- Detail: openclaw: openclaw
- Suggested fix: Either add the missing compose service definitions or mark the service as external in a documented manifest field.

### 2. High: Per-service smoke command ignores the manifest smoke_flow value

- References: `internal/cli/service_command.go:148`
- Detail: `homelab service <slug> smoke` runs the same config test for every service instead of selecting the manifest's declared flow.
- Suggested fix: Route manifest smoke_flow to a flow-specific test or make the command explicit that it is a config-only SSO check.

### 3. Medium: Dashboard filtering contains stale removed-service entries

- References: `internal/servicebuilder/builder.go:92`
- Detail: 1 exclusion entry no longer maps to a service manifest or service link.
- Suggested fix: Move dashboard visibility into service manifests or prune stale hard-coded exclusions during service removal.

### 4. Medium: Live-state mutation paths need durable approval guardrails

- References: `services/homarr/homarr.go:151`, `internal/install/install.go:227`
- Detail: Homarr board seeding mutates SQLite through Docker exec and reset removes secrets/env-derived state.
- Suggested fix: Use hooks or command wrappers to require explicit confirmation before live DB, secret, reset, or destructive Docker paths.

### 5. Medium: Service metadata is duplicated across manifests, URLs, and dashboard filtering

- References: `services/*/service.yaml`, `internal/config/urls.go:46`, `internal/servicebuilder/builder.go:92`
- Detail: URL keys, display names, dashboard visibility, provider specs, and manifest docs are maintained in separate files.
- Suggested fix: Promote URL/dashboard/SSO attributes into the service manifest or generate derived tables from one registry.

### 6. Medium: Several manifest smoke flows are not backed by browser flow coverage

- References: `internal/smoketest/flows.go`, `services/authentik/service.yaml`, `services/cadvisor/service.yaml`, `services/ghost/service.yaml`, `services/loki/service.yaml`, `services/paperclip/service.yaml`, `services/prometheus/service.yaml`, `services/promtail/service.yaml`, `services/social/service.yaml`, `services/sonarqube/service.yaml`, `services/woodpecker/service.yaml`
- Detail: authentik (unresolved), cadvisor (unresolved), ghost (unresolved), loki (unresolved), paperclip (unresolved), prometheus (unresolved), promtail (unresolved), social (unresolved), sonarqube (unresolved), woodpecker (unresolved)
- Suggested fix: Add matching smoke flow definitions or downgrade manifest smoke_flow values to the actual config-only coverage they receive.

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

- Files read: 57
- Scope: whitelisted repo source, docs, manifests, and compose files only.
- Excluded: `.env` files, generated smoke evidence, screenshots, logs, JSONL evidence, local agent/editor state, and live Docker/Authentik state.

## Validation Commands

```bash
env UV_CACHE_DIR=/tmp/uv-cache uv run python /home/caboose/.agents/skills/homelab-architecture-auditor/scripts/audit_homelab_architecture.py . --check-safety
go test ./internal/servicebuilder ./services/homarr
git diff --check
```
