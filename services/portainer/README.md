# Portainer

Portainer manages Docker resources. Its configurator sets up OIDC through Authentik.

## Agent Control Role

Portainer provides Docker visibility and approved operational handoffs for
Paperclip-driven work. Agents may use Portainer context for inspection, but
container restarts, deploys, volume changes, deletes, and other Docker
mutations require explicit human approval before execution.
