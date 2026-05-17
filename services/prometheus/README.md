# Prometheus

Prometheus collects metrics for the homelab stack. It is tracked for operational status and documentation.

The default scrape set includes host, container, Authentik, and other endpoints
that expose Prometheus metrics. Mattermost Team Edition is intentionally omitted
because its profiling listener does not provide HTTP `/metrics`.
