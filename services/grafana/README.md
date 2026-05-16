# Grafana

Grafana provides dashboards for Prometheus and Loki data. Its configurator
manages Authentik OAuth client settings and Grafana OAuth environment secrets.

Grafana runs on host networking and binds to `127.0.0.1:3001` by default. This
keeps the server-side Prometheus datasource on `http://127.0.0.1:9090`, which is
required because Prometheus runs on host networking to scrape host-loopback
metrics targets. Loki publishes `127.0.0.1:3101` for the same reason; `3100` is
reserved for the host-network Paperclip API.
