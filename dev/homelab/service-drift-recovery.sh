#!/usr/bin/env bash
# Guarded repairs for known persistent service-state drift.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

HOMELAB_DOMAIN="${HOMELAB_DOMAIN:-caboose-ai.io}"
HOMELAB_COMPOSE_DIR="${HOMELAB_COMPOSE_DIR:-/opt/homelab}"
HOMELAB_ENV="${HOMELAB_ENV:-}"
YES=false
ACTION=""

usage() {
  cat <<EOF
Usage: $0 <recovery> [--yes] [options]

Recover known persistent service drift. Dry-run is the default; pass --yes to
mutate live containers.

Recoveries:
  sonarqube-db-password      Reset SonarQube Postgres role password to SONAR_DB_PASS.
  mattermost-db-password     Reset Mattermost Postgres role password to MATTERMOST_DB_PASS.
  sonarqube-admin-password   Reset built-in SonarQube admin to admin/admin, then rerun configurator.

Options:
  --yes                 Run the recovery instead of printing the plan.
  --domain DOMAIN       Homelab domain. Default: $HOMELAB_DOMAIN.
  --compose-dir DIR     Homelab compose directory. Default: $HOMELAB_COMPOSE_DIR.
  --env-file PATH       Env file to read. Default: <compose-dir>/.env.
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
  if ! "$YES"; then
    return 0
  fi
  "$@"
}

parse_env_value() {
  local raw="$1"
  case "$raw" in
    "'"*"'")
      raw="${raw#\'}"
      raw="${raw%\'}"
      ;;
    '"'*'"')
      raw="${raw#\"}"
      raw="${raw%\"}"
      raw="${raw//\\\"/\"}"
      raw="${raw//\\\\/\\}"
      ;;
  esac
  printf '%s' "$raw"
}

read_env_key() {
  local key="$1" line value
  line="$(grep -m 1 "^${key}=" "$HOMELAB_ENV" 2>/dev/null || true)"
  [ -n "$line" ] || return 0
  value="${line#*=}"
  parse_env_value "$value"
}

load_secret() {
  local key="$1" value
  value="${!key:-}"
  if [ -z "$value" ]; then
    value="$(read_env_key "$key")"
  fi
  [ -n "$value" ] || die "${key} is required in the environment or ${HOMELAB_ENV}"
  export "$key=$value"
}

homelab_cmd() {
  if [ -n "${HOMELAB_BIN:-}" ]; then
    printf '%s\0' "$HOMELAB_BIN"
  else
    printf '%s\0' go run ./cmd/homelab
  fi
}

recover_postgres_password() {
  local env_key="$1" db_container="$2" db_user="$3" db_name="$4" app_container="$5"
  if "$YES"; then
    load_secret "$env_key"
  fi

  run_cmd docker exec -e "$env_key" "$db_container" sh -lc \
    "psql -v ON_ERROR_STOP=1 -U $db_user -d $db_name -v new_password=\"\$$env_key\" -c \"ALTER USER $db_user WITH PASSWORD :'new_password';\""
  run_cmd docker restart "$app_container"
}

recover_sonarqube_admin_password() {
  if "$YES"; then
    load_secret SONAR_ADMIN_PASSWORD
  fi

  local reset_sql
  # SonarSource-documented PostgreSQL reset for the built-in admin account.
  reset_sql="update users set crypted_password='100000\$t2h8AtNs1AlCHuLobDjHQTn9XppwTIx88UjqUm4s8RsfTuXQHSd/fpFexAnewwPsO6jGFQUv/24DnO55hY6Xew==', salt='k9x9eN127/3e/hf38iNiKwVfaVk=', hash_method='PBKDF2', reset_password='true', user_local='true', active='true' where login='admin';"

  run_cmd docker stop sonarqube
  run_cmd docker exec sonarqube-db psql -v ON_ERROR_STOP=1 -U sonar -d sonar -c "$reset_sql"
  run_cmd docker start sonarqube

  local -a cmd=()
  while IFS= read -r -d '' arg; do
    cmd+=("$arg")
  done < <(homelab_cmd)
  (cd "$REPO_ROOT" && run_cmd "${cmd[@]}" service --domain "$HOMELAB_DOMAIN" --compose-dir "$HOMELAB_COMPOSE_DIR" sonarqube configure --force)
}

if [ "$#" -gt 0 ]; then
  ACTION="$1"
  shift
fi

while [ "$#" -gt 0 ]; do
  case "$1" in
    --yes)
      YES=true
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

[ -n "$ACTION" ] || die "recovery name is required"

if [ -z "$HOMELAB_ENV" ]; then
  HOMELAB_ENV="${HOMELAB_COMPOSE_DIR}/.env"
fi

if ! "$YES"; then
  log "dry-run only; pass --yes to mutate live state"
fi

case "$ACTION" in
  sonarqube-db-password)
    recover_postgres_password SONAR_DB_PASS sonarqube-db sonar sonar sonarqube
    ;;
  mattermost-db-password)
    recover_postgres_password MATTERMOST_DB_PASS mattermost-db mattermost mattermost mattermost
    ;;
  sonarqube-admin-password)
    recover_sonarqube_admin_password
    ;;
  *)
    die "unknown recovery: $ACTION"
    ;;
esac
