#!/usr/bin/env bash
set -euo pipefail

domain="${HOMELAB_DOMAIN:-caboose-ai.io}"
origin="${HOMELAB_TUNNEL_ORIGIN:-https://127.0.0.1:443}"
tunnel_id="${HOMELAB_TUNNEL_ID:-<tunnel-id>}"
credentials_file="${HOMELAB_TUNNEL_CREDENTIALS_FILE:-/etc/cloudflared/${tunnel_id}.json}"
hosts_input="${HOMELAB_TUNNEL_HOSTS:-}"
config_path=""
print_config=false
validate=false
cleanup_path=""

usage() {
  cat <<'USAGE'
Usage: setup-cloudflare-tunnel.sh [--print] [--config PATH] [--validate]

Generate a locally managed Cloudflare Tunnel ingress config for the homelab
public profile. The tunnel forwards browser-facing hostnames to local Caddy,
so no static public IP or inbound WAN port forwarding is required.

Environment:
  HOMELAB_DOMAIN                   Base domain. Default: caboose-ai.io
  HOMELAB_TUNNEL_ID                Cloudflare tunnel UUID or name.
  HOMELAB_TUNNEL_CREDENTIALS_FILE  cloudflared credentials JSON path.
  HOMELAB_TUNNEL_ORIGIN            Local Caddy origin. Default: https://127.0.0.1:443
  HOMELAB_TUNNEL_HOSTS             Comma/space separated host override.
  HOMELAB_TUNNEL_CONFIG            Default output path for --config.
USAGE
}

die() {
  echo "$*" >&2
  exit 1
}

default_hosts() {
  printf '%s\n' \
    "$domain" \
    "auth.$domain" \
    "dash.$domain" \
    "git.$domain" \
    "ci.$domain" \
    "docker.$domain" \
    "grafana.$domain" \
    "ai.$domain" \
    "sonar.$domain" \
    "chat.$domain" \
    "openclaw.$domain" \
    "blog.$domain" \
    "paperclip.$domain" \
    "mcp.$domain"
}

tunnel_hosts() {
  if [[ -n "$hosts_input" ]]; then
    printf '%s\n' "$hosts_input" | tr ', ' '\n\n' | sed '/^$/d'
    return
  fi
  default_hosts
}

generate_config() {
  printf 'tunnel: %s\n' "$tunnel_id"
  printf 'credentials-file: %s\n' "$credentials_file"
  printf '\n'
  printf 'ingress:\n'
  while IFS= read -r host; do
    [[ -n "$host" ]] || continue
    printf '  - hostname: %s\n' "$host"
    printf '    service: %s\n' "$origin"
    printf '    originRequest:\n'
    printf '      httpHostHeader: %s\n' "$host"
    printf '      originServerName: %s\n' "$host"
  done < <(tunnel_hosts)
  printf '  - service: http_status:404\n'
}

write_config() {
  local path="$1"
  mkdir -p "$(dirname "$path")"
  generate_config >"$path"
  echo "Cloudflare Tunnel config written: $path"
}

cleanup() {
  if [[ -n "$cleanup_path" ]]; then
    rm -f "$cleanup_path"
  fi
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --print)
      print_config=true
      shift
      ;;
    --config)
      [[ $# -ge 2 ]] || die "--config requires a path"
      config_path="$2"
      shift 2
      ;;
    --validate)
      validate=true
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      die "unknown argument: $1"
      ;;
  esac
done

if [[ "$validate" == true ]] && ! command -v cloudflared >/dev/null 2>&1; then
  echo "cloudflared is required for --validate" >&2
  exit 127
fi

if [[ "$validate" == true && -z "${HOMELAB_TUNNEL_ID:-}" ]]; then
  echo "HOMELAB_TUNNEL_ID is required for --validate" >&2
  exit 1
fi

if [[ "$validate" == true && ! -f "$credentials_file" ]]; then
  echo "HOMELAB_TUNNEL_CREDENTIALS_FILE does not exist: $credentials_file" >&2
  exit 1
fi

if [[ "$print_config" == true ]]; then
  generate_config
  if [[ "$validate" == false ]]; then
    exit 0
  fi
fi

if [[ "$print_config" == false && -z "$config_path" && -n "${HOMELAB_TUNNEL_CONFIG:-}" ]]; then
  config_path="$HOMELAB_TUNNEL_CONFIG"
fi

if [[ -z "$config_path" && "$validate" == true ]]; then
  cleanup_path="$(mktemp)"
  trap cleanup EXIT
  config_path="$cleanup_path"
fi

if [[ -z "$config_path" && "$print_config" == false ]]; then
  generate_config
  exit 0
fi

if [[ -n "$config_path" ]]; then
  write_config "$config_path"
fi

if [[ "$validate" == true ]]; then
  cloudflared --config "$config_path" tunnel ingress validate
fi
