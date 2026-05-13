#!/usr/bin/env bash
set -euo pipefail

host="${HOMELAB_MCP_EXTERNAL_HOST:-mcp.caboose-ai.io}"
target="${HOMELAB_MCP_TARGET:-localhost:8090}"
public_ip="${HOMELAB_PUBLIC_IP:-104.153.31.50}"
caddyfile="${CADDYFILE:-/etc/caddy/Caddyfile}"
api_base="${CLOUDFLARE_API_BASE:-https://api.cloudflare.com/client/v4}"

need() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required command: $1" >&2
    exit 127
  fi
}

secret() {
  local name="$1"
  local value="${!name:-}"
  if [[ -n "$value" ]]; then
    printf '%s' "$value"
    return
  fi
  if command -v fnox >/dev/null 2>&1; then
    if value="$(fnox get "$name" 2>/dev/null)"; then
      printf '%s' "$value"
      return
    fi
  fi
  echo "$name is required; export it or sign in to 1Password for fnox." >&2
  return 1
}

upsert_dns_record() {
  local token zone_id existing record_id payload response
  token="$(secret CLOUDFLARE_API_TOKEN)"
  zone_id="$(secret CLOUDFLARE_ZONE_ID)"

  existing="$(
    curl -fsS \
      -H "Authorization: Bearer $token" \
      -H "Content-Type: application/json" \
      "$api_base/zones/$zone_id/dns_records?type=A&name=$host"
  )"

  record_id="$(jq -r '.result[0].id // empty' <<<"$existing")"
  payload="$(
    jq -n \
      --arg type "A" \
      --arg name "$host" \
      --arg content "$public_ip" \
      '{type:$type,name:$name,content:$content,ttl:1,proxied:false}'
  )"

  if [[ -n "$record_id" ]]; then
    response="$(
      curl -fsS -X PUT \
        -H "Authorization: Bearer $token" \
        -H "Content-Type: application/json" \
        --data "$payload" \
        "$api_base/zones/$zone_id/dns_records/$record_id"
    )"
  else
    response="$(
      curl -fsS -X POST \
        -H "Authorization: Bearer $token" \
        -H "Content-Type: application/json" \
        --data "$payload" \
        "$api_base/zones/$zone_id/dns_records"
    )"
  fi

  jq -e '.success == true' >/dev/null <<<"$response"
  echo "Cloudflare A record is set: $host -> $public_ip"
}

ensure_caddy_route() {
  if grep -Eq "^${host//./\\.}[[:space:]]*\\{" "$caddyfile"; then
    echo "Caddy route already exists for $host"
    return
  fi

  local tmp
  tmp="$(mktemp)"
  cp "$caddyfile" "$tmp"
  {
    echo
    echo "# Homelab MCP - bearer-token MCP HTTP endpoint"
    echo "$host {"
    echo "    import secure_headers"
    echo "    reverse_proxy $target"
    echo "}"
  } >>"$tmp"

  sudo caddy validate --config "$tmp" --adapter caddyfile
  sudo install -m 0644 -o nobody -g nogroup "$tmp" "$caddyfile"
  sudo systemctl reload caddy
  rm -f "$tmp"
  echo "Caddy route is installed: $host -> $target"
}

verify_external() {
  local resolved
  resolved="$(dig +short "$host" | sort -u | tr '\n' ' ')"
  if [[ -z "$resolved" ]]; then
    echo "DNS did not resolve for $host" >&2
    exit 1
  fi
  echo "DNS resolves: $host -> $resolved"
  for attempt in {1..12}; do
    if curl -i --max-time 10 "https://$host/"; then
      return
    fi
    echo "HTTPS probe failed; retrying after certificate setup ($attempt/12)" >&2
    sleep 5
  done
  echo "HTTPS probe did not succeed for https://$host/" >&2
  exit 1
}

need curl
need dig
need jq
need sudo

upsert_dns_record
ensure_caddy_route
verify_external
