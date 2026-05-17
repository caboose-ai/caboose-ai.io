# Mattermost

Mattermost provides team chat. Its configurator patches local configuration needed for the homelab deployment.

The stack uses Mattermost Team Edition. Do not configure Prometheus to scrape
Mattermost on port `8067`: Team Edition starts the profiling listener there, but
does not expose the Prometheus `/metrics` endpoint.
