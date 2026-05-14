#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$(dirname "$BATS_TEST_FILENAME")/../../../.." && pwd)"
  SCRIPT="$REPO_ROOT/dev/homelab/mcp-test-live.sh"
}

@test "mcp test live dry-run only probes existing access" {
  run "$SCRIPT" \
    --dry-run \
    --endpoint https://mcp.caboose-ai.io \
    --credential-file "$BATS_TEST_TMPDIR/credential.json"

  [ "$status" -eq 0 ]
  [[ "$output" == *"unauth_status=\$(curl -sS"* ]]
  [[ "$output" == *"test \"\$unauth_status\" = 401"* ]]
  [[ "$output" == *"go run ./cmd/mcp access token --credential-file $BATS_TEST_TMPDIR/credential.json"* ]]
  [[ "$output" == *"auth_status=\$(curl -sS"* ]]
  [[ "$output" == *"test \"\$auth_status\" = 200"* ]]
  [[ "$output" == *"grep -q"* ]]
  [[ "$output" == *"initialize"* ]]
  [[ "$output" != *"access request"* ]]
  [[ "$output" != *"access approve"* ]]
  [[ "$output" != *"access import"* ]]
}
