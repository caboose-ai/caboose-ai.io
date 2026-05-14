#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
compose_dir="${HOMELAB_COMPOSE_DIR:-/opt/homelab}"
endpoint="${HOMELAB_MCP_EXTERNAL_URL:-https://mcp.caboose-ai.io}"
name="${HOMELAB_MCP_ACCESS_NAME:-$(hostname) live client $(date -u +%Y%m%dT%H%M%SZ)}"
credential_file="${HOMELAB_MCP_CREDENTIAL_FILE:-}"
workdir="${HOMELAB_MCP_ACCESS_WORKDIR:-}"
dry_run=false
skip_setup=false
keep_artifacts=false
print_token=false

usage() {
  cat <<'EOF'
Usage: dev/homelab/mcp-access-live.sh [flags]

Create, approve, import, and smoke-test a live Homelab MCP access credential.

Flags:
  --name NAME              Client name for the Authentik service account request
  --endpoint URL           MCP endpoint (default: https://mcp.caboose-ai.io)
  --compose-dir DIR        Live compose directory (default: /opt/homelab)
  --credential-file PATH   Import credential to PATH instead of the default user config
  --workdir DIR            Use DIR for temporary request/release files
  --skip-setup             Skip homelab mcp access setup
  --keep-artifacts         Keep request/release/pending files
  --print-token            Print an export command for the minted bearer token
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
    --workdir)
      workdir="${2:?--workdir requires a value}"
      shift 2
      ;;
    --skip-setup)
      skip_setup=true
      shift
      ;;
    --keep-artifacts)
      keep_artifacts=true
      shift
      ;;
    --print-token)
      print_token=true
      shift
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

if [[ -z "$workdir" ]]; then
  if [[ "$dry_run" == "true" ]]; then
    workdir="/tmp/homelab-mcp-access.DRYRUN"
  else
    workdir="$(mktemp -d /tmp/homelab-mcp-access.XXXXXX)"
  fi
fi

pending_dir="$workdir/pending"
request_file="$workdir/request.json"
release_file="$workdir/release.json"
endpoint_url="${endpoint%/}/"
initialize_payload='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"mcp-access-live","version":"dev"}}}'

cleanup() {
  if [[ "$dry_run" == "true" || "$keep_artifacts" == "true" ]]; then
    return
  fi
  if [[ "$workdir" == /tmp/homelab-mcp-access.* ]]; then
    rm -rf "$workdir"
  fi
}
trap cleanup EXIT

print_cmd() {
  printf '+'
  printf ' %s' "$@"
  printf '\n'
}

run_cmd() {
  if [[ "$dry_run" == "true" ]]; then
    print_cmd "$@"
    return 0
  fi
  "$@"
}

credential_args=()
if [[ -n "$credential_file" ]]; then
  credential_args=(--credential-file "$credential_file")
fi

if [[ "$dry_run" != "true" ]]; then
  mkdir -p "$workdir" "$pending_dir"
fi

cd "$repo_root"

if [[ "$skip_setup" != "true" ]]; then
  run_cmd go run ./cmd/homelab mcp --compose-dir "$compose_dir" access setup
fi

run_cmd go run ./cmd/mcp access request \
  --name "$name" \
  --endpoint "$endpoint" \
  --pending-dir "$pending_dir" \
  --out "$request_file"

run_cmd go run ./cmd/homelab mcp --compose-dir "$compose_dir" access approve \
  "$request_file" \
  --out "$release_file"

run_cmd go run ./cmd/mcp access import \
  --pending-dir "$pending_dir" \
  "${credential_args[@]}" \
  "$release_file"

token_cmd=(go run ./cmd/mcp access token "${credential_args[@]}")
if [[ "$dry_run" == "true" ]]; then
  print_cmd "${token_cmd[@]}"
  print_cmd curl -i --max-time 10 \
    -H "Authorization: Bearer \$TOKEN" \
    -H "Content-Type: application/json" \
    -H "Accept: application/json, text/event-stream" \
    --data "$initialize_payload" \
    "$endpoint_url"
  exit 0
fi

token="$("${token_cmd[@]}")"

if [[ "$print_token" == "true" ]]; then
  printf 'export HOMELAB_MCP_TOKEN=%q\n' "$token"
fi

smoke_headers="$workdir/smoke.headers"
smoke_body="$workdir/smoke.body"
smoke_status="$(
  curl -sS \
    -o "$smoke_body" \
    -D "$smoke_headers" \
    -w '%{http_code}' \
    --max-time 10 \
    -H "Authorization: Bearer $token" \
    -H "Content-Type: application/json" \
    -H "Accept: application/json, text/event-stream" \
    --data "$initialize_payload" \
    "$endpoint_url"
)"

if [[ "$smoke_status" != "200" ]]; then
  echo "authenticated initialize returned HTTP $smoke_status, want 200" >&2
  cat "$smoke_body" >&2
  exit 1
fi
if ! grep -q '"name":"homelab"' "$smoke_body"; then
  echo "authenticated initialize response did not identify the homelab server" >&2
  cat "$smoke_body" >&2
  exit 1
fi

echo "initialize=200 server=homelab"
