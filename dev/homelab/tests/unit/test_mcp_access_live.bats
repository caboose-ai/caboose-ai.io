#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$(dirname "$BATS_TEST_FILENAME")/../../../.." && pwd)"
  SCRIPT="$REPO_ROOT/dev/homelab/mcp-access-live.sh"
}

@test "mcp access live dry-run prints full command sequence" {
  credential_file="$BATS_TEST_TMPDIR/path with space/credential.json"

  run "$SCRIPT" \
    --dry-run \
    --name "ci smoke" \
    --compose-dir /opt/homelab \
    --endpoint https://mcp.caboose-ai.io \
    --credential-file "$credential_file"

  [ "$status" -eq 0 ]
  [[ "$output" == *"go run ./cmd/homelab mcp --compose-dir /opt/homelab access setup"* ]]
  [[ "$output" == *"go run ./cmd/mcp access request"* ]]
  [[ "$output" == *"--name ci\\ smoke"* ]]
  [[ "$output" == *"--endpoint https://mcp.caboose-ai.io"* ]]
  [[ "$output" == *"go run ./cmd/homelab mcp --compose-dir /opt/homelab access approve"* ]]
  [[ "$output" == *"go run ./cmd/mcp access import"* ]]
  [[ "$output" == *"--credential-file $BATS_TEST_TMPDIR/path\\ with\\ space/credential.json"* ]]
  [[ "$output" == *"go run ./cmd/mcp access token"* ]]
  [[ "$output" == *"curl -i --max-time 10"* ]]
  [[ "$output" == *"https://mcp.caboose-ai.io/"* ]]
}

@test "mcp access live cleans custom workdir artifacts and validates smoke status" {
  bin_dir="$BATS_TEST_TMPDIR/bin"
  mkdir -p "$bin_dir"

  cat >"$bin_dir/go" <<'EOF'
#!/usr/bin/env bash
printf 'go %s\n' "$*" >> "$GO_LOG"
case "$*" in
  *" access token"*) echo "stub-token" ;;
esac
exit 0
EOF
  chmod +x "$bin_dir/go"

  cat >"$bin_dir/curl" <<'EOF'
#!/usr/bin/env bash
body=""
headers=""
write_out=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    -o) body="$2"; shift 2 ;;
    -D) headers="$2"; shift 2 ;;
    -w) write_out="$2"; shift 2 ;;
    *) shift ;;
  esac
done
[[ -n "$body" ]] && printf '{"serverInfo":{"name":"homelab"}}\n' > "$body"
[[ -n "$headers" ]] && printf 'HTTP/2 200\r\n' > "$headers"
[[ "$write_out" == "%{http_code}" ]] && printf '200'
EOF
  chmod +x "$bin_dir/curl"

  GO_LOG="$BATS_TEST_TMPDIR/go.log" PATH="$bin_dir:$PATH" run "$SCRIPT" \
    --skip-setup \
    --name "ci smoke" \
    --endpoint https://mcp.caboose-ai.io \
    --credential-file "$BATS_TEST_TMPDIR/credential.json" \
    --workdir "$BATS_TEST_TMPDIR/new-work"

  [ "$status" -eq 0 ]
  [ -d "$BATS_TEST_TMPDIR/new-work" ]
  [ ! -e "$BATS_TEST_TMPDIR/new-work/pending" ]
  [ ! -e "$BATS_TEST_TMPDIR/new-work/request.json" ]
  [ ! -e "$BATS_TEST_TMPDIR/new-work/release.json" ]
  [[ "$output" == *"initialize=200 server=homelab"* ]]
  [[ "$(cat "$BATS_TEST_TMPDIR/go.log")" == *"access request"* ]]
  [[ "$(cat "$BATS_TEST_TMPDIR/go.log")" == *"access approve"* ]]
  [[ "$(cat "$BATS_TEST_TMPDIR/go.log")" == *"access import"* ]]
  [[ "$(cat "$BATS_TEST_TMPDIR/go.log")" == *"access token"* ]]
}

@test "mcp access live print-token keeps stdout to export only" {
  bin_dir="$BATS_TEST_TMPDIR/bin"
  mkdir -p "$bin_dir"

  cat >"$bin_dir/go" <<'EOF'
#!/usr/bin/env bash
case "$*" in
  *" access token"*) echo "stub-token" ;;
  *) echo "progress from go" ;;
esac
exit 0
EOF
  chmod +x "$bin_dir/go"

  cat >"$bin_dir/curl" <<'EOF'
#!/usr/bin/env bash
body=""
headers=""
write_out=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    -o) body="$2"; shift 2 ;;
    -D) headers="$2"; shift 2 ;;
    -w) write_out="$2"; shift 2 ;;
    *) shift ;;
  esac
done
[[ -n "$body" ]] && printf '{"serverInfo":{"name":"homelab"}}\n' > "$body"
[[ -n "$headers" ]] && printf 'HTTP/2 200\r\n' > "$headers"
[[ "$write_out" == "%{http_code}" ]] && printf '200'
EOF
  chmod +x "$bin_dir/curl"

  stdout="$BATS_TEST_TMPDIR/stdout"
  stderr="$BATS_TEST_TMPDIR/stderr"
  PATH="$bin_dir:$PATH" "$SCRIPT" \
    --skip-setup \
    --print-token \
    --name "ci smoke" \
    --endpoint https://mcp.caboose-ai.io \
    --credential-file "$BATS_TEST_TMPDIR/credential.json" \
    --workdir "$BATS_TEST_TMPDIR/print-token-work" \
    >"$stdout" 2>"$stderr"

  [ "$(cat "$stdout")" = "export HOMELAB_MCP_TOKEN=stub-token" ]
  [[ "$(cat "$stderr")" == *"progress from go"* ]]
  [[ "$(cat "$stderr")" == *"initialize=200 server=homelab"* ]]
}
