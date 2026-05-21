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

@test "mise separates local and external mcp readiness tasks" {
  run grep -F '[tasks."mcp:probe-local"]' "$MISE_FILE"
  [ "$status" -eq 0 ]
  run grep -F 'http://$HOMELAB_MCP_HTTP_ADDR/' "$MISE_FILE"
  [ "$status" -eq 0 ]
  run grep -F '[tasks."mcp:external-readiness"]' "$MISE_FILE"
  [ "$status" -eq 0 ]
  run grep -F 'dev/homelab/mcp-external-readiness.sh' "$MISE_FILE"
  [ "$status" -eq 0 ]
  run grep -F 'Legacy/static-IP path' "$MISE_FILE"
  [ "$status" -eq 0 ]
}

@test "mise exposes cloudflare tunnel config task as the public default path" {
  run grep -F '[tasks."tunnel:config"]' "$MISE_FILE"
  [ "$status" -eq 0 ]
  run grep -F '[tasks."tunnel:print"]' "$MISE_FILE"
  [ "$status" -eq 0 ]
  run grep -F 'dev/homelab/setup-cloudflare-tunnel.sh' "$MISE_FILE"
  [ "$status" -eq 0 ]
  run grep -F 'HOMELAB_TUNNEL_CONFIG:-${XDG_CONFIG_HOME:-$HOME/.config}/cloudflared/homelab.yml' "$MISE_FILE"
  [ "$status" -eq 0 ]
}

@test "mise exposes local MVP release gate and per-service logs task" {
  run grep -F '[tasks."release:mvp-local"]' "$MISE_FILE"
  [ "$status" -eq 0 ]
  run grep -F 'dev/homelab/mvp-local-readiness.sh' "$MISE_FILE"
  [ "$status" -eq 0 ]
  run grep -F 'HOMELAB_MVP_COMPOSE_DIR:-dev/homelab' "$MISE_FILE"
  [ "$status" -eq 0 ]

  run grep -F '[tasks."service:logs"]' "$MISE_FILE"
  [ "$status" -eq 0 ]
  run grep -F 'go run ./cmd/homelab service --domain \"$HOMELAB_DOMAIN\" --compose-dir \"$HOMELAB_COMPOSE_DIR\" logs' "$MISE_FILE"
  [ "$status" -eq 0 ]
}

@test "mise exposes installed Homebrew binary compose-dir guard" {
  run grep -F '[tasks."homelab:installed-binary-check"]' "$MISE_FILE"
  [ "$status" -eq 0 ]
  run grep -F 'dev/homelab/homebrew-stack-readiness.sh' "$MISE_FILE"
  [ "$status" -eq 0 ]
  run grep -F '${HOMELAB_BIN:-homelab}' "$MISE_FILE"
  [ "$status" -eq 0 ]
}
