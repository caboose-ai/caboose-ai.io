#!/usr/bin/env bats

bats_require_minimum_version 1.5.0

setup() {
  REPO_ROOT="$(cd "$(dirname "$BATS_TEST_FILENAME")/../../../.." && pwd)"
  SCRIPT="$REPO_ROOT/dev/homelab/mcp-external-readiness.sh"
}

@test "external mcp readiness dry-run validates tunnel then access lifecycle and probe" {
  run "$SCRIPT" \
    --dry-run \
    --name "ci client" \
    --compose-dir /opt/homelab \
    --endpoint https://mcp.caboose-ai.io \
    --credential-file "$BATS_TEST_TMPDIR/credential.json"

  [ "$status" -eq 0 ]
  [[ "$output" == *"+ dev/homelab/setup-cloudflare-tunnel.sh --print"* ]]
  [[ "$output" == *"+ dev/homelab/setup-cloudflare-tunnel.sh --config "* ]]
  [[ "$output" == *" --validate"* ]]
  [[ "$output" == *"+ dev/homelab/mcp-access-live.sh --name ci\\ client --endpoint https://mcp.caboose-ai.io --compose-dir /opt/homelab --credential-file $BATS_TEST_TMPDIR/credential.json --dry-run"* ]]
  [[ "$output" == *"+ dev/homelab/mcp-test-live.sh --endpoint https://mcp.caboose-ai.io --credential-file $BATS_TEST_TMPDIR/credential.json --dry-run"* ]]
  [[ "$output" != *"HOMELAB_PUBLIC_IP"* ]]
  [[ "$output" != *"setup-mcp-external.sh"* ]]
}

@test "external mcp readiness uses configured binaries and tunnel config" {
  run env \
    HOMELAB_BIN=/tmp/homelab \
    HOMELAB_MCP_BIN=/tmp/homelab-mcp \
    HOMELAB_TUNNEL_CONFIG=/tmp/homelab.yml \
    "$SCRIPT" \
      --dry-run \
      --name "laptop" \
      --compose-dir /srv/homelab \
      --endpoint https://mcp.example.test

  [ "$status" -eq 0 ]
  [[ "$output" == *"+ dev/homelab/setup-cloudflare-tunnel.sh --config /tmp/homelab.yml --validate"* ]]
  [[ "$output" == *"--homelab-bin /tmp/homelab"* ]]
  [[ "$output" == *"--homelab-mcp-bin /tmp/homelab-mcp"* ]]
  [[ "$output" == *"--endpoint https://mcp.example.test"* ]]
  [[ "$output" == *"--compose-dir /srv/homelab"* ]]
}
