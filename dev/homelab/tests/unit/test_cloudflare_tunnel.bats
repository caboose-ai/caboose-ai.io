#!/usr/bin/env bats

bats_require_minimum_version 1.5.0

setup() {
  REPO_ROOT="$(cd "$(dirname "$BATS_TEST_FILENAME")/../../../.." && pwd)"
  SCRIPT="$REPO_ROOT/dev/homelab/setup-cloudflare-tunnel.sh"
}

@test "default config routes public hostnames through local Caddy and ends with 404" {
  run env \
    HOMELAB_TUNNEL_ID=test-tunnel \
    HOMELAB_TUNNEL_CREDENTIALS_FILE=/tmp/test-tunnel.json \
    "$SCRIPT" --print

  [ "$status" -eq 0 ]
  [[ "$output" == *"tunnel: test-tunnel"* ]]
  [[ "$output" == *"credentials-file: /tmp/test-tunnel.json"* ]]
  [[ "$output" == *"service: https://127.0.0.1:443"* ]]

  for host in \
    caboose-ai.io \
    auth.caboose-ai.io \
    dash.caboose-ai.io \
    git.caboose-ai.io \
    ci.caboose-ai.io \
    docker.caboose-ai.io \
    grafana.caboose-ai.io \
    ai.caboose-ai.io \
    sonar.caboose-ai.io \
    chat.caboose-ai.io \
    openclaw.caboose-ai.io \
    blog.caboose-ai.io \
    paperclip.caboose-ai.io \
    mcp.caboose-ai.io
  do
    [[ "$output" == *"hostname: $host"* ]]
    [[ "$output" == *"httpHostHeader: $host"* ]]
    [[ "$output" == *"originServerName: $host"* ]]
  done

  [ "$(printf '%s\n' "$output" | tail -n 1)" = "  - service: http_status:404" ]
}

@test "domain origin and host overrides are reflected in generated config" {
  run env \
    HOMELAB_DOMAIN=example.test \
    HOMELAB_TUNNEL_ID=example-tunnel \
    HOMELAB_TUNNEL_CREDENTIALS_FILE=/tmp/example-tunnel.json \
    HOMELAB_TUNNEL_ORIGIN=https://127.0.0.1:8443 \
    HOMELAB_TUNNEL_HOSTS="auth.example.test,git.example.test" \
    "$SCRIPT" --print

  [ "$status" -eq 0 ]
  [[ "$output" == *"hostname: auth.example.test"* ]]
  [[ "$output" == *"hostname: git.example.test"* ]]
  [[ "$output" != *"hostname: ci.example.test"* ]]
  [[ "$output" == *"service: https://127.0.0.1:8443"* ]]
}

@test "print ignores configured output path and stays stdout only" {
  config_path="$BATS_TEST_TMPDIR/should-not-exist.yml"

  run env \
    HOMELAB_TUNNEL_ID=test-tunnel \
    HOMELAB_TUNNEL_CREDENTIALS_FILE=/tmp/test-tunnel.json \
    HOMELAB_TUNNEL_CONFIG="$config_path" \
    "$SCRIPT" --print

  [ "$status" -eq 0 ]
  [[ "$output" == *"ingress:"* ]]
  [[ "$output" != *"Cloudflare Tunnel config written"* ]]
  [ ! -e "$config_path" ]
}

@test "validate writes config and calls cloudflared ingress validation" {
  bin_dir="$BATS_TEST_TMPDIR/bin"
  mkdir -p "$bin_dir"

  cat >"$bin_dir/cloudflared" <<'EOF'
#!/usr/bin/env bash
printf 'cloudflared %s\n' "$*" >> "$CALL_LOG"
exit 0
EOF
  chmod +x "$bin_dir/cloudflared"

  export CALL_LOG="$BATS_TEST_TMPDIR/calls.log"
  config_path="$BATS_TEST_TMPDIR/config.yml"
  credentials_file="$BATS_TEST_TMPDIR/test-tunnel.json"
  touch "$credentials_file"

  run env \
    PATH="$bin_dir:$PATH" \
    HOMELAB_TUNNEL_ID=test-tunnel \
    HOMELAB_TUNNEL_CREDENTIALS_FILE="$credentials_file" \
    "$SCRIPT" --config "$config_path" --validate

  [ "$status" -eq 0 ]
  [ -f "$config_path" ]
  run grep -F "cloudflared --config $config_path tunnel ingress validate" "$CALL_LOG"
  [ "$status" -eq 0 ]
  run grep -F "hostname: auth.caboose-ai.io" "$config_path"
  [ "$status" -eq 0 ]
}

@test "validate without config path uses a temporary config and calls cloudflared" {
  bin_dir="$BATS_TEST_TMPDIR/bin"
  mkdir -p "$bin_dir"

  cat >"$bin_dir/cloudflared" <<'EOF'
#!/usr/bin/env bash
printf 'cloudflared %s\n' "$*" >> "$CALL_LOG"
exit 0
EOF
  chmod +x "$bin_dir/cloudflared"

  export CALL_LOG="$BATS_TEST_TMPDIR/calls.log"
  credentials_file="$BATS_TEST_TMPDIR/test-tunnel.json"
  touch "$credentials_file"

  run env \
    PATH="$bin_dir:$PATH" \
    HOMELAB_TUNNEL_ID=test-tunnel \
    HOMELAB_TUNNEL_CREDENTIALS_FILE="$credentials_file" \
    "$SCRIPT" --validate

  [ "$status" -eq 0 ]
  run grep -F "cloudflared --config" "$CALL_LOG"
  [ "$status" -eq 0 ]
  run grep -F "tunnel ingress validate" "$CALL_LOG"
  [ "$status" -eq 0 ]
}

@test "validate requires real tunnel metadata instead of placeholders" {
  bin_dir="$BATS_TEST_TMPDIR/bin"
  mkdir -p "$bin_dir"
  ln -s /usr/bin/bash "$bin_dir/bash"

  cat >"$bin_dir/cloudflared" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
  chmod +x "$bin_dir/cloudflared"

  run env \
    PATH="$bin_dir" \
    "$SCRIPT" --validate

  [ "$status" -eq 1 ]
  [[ "$output" == *"HOMELAB_TUNNEL_ID is required for --validate"* ]]
}

@test "validate requires an existing credentials file" {
  bin_dir="$BATS_TEST_TMPDIR/bin"
  mkdir -p "$bin_dir"
  ln -s /usr/bin/bash "$bin_dir/bash"

  cat >"$bin_dir/cloudflared" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
  chmod +x "$bin_dir/cloudflared"

  run env \
    PATH="$bin_dir" \
    HOMELAB_TUNNEL_ID=test-tunnel \
    HOMELAB_TUNNEL_CREDENTIALS_FILE="$BATS_TEST_TMPDIR/missing.json" \
    "$SCRIPT" --validate

  [ "$status" -eq 1 ]
  [[ "$output" == *"HOMELAB_TUNNEL_CREDENTIALS_FILE does not exist"* ]]
}

@test "validate fails clearly when cloudflared is unavailable" {
  empty_path="$BATS_TEST_TMPDIR/empty"
  mkdir -p "$empty_path"
  ln -s /usr/bin/bash "$empty_path/bash"

  run -127 env \
    PATH="$empty_path" \
    HOMELAB_TUNNEL_ID=test-tunnel \
    HOMELAB_TUNNEL_CREDENTIALS_FILE=/tmp/test-tunnel.json \
    "$SCRIPT" --validate

  [[ "$output" == *"cloudflared is required for --validate"* ]]
}
