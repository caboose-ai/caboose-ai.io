#!/usr/bin/env bash
set -euo pipefail

# rotate-leaked-secrets.sh
# Rotates CAB-3 leaked key inventory without printing secret values.
#
# Usage:
#   DOMAIN=caboose-ai.io dev/homelab/rotate-leaked-secrets.sh
#
# Optional:
#   HOMELAB_DIR=/opt/homelab
#   HOMELAB_ENV=/opt/homelab/.env
#   FORGEJO_ADMIN_USERNAME=admin
#   SKIP_SSO_CHECK=true

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
default_homelab_dir="/opt/homelab"
if [[ ! -f "${default_homelab_dir}/docker-compose.yml" ]]; then
  default_homelab_dir="${SCRIPT_DIR}"
fi
export HOMELAB_DIR="${HOMELAB_DIR:-${default_homelab_dir}}"
COMPOSE_FILE="${HOMELAB_DIR}/docker-compose.yml"
export HOMELAB_ENV="${HOMELAB_ENV:-${HOMELAB_DIR}/.env}"

if [[ -z "${DOMAIN:-}" ]]; then
  echo "ERROR: DOMAIN must be set (example: DOMAIN=caboose-ai.io)" >&2
  exit 1
fi

if [[ ! -f "${COMPOSE_FILE}" ]]; then
  echo "ERROR: docker compose file not found at ${COMPOSE_FILE}" >&2
  exit 1
fi

if ! command -v docker >/dev/null 2>&1; then
  echo "ERROR: docker is required" >&2
  exit 1
fi

source "${SCRIPT_DIR}/lib/common.sh"
require_cmd docker

log_step "CAB-3 leaked key rotation started"

rand_hex() {
  local bytes="$1"
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -hex "$bytes"
    return 0
  fi
  od -An -N "$bytes" -tx1 /dev/urandom | tr -d ' \n'
}

rand_alnum() {
  local length="$1"
  local out="" chunk=""
  while ((${#out} < length)); do
    chunk="$(rand_hex 32 | tr -dc 'A-Za-z0-9')"
    out="${out}${chunk}"
  done
  echo "${out:0:length}"
}

rotate_authentik_pg_pass() {
  local new_pass
  new_pass="$(rand_alnum 32)"
  log_info "Rotating AUTHENTIK_PG_PASS and Authentik DB role password"
  docker exec -u postgres authentik-postgres psql -d postgres -v ON_ERROR_STOP=1 -c "ALTER USER authentik WITH PASSWORD '${new_pass}';" >/dev/null
  secret_put AUTHENTIK_PG_PASS "$new_pass"
}

rotate_authentik_secret_key() {
  local new_key
  new_key="$(rand_hex 25)"
  log_info "Rotating AUTHENTIK_SECRET_KEY"
  secret_put AUTHENTIK_SECRET_KEY "$new_key"
}

rotate_authentik_bootstrap_password() {
  local new_pass escaped
  new_pass="$(rand_alnum 32)"
  escaped="${new_pass//\\/\\\\}"
  escaped="${escaped//\"/\\\"}"
  log_info "Rotating AUTHENTIK_BOOTSTRAP_PASSWORD and admin user password"
  docker exec authentik-server sh -c "PATH=/ak-root/.venv/bin:\$PATH /lifecycle/ak shell -c \"from authentik.core.models import User; u = User.objects.filter(username='auth-admin').first() or User.objects.filter(username='akadmin').first(); u and (u.set_password('${escaped}') or u.save())\"" >/dev/null
  secret_put AUTHENTIK_BOOTSTRAP_PASSWORD "$new_pass"
}

rotate_authentik_bootstrap_token() {
  local out token
  log_info "Rotating AUTHENTIK_BOOTSTRAP_TOKEN"
  out="$(docker exec authentik-server sh -c "PATH=/ak-root/.venv/bin:\$PATH /lifecycle/ak shell -c \"import json; from authentik.core.models import Group, Token, TokenIntents, User, UserTypes, default_token_key; u = User.objects.filter(username='auth-admin').first() or User.objects.filter(username='akadmin').first(); u = u or User.objects.create(username='auth-admin', name='authentik Default Admin', type=UserTypes.INTERNAL); u.is_active = True; u.save(); g, _ = Group.objects.get_or_create(name='authentik Admins', defaults={'is_superuser': True}); g.is_superuser = True; g.save(update_fields=['is_superuser']); g.users.add(u); t, _ = Token.objects.update_or_create(identifier='authentik-bootstrap-token', defaults={'intent': TokenIntents.INTENT_API, 'user': u, 'expiring': False, 'key': default_token_key()}); print(json.dumps({'token': t.key}))\"")"
  token="$(echo "$out" | grep -o '"token"[[:space:]]*:[[:space:]]*"[^"]*"' | head -n1 | sed -E 's/^"token"[[:space:]]*:[[:space:]]*"([^"]*)"$/\1/')"
  if [[ -z "${token}" ]]; then
    log_error "Failed to parse regenerated bootstrap token"
    return 1
  fi
  secret_put AUTHENTIK_BOOTSTRAP_TOKEN "$token"
}

rotate_woodpecker_agent_secret() {
  local new_secret
  new_secret="$(rand_alnum 32)"
  log_info "Rotating WOODPECKER_AGENT_SECRET"
  secret_put WOODPECKER_AGENT_SECRET "$new_secret"
}

rotate_grafana_admin_password() {
  local new_pass
  new_pass="$(rand_alnum 32)"
  log_info "Rotating GRAFANA_ADMIN_PASSWORD and Grafana admin account"
  docker exec grafana grafana cli --homepath /usr/share/grafana admin reset-admin-password "$new_pass" >/dev/null
  secret_put GRAFANA_ADMIN_PASSWORD "$new_pass"
}

rotate_gitea_admin_password() {
  local new_pass user
  new_pass="$(rand_alnum 32)"
  user="${FORGEJO_ADMIN_USERNAME:-admin}"
  if [[ ! "$user" =~ ^[A-Za-z0-9_.-]+$ ]]; then
    log_error "FORGEJO_ADMIN_USERNAME contains unsupported characters"
    return 1
  fi
  log_info "Rotating GITEA_ADMIN_PASSWORD and Forgejo admin account"
  docker exec forgejo su git -c "gitea admin user change-password --username ${user} --password ${new_pass}" >/dev/null
  secret_put GITEA_ADMIN_PASSWORD "$new_pass"
}

rotate_sonar_db_pass() {
  local new_pass
  new_pass="$(rand_alnum 32)"
  log_info "Rotating SONAR_DB_PASS and Sonar PostgreSQL role password"
  docker exec -u postgres sonarqube-db psql -d postgres -v ON_ERROR_STOP=1 -c "ALTER USER sonar WITH PASSWORD '${new_pass}';" >/dev/null
  secret_put SONAR_DB_PASS "$new_pass"
}

rotate_n8n_credentials() {
  local new_user new_pass
  new_user="n8n-admin-$(date +%Y%m%d%H%M%S)"
  new_pass="$(rand_alnum 32)"
  log_info "Rotating N8N_USER and N8N_PASSWORD"
  secret_put N8N_USER "$new_user"
  secret_put N8N_PASSWORD "$new_pass"
}

restart_impacted_services() {
  log_step "Restarting impacted services"
  docker compose -f "${COMPOSE_FILE}" up -d \
    authentik-postgres authentik-server authentik-worker \
    woodpecker-server woodpecker-agent \
    grafana \
    sonarqube-db sonarqube \
    forgejo >/dev/null
  log_success "Impacted services restarted"
}

run_post_checks() {
  if [[ "${SKIP_SSO_CHECK:-false}" == "true" ]]; then
    log_warn "Skipping SSO checks because SKIP_SSO_CHECK=true"
    return 0
  fi
  log_step "Post-rotation health checks"
  if ! command -v mise >/dev/null 2>&1; then
    log_warn "Skipping SSO checks because mise is not available"
    return 0
  fi
  (cd "${REPO_ROOT}" && HOMELAB_COMPOSE_DIR="${HOMELAB_DIR}" mise run sso:check-quick)
}

rotate_authentik_pg_pass
rotate_authentik_secret_key
rotate_authentik_bootstrap_password
rotate_authentik_bootstrap_token
rotate_woodpecker_agent_secret
rotate_grafana_admin_password
rotate_gitea_admin_password
rotate_sonar_db_pass
rotate_n8n_credentials
restart_impacted_services
run_post_checks

log_success "CAB-3 leaked key rotation complete"
