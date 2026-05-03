# Homelab Orchestrators

Docker Compose remains the default backend. Existing users do not need to change anything.

## Compose mode (default)

```yaml
# homelab.yml
orchestrator: compose
compose_dir: dev/homelab
domain: example.com
```

## Kubernetes mode (k3s-compatible)

```yaml
# homelab.yml
orchestrator: kubernetes
compose_dir: dev/homelab
domain: example.com
kubernetes:
  namespace: homelab
  kubeconfig: /etc/rancher/k3s/k3s.yaml
  context: default
```

Run with flags:

```bash
homelab install --non-interactive --orchestrator kubernetes --kube-namespace homelab --domain example.com
```

## Single-node k3s quickstart (static IP)

1. Install k3s on the node and ensure the host has a stable/static IP and DNS records for your homelab domain.
2. Verify kubectl access: `kubectl get nodes`.
3. Run homelab install with `--orchestrator kubernetes`.
4. Optional: use k9s for operator visibility; k9s is not required by runtime.

## Migration note

Existing Docker Compose users can keep their current workflow unchanged. Switch to Kubernetes only when you explicitly set `orchestrator: kubernetes` or pass `--orchestrator kubernetes`.
