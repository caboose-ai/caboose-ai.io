#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
compose_dir="${HOMELAB_COMPOSE_DIR:-/opt/homelab}"
endpoint="${HOMELAB_MCP_EXTERNAL_URL:-https://mcp.caboose-ai.io}"
name="${HOMELAB_MCP_ACCESS_NAME:-$(hostname) external mcp $(date -u +%Y%m%dT%H%M%SZ)}"
credential_file="${HOMELAB_MCP_CREDENTIAL_FILE:-}"
tunnel_config="${HOMELAB_TUNNEL_CONFIG:-${XDG_CONFIG_HOME:-$HOME/.config}/cloudflared/homelab.yml}"
dry_run=false

usage() {
  cat <<'EOF'
Usage: dev/homelab/mcp-external-readiness.sh [flags]

Validate the supported Cloudflare Tunnel external MCP profile.

The gate validates the Cloudflare Tunnel ingress config, then runs the external
MCP access lifecycle and existing-access endpoint probe. It does not configure
legacy static-IP DNS or Caddy routes.

Flags:
  --name NAME              Client name for the access request
  --endpoint URL           MCP endpoint (default: https://mcp.caboose-ai.io)
  --compose-dir DIR        Live compose directory (default: /opt/homelab)
  --credential-file PATH   Import/use MCP credential at PATH
  --tunnel-config PATH     Cloudflare Tunnel config output path
  --dry-run                Print commands without running them
  -h, --help               Show this help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --name)
      name="${2:?--name requires a value}"
      shift 2
      ;;
    --endpoint)
      endpoint="${2:?--endpoint requires a value}"
      shift 2
      ;;
    --compose-dir)
      compose_dir="${2:?--compose-dir requires a value}"
      shift 2
      ;;
    --credential-file)
      credential_file="${2:?--credential-file requires a value}"
      shift 2
      ;;
    --tunnel-config)
      tunnel_config="${2:?--tunnel-config requires a value}"
      shift 2
      ;;
    --dry-run)
      dry_run=true
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown flag: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

print_cmd() {
  printf '+'
  printf ' %q' "$@"
  printf '\n'
}

run_cmd() {
  print_cmd "$@"
  if [[ "$dry_run" == "true" ]]; then
    return 0
  fi
  "$@"
}

cd "$repo_root"

access_args=(
  --name "$name"
  --endpoint "$endpoint"
  --compose-dir "$compose_dir"
)
test_args=(
  --endpoint "$endpoint"
)

if [[ -n "$credential_file" ]]; then
  access_args+=(--credential-file "$credential_file")
  test_args+=(--credential-file "$credential_file")
fi
if [[ -n "${HOMELAB_BIN:-}" ]]; then
  access_args+=(--homelab-bin "$HOMELAB_BIN")
fi
if [[ -n "${HOMELAB_MCP_BIN:-}" ]]; then
  access_args+=(--homelab-mcp-bin "$HOMELAB_MCP_BIN")
  test_args+=(--homelab-mcp-bin "$HOMELAB_MCP_BIN")
fi
if [[ "$dry_run" == "true" ]]; then
  access_args+=(--dry-run)
  test_args+=(--dry-run)
fi

run_cmd dev/homelab/setup-cloudflare-tunnel.sh --print
run_cmd dev/homelab/setup-cloudflare-tunnel.sh \
  --config "$tunnel_config" \
  --validate
run_cmd dev/homelab/mcp-access-live.sh "${access_args[@]}"
run_cmd dev/homelab/mcp-test-live.sh "${test_args[@]}"
