# Portainer

Portainer manages Docker resources. Its configurator sets up OIDC through
Authentik, ensures the local Docker socket environment exists, and promotes the
configured homelab admin email to Portainer admin when that OAuth-created user
is present.

## Agent Control Role

Portainer provides Docker visibility and approved operational handoffs for
Paperclip-driven work. Agents may use Portainer context for inspection, but
container restarts, deploys, volume changes, deletes, and other Docker
mutations require explicit human approval before execution.

## Recover Environment Access

If Portainer is healthy but `homelab service portainer configure --force` cannot
authenticate as `admin`, the initialized Portainer database no longer matches
the stored `PORTAINER_ADMIN_PASSWORD`.

Run the guarded recovery task:

```bash
mise run portainer:recover-access
```

Without `--yes`, the task only diagnoses the drift and refuses to reset
Portainer. To repair the local instance, rerun the underlying script with
confirmation:

```bash
dev/homelab/portainer-recover-access.sh --yes
```

The repair path discovers the live Portainer `/data` mount, stops Portainer,
runs Portainer's official `portainer/helper-reset-password` container with the
stored secret, restarts Portainer, then reapplies the repo configurator so OAuth,
the email claim mapping, and the local Docker environment are restored.
