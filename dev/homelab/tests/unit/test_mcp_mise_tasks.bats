#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$(dirname "$BATS_TEST_FILENAME")/../../../.." && pwd)"
  MISE_FILE="$REPO_ROOT/mise.toml"
}

@test "mise exposes mcp live access and smoke-test tasks" {
  run grep -F '[tasks."mcp:access-live"]' "$MISE_FILE"
  [ "$status" -eq 0 ]
  run grep -F 'dev/homelab/mcp-access-live.sh' "$MISE_FILE"
  [ "$status" -eq 0 ]

  run grep -F '[tasks."mcp:test-live"]' "$MISE_FILE"
  [ "$status" -eq 0 ]
  run grep -F 'dev/homelab/mcp-test-live.sh' "$MISE_FILE"
  [ "$status" -eq 0 ]
}
