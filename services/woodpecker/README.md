# Woodpecker

Woodpecker runs CI jobs for Forgejo repositories. Its configurator wires the Forgejo OAuth application and Woodpecker secrets.

## Agent Control Role

Woodpecker is the self-hosted CI verification surface for Paperclip-driven
development work. Agents should attach Woodpecker check results to the
Paperclip task before asking for final human review or deploy approval.
