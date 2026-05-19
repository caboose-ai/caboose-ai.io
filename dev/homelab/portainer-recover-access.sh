#!/usr/bin/env bash
# Repair Portainer access when the stored homelab admin password drifted from
# the initialized Portainer database, then reapply the service configurator.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

PORTAINER_URL="${PORTAINER_URL:-http://127.0.0.1:9000}"
PORTAINER_CONTAINER="${PORTAINER_CONTAINER:-portainer}"
HOMELAB_DOMAIN="${HOMELAB_DOMAIN:-caboose-ai.io}"
HOMELAB_COMPOSE_DIR="${HOMELAB_COMPOSE_DIR:-/opt/homelab}"
HOMELAB_ENV="${HOMELAB_ENV:-}"
DRY_RUN=false
YES=false

usage() {
  cat <<EOF
Usage: $0 [--yes] [options]

Checks whether the stored Portainer admin password can authenticate locally.
If it can, the command reapplies:
  homelab service portainer configure --force

If the stored password does not authenticate, --yes is required before the
script stops Portainer, runs portainer/helper-reset-password against the live
data volume, starts Portainer again, and reapplies the service configurator.

Options:
  --yes                 Allow the Portainer password-reset repair path.
  --dry-run             Print planned actions without touching Portainer.
  --domain DOMAIN       Homelab domain. Default: $HOMELAB_DOMAIN.
  --compose-dir DIR     Homelab compose directory. Default: $HOMELAB_COMPOSE_DIR.
  --env-file PATH       Env file to read. Default: <compose-dir>/.env.
  --portainer-url URL   Local Portainer API URL. Default: $PORTAINER_URL.
  --container NAME      Portainer container name. Default: $PORTAINER_CONTAINER.
  --homelab-bin PATH    Use an installed homelab binary instead of go run.
  -h, --help            Show this help.
EOF
}

log() {
  printf '%s\n' "$*" >&2
}

die() {
  log "error: $*"
  exit 1
}

join_args() {
  local out="" arg
  for arg in "$@"; do
    if [ -n "$out" ]; then
      out+=" "
    fi
    out+="$arg"
  done
  printf '%s' "$out"
}

run_cmd() {
  log "+ $(join_args "$@")"
  "$@"
}

run_reset_helper() {
  local data_mount="$1" admin_pass="$2"
  log "+ docker run --rm -v ${data_mount}:/data portainer/helper-reset-password --password <redacted>"
  docker run --rm -v "${data_mount}:/data" portainer/helper-reset-password --password "$admin_pass"
}

PORTAINER_STOPPED=false
restart_stopped_portainer() {
  if "$PORTAINER_STOPPED"; then
    log "Portainer reset failed after stop; attempting to start ${PORTAINER_CONTAINER} before exit."
    docker start "$PORTAINER_CONTAINER" >/dev/null 2>&1 || true
  fi
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --yes)
      YES=true
      ;;
    --dry-run)
      DRY_RUN=true
      ;;
    --domain)
      shift
      [ "$#" -gt 0 ] || die "--domain requires a value"
      HOMELAB_DOMAIN="$1"
      ;;
    --compose-dir)
      shift
      [ "$#" -gt 0 ] || die "--compose-dir requires a value"
      HOMELAB_COMPOSE_DIR="$1"
      ;;
    --env-file)
      shift
      [ "$#" -gt 0 ] || die "--env-file requires a value"
      HOMELAB_ENV="$1"
      ;;
    --portainer-url)
      shift
      [ "$#" -gt 0 ] || die "--portainer-url requires a value"
      PORTAINER_URL="$1"
      ;;
    --container)
      shift
      [ "$#" -gt 0 ] || die "--container requires a value"
      PORTAINER_CONTAINER="$1"
      ;;
    --homelab-bin)
      shift
      [ "$#" -gt 0 ] || die "--homelab-bin requires a value"
      HOMELAB_BIN="$1"
      ;;
    -h | --help)
      usage
      exit 0
      ;;
    *)
      die "unknown option: $1"
      ;;
  esac
  shift
done

if [ -z "$HOMELAB_ENV" ]; then
  HOMELAB_ENV="${HOMELAB_COMPOSE_DIR}/.env"
fi

if "$DRY_RUN"; then
  printf 'would read Portainer admin password from %s\n' "$HOMELAB_ENV"
  printf 'would check Portainer health at %s/api/system/status\n' "$PORTAINER_URL"
  printf 'would authenticate Portainer admin without printing the password\n'
  printf 'would run homelab service --domain %s --compose-dir %s portainer configure --force when admin auth works\n' "$HOMELAB_DOMAIN" "$HOMELAB_COMPOSE_DIR"
  if "$YES"; then
    printf 'would inspect %s for its /data mount\n' "$PORTAINER_CONTAINER"
    printf 'would stop %s\n' "$PORTAINER_CONTAINER"
    printf 'would reset Portainer admin password with portainer/helper-reset-password\n'
    printf 'would start %s and re-run Portainer service configuration\n' "$PORTAINER_CONTAINER"
  else
    printf 'would require --yes before password-reset repair if admin auth fails\n'
  fi
  exit 0
fi

command -v curl >/dev/null 2>&1 || die "curl is required"
command -v jq >/dev/null 2>&1 || die "jq is required"
command -v docker >/dev/null 2>&1 || die "docker is required"

read_env_key() {
  local key="$1"
  grep -m 1 "^${key}=" "$HOMELAB_ENV" 2>/dev/null | cut -d= -f2- || true
}

admin_pass="${PORTAINER_ADMIN_PASSWORD:-${PORTAINER_ADMIN_PASS:-}}"
if [ -z "$admin_pass" ]; then
  admin_pass="$(read_env_key PORTAINER_ADMIN_PASSWORD)"
fi
if [ -z "$admin_pass" ]; then
  admin_pass="$(read_env_key PORTAINER_ADMIN_PASS)"
fi
[ -n "$admin_pass" ] || die "PORTAINER_ADMIN_PASSWORD is required in the environment or $HOMELAB_ENV"

check_health() {
  curl -fsS -o /dev/null "${PORTAINER_URL}/api/system/status"
}

portainer_login() {
  local response body code jwt payload
  payload="$(jq -cn --arg username "admin" --arg password "$admin_pass" '{Username:$username,Password:$password}')"
  response="$(curl -sS -X POST "${PORTAINER_URL}/api/auth" \
    -H "Content-Type: application/json" \
    -d "$payload" \
    -w '\n%{http_code}')" || return 1
  code="${response##*$'\n'}"
  body="${response%$'\n'*}"
  if [ "$code" = "200" ]; then
    jwt="$(printf '%s' "$body" | jq -er '.jwt // empty' 2>/dev/null)" || return 1
    [ -n "$jwt" ] || return 1
    return 0
  fi
  if [ "$code" = "401" ] || [ "$code" = "422" ]; then
    return 2
  fi
  log "Portainer auth returned HTTP ${code}"
  return 1
}

portainer_data_mount() {
  docker inspect "$PORTAINER_CONTAINER" \
    --format '{{range .Mounts}}{{if eq .Destination "/data"}}{{if .Name}}{{.Name}}{{else}}{{.Source}}{{end}}{{end}}{{end}}'
}

wait_for_portainer() {
  local attempt
  for attempt in $(seq 1 30); do
    if check_health >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  return 1
}

configure_portainer() {
  local homelab_cmd
  if [ -n "${HOMELAB_BIN:-}" ]; then
    homelab_cmd=("$HOMELAB_BIN")
  else
    homelab_cmd=(go run ./cmd/homelab)
  fi
  (cd "$REPO_ROOT" && run_cmd "${homelab_cmd[@]}" service --domain "$HOMELAB_DOMAIN" --compose-dir "$HOMELAB_COMPOSE_DIR" portainer configure --force)
}

log "Checking Portainer health at ${PORTAINER_URL}"
check_health || die "Portainer health check failed at ${PORTAINER_URL}/api/system/status"

login_status=0
portainer_login || login_status="$?"
case "$login_status" in
  0)
    log "Stored Portainer admin password authenticates; reapplying service configuration."
    configure_portainer
    exit 0
    ;;
  2)
    if ! "$YES"; then
      die "stored Portainer admin password does not authenticate; rerun with --yes to reset the Portainer admin password to the stored secret and reapply service configuration"
    fi
    ;;
  *)
    die "failed to authenticate with Portainer"
    ;;
esac

data_mount="$(portainer_data_mount)"
[ -n "$data_mount" ] || die "could not find Portainer /data mount on container ${PORTAINER_CONTAINER}"

log "Resetting Portainer admin password to the stored secret using the live data mount."
trap restart_stopped_portainer EXIT
run_cmd docker stop "$PORTAINER_CONTAINER"
PORTAINER_STOPPED=true
run_reset_helper "$data_mount" "$admin_pass"
run_cmd docker start "$PORTAINER_CONTAINER"
PORTAINER_STOPPED=false
trap - EXIT

wait_for_portainer || die "Portainer did not become healthy after restart"
portainer_login || die "Portainer admin password reset completed, but the stored password still does not authenticate"
configure_portainer
