#!/usr/bin/env bats

setup() {
  export DOMAIN="test.example.com"
  export AUTHENTIK_TOKEN="test-token"
  export PORTAINER_ADMIN_PASS="testpass"
  export DRY_RUN="false"
  export FORCE="false"
  export VERBOSE="false"
  export HOMELAB_ENV="/dev/null"
  export FORGEJO_CONTAINER="forgejo"

  source "$(dirname "$BATS_TEST_FILENAME")/../../lib/common.sh"
  source "$(dirname "$BATS_TEST_FILENAME")/../../lib/authentik.sh"
  source "$(dirname "$BATS_TEST_FILENAME")/../../lib/portainer.sh"
}

# ── portainer_get_jwt ──────────────────────────────────────────────────────

@test "portainer_get_jwt: returns JWT on success" {
  curl() {
    echo '{"jwt":"eyJhbGciOiJ.test.token"}'
  }
  export -f curl

  result=$(portainer_get_jwt)
  [ "$result" = "eyJhbGciOiJ.test.token" ]
}

@test "portainer_get_jwt: fails when PORTAINER_ADMIN_PASS unset" {
  unset PORTAINER_ADMIN_PASS
  run portainer_get_jwt
  [ "$status" -ne 0 ]
}

@test "portainer_get_jwt: fails when API unreachable" {
  curl() { return 1; }
  export -f curl

  run portainer_get_jwt
  [ "$status" -ne 0 ]
  [[ "$output" == *"Cannot authenticate"* ]]
}

# ── portainer_current_oauth_client ─────────────────────────────────────────

@test "portainer_current_oauth_client: returns client_id when configured" {
  curl() {
    echo '{"OAuthSettings":{"ClientID":"abc-existing"}}'
  }
  export -f curl

  result=$(portainer_current_oauth_client "fake-jwt")
  [ "$result" = "abc-existing" ]
}

@test "portainer_current_oauth_client: returns empty when no OAuth" {
  curl() {
    echo '{"OAuthSettings":{}}'
  }
  export -f curl

  result=$(portainer_current_oauth_client "fake-jwt")
  [ "$result" = "" ]
}

# ── setup_portainer — fresh install ────────────────────────────────────────

@test "setup_portainer: configures OAuth when not yet set" {
  curl() {
    case "$*" in
      *"providers/oauth2"*)
        echo '{"results":[{"name":"portainer","client_id":"pid123","client_secret":"psec456"}]}'
        ;;
      *"/api/auth"*)
        echo '{"jwt":"test-jwt"}'
        ;;
      *"/api/settings"*"pid123"*)
        echo '{}'
        ;;
      *"/api/settings"*)
        echo '{"OAuthSettings":{}}'
        ;;
    esac
  }
  export -f curl

  run setup_portainer
  [ "$status" -eq 0 ]
  [[ "$output" == *"configured"* ]]
}

# ── setup_portainer — already configured ───────────────────────────────────

@test "setup_portainer: skips when already configured with same client_id" {
  curl() {
    case "$*" in
      *"providers/oauth2"*)
        echo '{"results":[{"name":"portainer","client_id":"pid123","client_secret":"psec456"}]}'
        ;;
      *"/api/auth"*)
        echo '{"jwt":"test-jwt"}'
        ;;
      *"/api/settings"*)
        echo '{"OAuthSettings":{"ClientID":"pid123"}}'
        ;;
    esac
  }
  export -f curl

  run setup_portainer
  [ "$status" -eq 0 ]
  [[ "$output" == *"already configured"* ]]
}

# ── setup_portainer — force re-apply ───────────────────────────────────────

@test "setup_portainer: --force re-applies even when matching" {
  FORCE="true"

  curl() {
    case "$*" in
      *"providers/oauth2"*)
        echo '{"results":[{"name":"portainer","client_id":"pid123","client_secret":"psec456"}]}'
        ;;
      *"/api/auth"*)
        echo '{"jwt":"test-jwt"}'
        ;;
      *"/api/settings"*"pid123"*"psec456"*)
        echo '{}'
        ;;
      *"/api/settings"*)
        echo '{"OAuthSettings":{"ClientID":"pid123"}}'
        ;;
    esac
  }
  export -f curl

  run setup_portainer
  [ "$status" -eq 0 ]
  [[ "$output" == *"configured"* ]]
}

# ── setup_portainer — provider missing ─────────────────────────────────────

@test "setup_portainer: fails when provider not in Authentik" {
  curl() {
    echo '{"results":[]}'
  }
  export -f curl

  run setup_portainer
  [ "$status" -ne 0 ]
  [[ "$output" == *"not found"* ]]
}

# ── setup_portainer — dry-run ──────────────────────────────────────────────

@test "setup_portainer: dry-run does not PUT settings" {
  DRY_RUN="true"

  curl() {
    case "$*" in
      *"providers/oauth2"*)
        echo '{"results":[{"name":"portainer","client_id":"pid123","client_secret":"psec456"}]}'
        ;;
      *"/api/auth"*)
        echo '{"jwt":"test-jwt"}'
        ;;
      *"/api/settings"*)
        echo '{"OAuthSettings":{}}'
        ;;
    esac
  }
  export -f curl

  run setup_portainer
  [ "$status" -eq 0 ]
  [[ "$output" == *"dry-run"* ]]
}

# ── Redirect URI derives from DOMAIN ──────────────────────────────────────

@test "setup_portainer: redirect URI uses DOMAIN" {
  [ "$PORTAINER_REDIRECT_URI" = "https://docker.test.example.com/" ]
}
