#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
endpoint="${HOMELAB_MCP_EXTERNAL_URL:-https://mcp.caboose-ai.io}"
credential_file="${HOMELAB_MCP_CREDENTIAL_FILE:-}"
homelab_mcp_bin="${HOMELAB_MCP_BIN:-homelab-mcp}"
timeout="${HOMELAB_MCP_TEST_TIMEOUT:-10}"
dry_run=false
verbose=false

usage() {
  cat <<'EOF'
Usage: dev/homelab/mcp-test-live.sh [flags]

Smoke-test the live Homelab MCP endpoint using existing access.

The script uses HOMELAB_MCP_TOKEN when set. Otherwise it runs
`homelab-mcp access token`, optionally with --credential-file.

Flags:
  --endpoint URL           MCP endpoint (default: https://mcp.caboose-ai.io)
  --credential-file PATH   Use a specific imported MCP credential file
  --homelab-mcp-bin PATH   homelab-mcp binary to use (default: homelab-mcp)
  --timeout SECONDS        Per-request curl timeout (default: 10)
  --dry-run                Print commands without running them
  --verbose                Print response bodies on failure
  -h, --help               Show this help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --endpoint)
      endpoint="${2:?--endpoint requires a value}"
      shift 2
      ;;
    --credential-file)
      credential_file="${2:?--credential-file requires a value}"
      shift 2
      ;;
    --homelab-mcp-bin)
      homelab_mcp_bin="${2:?--homelab-mcp-bin requires a value}"
      shift 2
      ;;
    --timeout)
      timeout="${2:?--timeout requires a value}"
      shift 2
      ;;
    --dry-run)
      dry_run=true
      shift
      ;;
    --verbose)
      verbose=true
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

endpoint_url="${endpoint%/}/"
initialize_payload='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"mcp-test-live","version":"dev"}}}'

credential_args=()
if [[ -n "$credential_file" ]]; then
  credential_args=(--credential-file "$credential_file")
fi
token_cmd=("$homelab_mcp_bin" access token "${credential_args[@]}")

print_cmd() {
  printf '+'
  printf ' %q' "$@"
  printf '\n'
}

print_capture_cmd() {
  local var_name="$1"
  shift
  printf '+ %s=$(' "$var_name"
  printf '%q' "$1"
  shift
  printf ' %q' "$@"
  printf ')\n'
}

print_shell_line() {
  printf '+ %s\n' "$*"
}

fail_response() {
  local message="$1" body="${2:-}"
  echo "$message" >&2
  if [[ "$verbose" == "true" && -n "$body" && -f "$body" ]]; then
    echo "--- response body ---" >&2
    cat "$body" >&2
  fi
  exit 1
}

cd "$repo_root"

if [[ "$dry_run" == "true" ]]; then
  print_capture_cmd unauth_status curl -sS \
    -o '<tmpdir>/unauth.body' \
    -D '<tmpdir>/unauth.headers' \
    -w '%{http_code}' \
    --max-time "$timeout" \
    "$endpoint_url"
  print_shell_line 'test "$unauth_status" = 401'
  print_cmd grep -qi '^www-authenticate:.*Bearer' '<tmpdir>/unauth.headers'
  if [[ -n "${HOMELAB_MCP_TOKEN:-}" ]]; then
    echo '+ TOKEN=<HOMELAB_MCP_TOKEN>'
  else
    print_cmd "${token_cmd[@]}"
  fi
  print_capture_cmd auth_status curl -sS \
    -o '<tmpdir>/auth.body' \
    -D '<tmpdir>/auth.headers' \
    -w '%{http_code}' \
    --max-time "$timeout" \
    -H "Authorization: Bearer \$TOKEN" \
    -H "Content-Type: application/json" \
    -H "Accept: application/json, text/event-stream" \
    --data "$initialize_payload" \
    "$endpoint_url"
  print_shell_line 'test "$auth_status" = 200'
  print_cmd grep -q '"name":"homelab"' '<tmpdir>/auth.body'
  exit 0
fi

tmpdir="$(mktemp -d /tmp/homelab-mcp-test.XXXXXX)"
trap 'rm -rf "$tmpdir"' EXIT

unauth_headers="$tmpdir/unauth.headers"
unauth_body="$tmpdir/unauth.body"
unauth_status="$(
  curl -sS \
    -o "$unauth_body" \
    -D "$unauth_headers" \
    -w '%{http_code}' \
    --max-time "$timeout" \
    "$endpoint_url"
)"

if [[ "$unauth_status" != "401" ]]; then
  fail_response "unauthenticated probe returned HTTP $unauth_status, want 401" "$unauth_body"
fi
if ! grep -qi '^www-authenticate:.*Bearer' "$unauth_headers"; then
  fail_response "unauthenticated probe did not return a Bearer challenge" "$unauth_body"
fi
echo "unauth=401 bearer challenge ok"

if [[ -n "${HOMELAB_MCP_TOKEN:-}" ]]; then
  token="$HOMELAB_MCP_TOKEN"
  echo "token=from HOMELAB_MCP_TOKEN"
else
  if ! token="$("${token_cmd[@]}" 2>"$tmpdir/token.err")"; then
    cat "$tmpdir/token.err" >&2
    echo "Could not mint a token. Import MCP access first, pass --credential-file, or set HOMELAB_MCP_TOKEN." >&2
    exit 1
  fi
  if [[ -z "$token" ]]; then
    echo "token command returned an empty token" >&2
    exit 1
  fi
  echo "token=minted from credential"
fi

auth_headers="$tmpdir/auth.headers"
auth_body="$tmpdir/auth.body"
auth_status="$(
  curl -sS \
    -o "$auth_body" \
    -D "$auth_headers" \
    -w '%{http_code}' \
    --max-time "$timeout" \
    -H "Authorization: Bearer $token" \
    -H "Content-Type: application/json" \
    -H "Accept: application/json, text/event-stream" \
    --data "$initialize_payload" \
    "$endpoint_url"
)"

if [[ "$auth_status" != "200" ]]; then
  fail_response "authenticated initialize returned HTTP $auth_status, want 200" "$auth_body"
fi
if ! grep -q '"name":"homelab"' "$auth_body"; then
  fail_response "authenticated initialize response did not identify the homelab server" "$auth_body"
fi

session_id="$(awk -F': ' 'tolower($1) == "mcp-session-id" {gsub(/\r/, "", $2); print $2; exit}' "$auth_headers")"
if [[ -n "$session_id" ]]; then
  echo "initialize=200 server=homelab session=$session_id"
else
  echo "initialize=200 server=homelab"
fi
