#!/usr/bin/env bash
# sso.sh — Unified SSO configuration CLI for homelab
#
# Usage:
#   sso.sh <subcommand> [--dry-run] [--force] [--verbose]
#
# Subcommands:
#   status           Check Authentik health and list configured providers
#   setup-forgejo    Configure Forgejo OIDC via Authentik
#   setup-portainer  Configure Portainer OAuth via Authentik
#   setup-woodpecker Create Woodpecker OAuth app in Forgejo
#   setup-grafana    Populate Grafana OAuth credentials in .env
#   setup-open-webui Populate Open-WebUI OIDC credentials in .env
#   setup-mattermost Patch Mattermost config.json for Authentik OIDC
#   setup-social     Add GitHub/Google social login sources to Authentik
#   all              Run all setup subcommands in dependency order
#
# Environment:
#   DOMAIN                 (required) Your homelab domain, e.g. example.com
#   AUTHENTIK_TOKEN        (required) Authentik API token
#   FORGEJO_ADMIN_USERNAME (optional) Forgejo admin username (default: admin)
#   PORTAINER_ADMIN_PASS   (required for setup-portainer)
#
# Flags:
#   --dry-run   Print what would happen without making changes
#   --force     Tear down and rebuild (e.g. regenerate Woodpecker secret)
#   --verbose   Show timestamps and curl details

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# ── Parse arguments ─────────────────────────────────────────────────────────

SUBCOMMAND=""
export DRY_RUN="false"
export FORCE="false"
export VERBOSE="false"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run) DRY_RUN="true"; shift ;;
    --force)   FORCE="true"; shift ;;
    --verbose) VERBOSE="true"; shift ;;
    -h|--help) SUBCOMMAND="help"; shift ;;
    -*)        echo "Unknown flag: $1" >&2; exit 1 ;;
    *)
      if [[ -z "$SUBCOMMAND" ]]; then
        SUBCOMMAND="$1"
      else
        echo "Unexpected argument: $1" >&2; exit 1
      fi
      shift
      ;;
  esac
done

[[ -z "$SUBCOMMAND" ]] && SUBCOMMAND="help"

# ── Help ────────────────────────────────────────────────────────────────────

show_help() {
  sed -n '2,/^$/{ s/^# //; s/^#//; p }' "${BASH_SOURCE[0]}"
}

if [[ "$SUBCOMMAND" == "help" ]]; then
  show_help
  exit 0
fi

# ── Source libraries ────────────────────────────────────────────────────────
# Guard: when SSO_LIB_ONLY=true, stop here (used by tests that source this file)

source "${SCRIPT_DIR}/lib/common.sh"
source "${SCRIPT_DIR}/lib/authentik.sh"
source "${SCRIPT_DIR}/lib/forgejo.sh"
source "${SCRIPT_DIR}/lib/portainer.sh"
source "${SCRIPT_DIR}/lib/woodpecker.sh"
source "${SCRIPT_DIR}/lib/grafana.sh"
source "${SCRIPT_DIR}/lib/open-webui.sh"
source "${SCRIPT_DIR}/lib/mattermost.sh"
source "${SCRIPT_DIR}/lib/social.sh"

if [[ "${SSO_LIB_ONLY:-}" == "true" ]]; then
  return 0 2>/dev/null || exit 0
fi

# ── Preflight ───────────────────────────────────────────────────────────────

require_cmd curl
require_cmd jq
require_cmd docker

if [[ "$DRY_RUN" == "true" ]]; then
  log_warn "Dry-run mode — no changes will be made"
fi

# ── Subcommand dispatch ─────────────────────────────────────────────────────

case "$SUBCOMMAND" in
  status)
    ak_health_check
    ;;

  setup-forgejo)
    setup_forgejo
    ;;

  setup-portainer)
    setup_portainer
    ;;

  setup-woodpecker)
    setup_woodpecker
    ;;

  setup-grafana)
    setup_grafana
    ;;

  setup-open-webui)
    setup_open_webui
    ;;

  setup-mattermost)
    setup_mattermost
    ;;

  setup-social)
    setup_social
    ;;

  all)
    log_step "Running all SSO setup subcommands"
    setup_forgejo
    setup_portainer
    setup_woodpecker
    setup_grafana
    setup_open_webui
    setup_mattermost
    setup_social
    log_step "All SSO setup complete"
    ;;

  *)
    log_error "Unknown subcommand: $SUBCOMMAND"
    echo "" >&2
    show_help
    exit 1
    ;;
esac
