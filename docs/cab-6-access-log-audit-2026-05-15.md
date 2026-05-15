# CAB-6 Access Log Audit (May 15, 2026)

Issue: [CAB-6](/CAB/issues/CAB-6)
Parent: [CAB-3](/CAB/issues/CAB-3)

## Scope

- Trigger: committed root `.env` exposure referenced by [CAB-3](/CAB/issues/CAB-3).
- Exposure anchor commit: `37630f2e4ac0f3ff55d87a4380cd16adb5882278` (authored May 3, 2026).
- Services in scope: Authentik, Grafana, Forgejo, reverse-proxy/access layer.
- Required outputs:
  - Exposure-window audit summary
  - 24h post-rotation verification summary
  - Escalation notes for suspicious events

## Audit Windows

- Window A (exposure): 2026-05-03 00:00:00 UTC -> timestamp when all [CAB-3](/CAB/issues/CAB-3) rotations completed.
- Window B (post-rotation): rotation-complete timestamp -> +24h.

## Evidence Collection Commands (Host Runtime)

Run on the homelab host with Docker + compose access:

```bash
# Authentik
cd /opt/homelab
docker compose logs authentik-server --since "2026-05-03T00:00:00Z" > authentik-window.log

# Grafana
cd /opt/homelab
docker compose logs grafana --since "2026-05-03T00:00:00Z" > grafana-window.log

# Forgejo
cd /opt/homelab
docker compose logs forgejo --since "2026-05-03T00:00:00Z" > forgejo-window.log

# Reverse proxy (adapt service name if needed)
cd /opt/homelab
docker compose logs caddy --since "2026-05-03T00:00:00Z" > proxy-window.log
```

Heuristics to flag suspicious events:

- Sudden login/authentication failures followed by successful admin logins from a new source.
- Token/API usage from unknown IPs or user agents.
- Authentication success for service/admin users outside expected operator windows.
- Bursts of 401/403/500 around auth endpoints with subsequent privileged actions.

## Findings In This Heartbeat

No live service logs were reachable from this execution environment:

- `docker` CLI unavailable (`docker: command not found`).
- `/opt/homelab` path not present.
- No local log exports for Authentik, Grafana, Forgejo, or proxy services were found in this workspace.

Because those sources were unavailable, this heartbeat could not produce final suspicious-access findings for Window A/Window B.

## Pending Operator Inputs To Unblock

- Provide exported logs for the four scoped services across Window A and Window B, or
- Run the collection commands above on the host and attach artifacts to [CAB-6](/CAB/issues/CAB-6).

## Escalation Template

If suspicious activity is found, add to [CAB-6](/CAB/issues/CAB-6):

- Timestamp (UTC), service, actor/user, source IP/UA.
- Why it is suspicious (baseline deviation).
- Immediate containment action (disable token/session, rotate secret, restrict access).
- Follow-up issue link(s) for remediation and verification.
