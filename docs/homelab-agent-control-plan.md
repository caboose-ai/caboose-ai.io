# Homelab Service Documentation and Agent Control Plan

This plan turns the caboose-ai homelab into an agent-assisted software shop with
service documentation first, then an OpenClaw-centered control surface that can
also be driven from Telegram. The goal is to make every service understandable
to humans and agents before granting agents the ability to coordinate setup,
operations, and development work.

## Outcomes

1. Every service has a current capability profile that can be read by humans,
   the Homelab CLI, the MCP server, OpenClaw, and Telegram Agent Bridge.
2. OpenClaw becomes the primary control surface for planning, routing, and
   supervising homelab tasks.
3. Telegram becomes a private remote-control layer for quick task dispatch,
   status checks, approvals, and readiness notifications.
4. Agent work defaults to local or self-hosted execution paths so outside token
   use is reserved for tasks that explicitly require an external provider.
5. Subagentic development can create, test, and ship new skills, services,
   apps, websites, APIs, and automations with human approval gates.

## Guiding Principles

- Document before automating: an agent must be able to answer what a service is,
  how it is exposed, how it authenticates, what secrets it needs, how to verify
  it, and what it can safely do before it can operate that service.
- Keep facts close to the service: registry facts live in
  `services/<slug>/service.yaml`; operational notes and capabilities live in
  `services/<slug>/README.md`; cross-service workflows live under `docs/`.
- Prefer local-first execution: route agent work through Ollama, local tools,
  the Homelab MCP server, and repo-native scripts before spending external AI,
  SaaS, or API tokens.
- Use least privilege: separate read-only discovery, write-capable setup,
  deploy-capable operations, and destructive maintenance behind explicit roles
  and confirmations.
- Make evidence durable: every automated change should leave logs, test output,
  smoke evidence, PR notes, or service documentation updates that another agent
  can consume later.

## Phase 1: Document Every Service

### 1. Create a Standard Service Capability Template

Extend every `services/<slug>/README.md` with these sections:

- Purpose: what the service does in the homelab.
- Capability profile: what humans and agents can use the service for.
- Control surfaces: UI, CLI, API, MCP tool, webhook, compose service, or host
  command entrypoints.
- Exposure and authentication: URL, dashboard visibility, Authentik mode,
  Telegram allowlist, local-only status, or external runtime boundary.
- Dependencies: compose services, external binaries, upstream services,
  persistent volumes, networks, and required DNS routes.
- Secrets and tokens: required secret names, storage expectations, rotation
  notes, and whether the secret spends external tokens.
- Setup path: first-run setup, configurator behavior, and manual fallback.
- Verification path: health check, smoke flow, useful CLI command, expected
  browser behavior, and logs to inspect.
- Agent permissions: safe read actions, safe write actions, confirmation-gated
  actions, and forbidden actions.
- Failure modes: common symptoms, likely causes, and rollback steps.

### 2. Build the Initial Capability Inventory

| Service | Primary capability | Agent use | Auth/exposure | Documentation priority |
| --- | --- | --- | --- | --- |
| Authentik | Identity provider, SSO, sources, forward auth | Create providers, verify login paths, inspect SSO health | Dashboard; identity provider | Highest because every protected service depends on it |
| Homarr | Homelab dashboard | Discover service links and dashboard visibility | Dashboard; OIDC | High because it is the human navigation hub |
| OpenClaw | Agent interface and model gateway | Central planning, task routing, local model selection, supervised execution | External runtime; proxy SSO | Highest because it is the target control surface |
| Telegram Agent Bridge | Private bot control plane | Remote `/ask`, `/agent`, status, approval, notification workflows | External runtime; Telegram allowlist | Highest because it is the remote approval and dispatch path |
| Homelab MCP | Automation API for agents | Service status, smoke checks, local tool execution, agent invocation | Bearer-token MCP endpoint | Highest because it is the safest machine interface |
| Open WebUI | Local/self-hosted chat UI | Model validation, local LLM testing, user-facing AI experiments | Dashboard; OIDC | High for minimizing outside token use |
| Paperclip | AI-labor control plane | Trigger plans, track execution, manage software-shop tasks, coordinate agent experiments | Dashboard; proxy SSO | High for subagentic development workflows |
| Forgejo | Git hosting | Repo creation, code review context, issues, PRs, release source | Dashboard; OIDC | High for development workflows |
| Woodpecker | CI/CD | Build/test/deploy pipelines, status feedback, release automation | Dashboard; proxy SSO | High for development workflows |
| SonarQube | Code quality and security | Static analysis findings, quality gates, remediation tasks | Local auth | Medium-high for code quality agents |
| Portainer | Docker management UI | Container inventory, logs, restarts, volume/network inspection | Dashboard; OIDC | Medium-high; write operations need confirmation |
| Grafana | Observability dashboards | Metrics triage, dashboard links, alert context | Dashboard; OIDC | Medium-high for operations agents |
| Prometheus | Metrics collection | Query metrics, health signals, alert inputs | Local-only/no SSO | Medium for operations agents |
| Loki | Log aggregation | Query logs, correlate incidents, summarize failures | Local-only/no SSO | Medium for operations agents |
| Promtail | Log shipping | Verify log ingestion path | Container health only | Medium-low; mostly dependency documentation |
| cAdvisor | Container metrics | Container resource data for triage | Local-only/no SSO | Medium-low; mostly dependency documentation |
| Mattermost | Team chat | Human coordination, status summaries, future chatops bridge | Local auth | Medium; decide whether to keep or replace with Telegram-first flows |
| Ghost | Blog/publishing | Publish docs, changelogs, public project notes | Proxy SSO | Low-medium; useful after development workflows mature |
| Social Login | Authentik GitHub/Google sources | Verify and repair external identity sources | Authentik source config | Medium; secrets can depend on external providers |

### 3. Add Machine-Readable Capability Metadata

After the README template is complete, add optional service metadata that agents
can load without scraping prose. A future manifest extension could include:

```yaml
capabilities:
  - identity
  - git
  - ci
agent:
  read:
    - status
    - logs
  write:
    - configure
  confirm:
    - restart
    - deploy
    - rotate_secrets
  forbidden:
    - delete_volumes_without_backup
cost_profile:
  external_tokens: none
  external_api_calls: optional
```

Keep this additive and backward-compatible. The first implementation can be a
`services/<slug>/capabilities.yaml` sidecar if changing `service.yaml` parsing
would slow down documentation work.

### 4. Validate the Documentation Set

Create a documentation check that ensures each service has:

- A manifest with `slug`, `display_name`, `dashboard`, `sso`, `health`, and
  `docs` fields where applicable.
- A README with the standard capability sections.
- A verification command or explicit reason no command exists.
- A declared agent permission tier.
- A token-cost note that labels local-only, self-hosted, external-free,
  external-optional, or external-required behavior.

## Phase 2: Connect Documentation to OpenClaw and Telegram

### 1. Define the Control Plane Roles

- OpenClaw: primary interactive interface for humans to plan and supervise
  work, choose models, route tasks, and review agent output.
- Telegram Agent Bridge: private remote command layer for short prompts,
  role-based agent dispatch, confirmations, and readiness notifications.
- Homelab MCP: typed tool layer for safe service status, smoke checks, logs,
  repo workflows, and agent invocation.
- Paperclip: planning and execution workspace for longer-running software-shop
  work; it can create task records, hold audit trails, assign subagent roles,
  and hand approved execution steps to OpenClaw, Telegram, Homelab MCP, or the
  Homelab CLI.
- Homelab CLI: deterministic local executor for service operations, install,
  configure, smoke, logs, and open commands.
- Forgejo and Woodpecker: source control and CI feedback loop for development
  tasks.
- Observability stack: Grafana, Prometheus, Loki, Promtail, and cAdvisor provide
  incident context and verification signals.

### 2. Route Requests Through a Task Classifier

OpenClaw should classify each request before invoking agents or tools:

| Task class | Example | Default route | Human gate |
| --- | --- | --- | --- |
| Discovery | "What services are unhealthy?" | Homelab MCP read-only tools | None |
| Documentation | "Document Paperclip capabilities" | Docs agent + repo tools | PR review |
| Setup | "Configure Grafana SSO" | Service setup agent + Homelab CLI/MCP | Before writes |
| Operations | "Restart Woodpecker" | Ops agent + Homelab CLI | Required for restart/deploy |
| Development | "Build a new API" | Architect, implementer, tester, reviewer subagents | PR and deploy approval |
| Incident | "Why is Forgejo down?" | Triage agent + observability tools | Before destructive repair |
| Token-spending | "Use external model/API" | Cost-aware router | Required unless pre-approved |

Telegram commands should use the same classifier but default to shorter, safer
operations. Any write, deploy, restart, commit, push, merge, reset, delete, or
external-token-spending action should require a confirmation command.

### 3. Build a Service Context Index

Generate an index that OpenClaw and Telegram agents can load before acting:

1. Parse every `services/<slug>/service.yaml`.
2. Load each documented README capability profile.
3. Attach relevant docs from `docs/`.
4. Emit a compact service card per service with:
   - slug and display name;
   - URLs and exposure;
   - SSO mode;
   - health and smoke checks;
   - secrets and external token notes;
   - agent permission tiers;
   - known dependencies;
   - runbook links.
5. Publish the index to the MCP server as a read-only resource and to OpenClaw
   as context.

### 4. Establish Telegram Workflows

Start with conservative commands:

- `/status`: show unhealthy services, recent CI failures, and current agent
  queue state.
- `/ask <prompt>`: answer using the service context index and local-first model
  routing.
- `/agent <role> <task>`: draft a plan without writes by default.
- `/agent confirm <role> <task>`: allow write-capable tasks after the command
  repeats the intended changes and risks.
- `/model`: show current model and whether it is local, self-hosted, or
  external-token-backed.
- `/pr <repo> <id>`: summarize readiness, checks, requested changes, and review
  state.

The bot should return links to OpenClaw sessions or PRs for long-running work
instead of trying to fit the full working context in Telegram.

### 5. Trigger Planning and Execution from Paperclip

Yes: Paperclip can be the project-management trigger for this system, while
OpenClaw remains the primary interactive control surface and Homelab MCP remains
the typed automation boundary. The recommended Paperclip flow is:

1. Intake: a human creates or updates a Paperclip task with the goal, services
   involved, constraints, token budget, and desired output.
2. Plan: Paperclip invokes the architect or service-documenter role to create a
   plan, risk label, required tools, and verification checklist.
3. Approve: Paperclip records human approval for any write, deploy, restart,
   secret, destructive, or external-token-spending action.
4. Execute: approved steps are dispatched through OpenClaw, Telegram Agent
   Bridge, Homelab MCP, or the Homelab CLI rather than through ad-hoc service
   access.
5. Verify: tester or operator agents attach command output, smoke results, CI
   status, screenshots when useful, and rollback notes to the Paperclip task.
6. Close the loop: Paperclip links the branch, PR, OpenClaw session, Telegram
   notification, service docs, and follow-up work so future agents can reuse the
   evidence.

Paperclip should start as a planner and audit log for execution rather than a
privileged executor. Direct execution from Paperclip is safe only after the task
classifier, service context index, confirmation gates, and least-privilege MCP
tools are in place. Until then, Paperclip-triggered work should produce plans,
PRs, and explicit handoffs for humans or confirmed agents to execute.

## Phase 3: Minimize Outside Token Use

### 1. Create a Cost-Aware Model Router

Use this routing order unless a task explicitly requests otherwise:

1. Deterministic tools and scripts: shell, Go tests, service CLI, MCP tools,
   smoke tests, linters, and static checks.
2. Local models through Ollama or Open WebUI for summarization, classification,
   first-pass code edits, and documentation drafts.
3. Self-hosted or already-authenticated OpenClaw gateway models when local
   models are insufficient.
4. External models only for high-complexity implementation, review, or tasks
   where a human confirms the token spend.

Every agent response should label its model path as `local`, `self-hosted`,
`external-free`, `external-optional`, or `external-spend`.

### 2. Cache and Reuse Context

- Maintain service cards so agents do not rediscover static facts repeatedly.
- Store resolved runbooks and smoke outcomes as evidence artifacts.
- Summarize long logs once, then reuse the summary with a pointer to the raw
  log command.
- Prefer repo-local docs and generated indexes over web searches.
- Keep reusable prompts for common roles such as service documenter, SSO setup,
  CI fixer, observability triage, and release assistant.

### 3. Separate External Secrets From Agent Defaults

Classify secrets as:

- local-only runtime secrets;
- self-hosted service credentials;
- external OAuth credentials;
- external AI/API token spend credentials.

Agents should be able to detect that a secret exists without reading the secret
value unless the task explicitly requires using it through a safe interface.

## Phase 4: Subagentic Development Workflow

### 1. Define Default Subagents

- Architect: turns a human goal into a design, service boundaries, and task
  breakdown.
- Service documenter: updates service READMEs, capability metadata, and runbooks.
- Implementer: writes code, service manifests, compose changes, or app files.
- Integrator: wires SSO, secrets, dashboards, MCP tools, and smoke flows.
- Tester: runs unit, build, smoke, and browser checks; records evidence.
- Reviewer: checks diffs, security concerns, token use, and rollback plans.
- Operator: performs deploy, restart, backup, restore, and incident actions only
  after confirmation.
- Release assistant: prepares PRs, changelogs, CI status summaries, and release
  notes.

### 2. Use a Standard Task Lifecycle

1. Intake: capture goal, constraints, token budget, services involved, and
   desired artifact.
2. Classify: identify task class, risk level, agent roles, required tools, and
   human gates.
3. Plan: create a short implementation plan with expected files, services, and
   checks.
4. Prepare context: load service cards, relevant docs, current repo state, and
   prior evidence.
5. Execute in branches: keep code changes isolated in Git branches and avoid
   touching unrelated user work.
6. Verify: run the smallest useful check first, then broader tests and smoke
   flows as risk increases.
7. Review: require a reviewer subagent or human to inspect security, data loss,
   external token use, and rollback notes.
8. Publish: open a PR with summary, tests, screenshots when applicable, and
   follow-up tasks.
9. Notify: send Telegram readiness updates and link back to the OpenClaw session
   or PR.
10. Learn: update docs, service cards, and prompts with any newly discovered
    facts.

### 3. Apply the Workflow to New Things

For new skills:

- Draft the skill contract, trigger criteria, allowed tools, and validation
  steps.
- Build a minimal local skill first.
- Test it against repo-local fixtures before allowing external API access.
- Document when it should be used and when it should be skipped.

For new services:

- Create `services/<slug>/service.yaml` and README first.
- Add compose or external runtime integration.
- Add SSO mode, dashboard policy, health check, and smoke path.
- Add configurator code only when setup cannot stay declarative.
- Add rollback and backup notes before enabling agent operations.

For new apps, websites, and APIs:

- Start with architecture and threat model notes.
- Decide whether the app belongs as a homelab service, a Forgejo repository, or
  an external deployment.
- Add CI, tests, observability, and release path before production exposure.
- Prefer local development loops and local model assistance before external
  token-backed agents.

## Implementation Roadmap

### Milestone A: Documentation Foundation

- Add the standard capability template to all service READMEs.
- Create the first service capability inventory from manifests and READMEs.
- Add token-cost and agent-permission sections per service.
- Add a documentation validation script or test.

### Milestone B: Context Index and Read-Only Agents

- Generate service cards from service manifests and README sections.
- Expose the cards through Homelab MCP as read-only resources.
- Teach OpenClaw and Telegram `/ask` to answer from service cards.
- Add read-only `/status` that reports health, smoke availability, and CI state.

### Milestone C: Controlled Write Workflows

- Add task classification and risk scoring.
- Add confirmation-gated Telegram and OpenClaw write operations.
- Add service-scoped agent roles that call Homelab CLI or MCP tools instead of
  ad-hoc shell commands when possible.
- Record evidence for every setup, configure, smoke, and deploy action.

### Milestone D: Development Factory

- Add subagent prompts for architect, implementer, integrator, tester, reviewer,
  operator, and release assistant.
- Wire Forgejo, Woodpecker, SonarQube, and Telegram notifications into the PR
  lifecycle.
- Add templates for new services, skills, APIs, apps, websites, and runbooks.
- Add local-first model routing with explicit external-token confirmation.

### Milestone E: Continuous Improvement

- Add feedback loops from failures into runbooks and service cards.
- Add dashboards for agent task volume, model routing, token-spend decisions,
  smoke health, and deployment outcomes.
- Periodically audit agent permissions, secrets access, and confirmation logs.

## Immediate Next Actions

1. Update every service README with the standard capability sections.
2. Add a lightweight docs checker that fails when a service lacks capability,
   verification, token-cost, or agent-permission sections.
3. Generate a first service-card index from existing manifests.
4. Add OpenClaw and Telegram prompts that can read the service-card index but
   only perform read-only actions.
5. Add confirmation language and risk labels for write-capable Telegram agent
   commands.
6. Pilot the workflow on one low-risk service, one SSO-backed service, and one
   development service before expanding to the entire stack.
