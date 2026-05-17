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
  [[ "$output" == *"homelab mcp --compose-dir /opt/homelab access setup"* ]]
  [[ "$output" == *"homelab-mcp access request"* ]]
  [[ "$output" == *"--name ci\\ smoke"* ]]
  [[ "$output" == *"--endpoint https://mcp.caboose-ai.io"* ]]
  [[ "$output" == *"homelab mcp --compose-dir /opt/homelab access approve"* ]]
  [[ "$output" == *"homelab-mcp access import"* ]]
  [[ "$output" == *"--credential-file $BATS_TEST_TMPDIR/path\\ with\\ space/credential.json"* ]]
  [[ "$output" == *"homelab-mcp access token"* ]]
  [[ "$output" == *"smoke_status=\$(curl -sS"* ]]
  [[ "$output" == *"-o /tmp/homelab-mcp-access.DRYRUN/smoke.body"* ]]
  [[ "$output" == *"test \"\$smoke_status\" = 200"* ]]
  [[ "$output" == *"grep -q"* ]]
  [[ "$output" == *"https://mcp.caboose-ai.io/"* ]]
}

@test "mcp access live cleans custom workdir artifacts and validates smoke status" {
  bin_dir="$BATS_TEST_TMPDIR/bin"
  mkdir -p "$bin_dir"

  cat >"$bin_dir/homelab" <<'EOF'
#!/usr/bin/env bash
printf 'homelab %s\n' "$*" >> "$CMD_LOG"
exit 0
EOF
  chmod +x "$bin_dir/homelab"

  cat >"$bin_dir/homelab-mcp" <<'EOF'
#!/usr/bin/env bash
printf 'homelab-mcp %s\n' "$*" >> "$CMD_LOG"
case "$*" in
  *"access token"*) echo "stub-token" ;;
esac
exit 0
EOF
  chmod +x "$bin_dir/homelab-mcp"

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

  export CMD_LOG="$BATS_TEST_TMPDIR/cmd.log"
  export PATH="$bin_dir:$PATH"
  run "$SCRIPT" \
    --skip-setup \
    --name "ci smoke" \
    --endpoint https://mcp.caboose-ai.io \
    --credential-file "$BATS_TEST_TMPDIR/credential.json" \
    --homelab-bin "$bin_dir/homelab" \
    --homelab-mcp-bin "$bin_dir/homelab-mcp" \
    --workdir "$BATS_TEST_TMPDIR/new-work"

  [ "$status" -eq 0 ]
  [ -d "$BATS_TEST_TMPDIR/new-work" ]
  [ ! -e "$BATS_TEST_TMPDIR/new-work/pending" ]
  [ ! -e "$BATS_TEST_TMPDIR/new-work/request.json" ]
  [ ! -e "$BATS_TEST_TMPDIR/new-work/release.json" ]
  [[ "$output" == *"initialize=200 server=homelab"* ]]
  [[ "$(cat "$CMD_LOG")" == *"access request"* ]]
  [[ "$(cat "$CMD_LOG")" == *"access approve"* ]]
  [[ "$(cat "$CMD_LOG")" == *"access import"* ]]
  [[ "$(cat "$CMD_LOG")" == *"access token"* ]]
}

@test "mcp access live print-token keeps stdout to export only" {
  bin_dir="$BATS_TEST_TMPDIR/bin"
  mkdir -p "$bin_dir"

  cat >"$bin_dir/homelab" <<'EOF'
#!/usr/bin/env bash
echo "progress from homelab"
exit 0
EOF
  chmod +x "$bin_dir/homelab"

  cat >"$bin_dir/homelab-mcp" <<'EOF'
#!/usr/bin/env bash
case "$*" in
  *"access token"*) echo "stub-token" ;;
  *) echo "progress from homelab-mcp" ;;
esac
exit 0
EOF
  chmod +x "$bin_dir/homelab-mcp"

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
  export PATH="$bin_dir:$PATH"
  "$SCRIPT" \
    --skip-setup \
    --print-token \
    --name "ci smoke" \
    --endpoint https://mcp.caboose-ai.io \
    --credential-file "$BATS_TEST_TMPDIR/credential.json" \
    --homelab-bin "$bin_dir/homelab" \
    --homelab-mcp-bin "$bin_dir/homelab-mcp" \
    --workdir "$BATS_TEST_TMPDIR/print-token-work" \
    >"$stdout" 2>"$stderr"

  [ "$(cat "$stdout")" = "export HOMELAB_MCP_TOKEN=stub-token" ]
  [[ "$(cat "$stderr")" == *"progress from homelab"* ]]
  [[ "$(cat "$stderr")" == *"initialize=200 server=homelab"* ]]
}
