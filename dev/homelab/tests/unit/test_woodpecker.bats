#!/usr/bin/env bats

setup() {
  TEST_ENV=$(mktemp)
  echo "GITEA_ADMIN_PASSWORD=testpass123" > "$TEST_ENV"

  export DOMAIN="test.example.com"
  export AUTHENTIK_TOKEN="test-token"
  export DRY_RUN="false"
  export FORCE="false"
  export VERBOSE="false"
  export HOMELAB_ENV="$TEST_ENV"
  export FORGEJO_CONTAINER="forgejo"
  export FORGEJO_ADMIN_USERNAME="admin"

  source "$(dirname "$BATS_TEST_FILENAME")/../../lib/common.sh"
  source "$(dirname "$BATS_TEST_FILENAME")/../../lib/authentik.sh"
  source "$(dirname "$BATS_TEST_FILENAME")/../../lib/forgejo.sh"
  source "$(dirname "$BATS_TEST_FILENAME")/../../lib/woodpecker.sh"
}

teardown() {
  rm -f "$TEST_ENV"
}

# ── Helpers ────────────────────────────────────────────────────────────────

mock_docker_with_ip() {
  docker() {
    case "$*" in
      *inspect*)
        echo "172.18.0.5"
        ;;
    esac
  }
  export -f docker
}

# ── woodpecker_find_oauth_app ──────────────────────────────────────────────

@test "woodpecker_find_oauth_app: returns app id when exists" {
  curl() {
    echo '[{"id":42,"name":"Woodpecker CI","client_id":"wid"}]'
  }
  export -f curl

  result=$(woodpecker_find_oauth_app "http://localhost:3000" "admin:pass")
  [ "$result" = "42" ]
}

@test "woodpecker_find_oauth_app: returns empty when not found" {
  curl() {
    echo '[{"id":1,"name":"Other App","client_id":"oid"}]'
  }
  export -f curl

  result=$(woodpecker_find_oauth_app "http://localhost:3000" "admin:pass")
  [ "$result" = "" ]
}

# ── setup_woodpecker — fresh install ───────────────────────────────────────

@test "setup_woodpecker: creates app and writes credentials" {
  docker() {
    case "$*" in
      *inspect*)
        echo "172.18.0.5"
        ;;
    esac
  }
  export -f docker

  curl() {
    case "$*" in
      *POST*applications/oauth2*)
        echo '{"client_id":"new-cid","client_secret":"new-csec"}'
        ;;
      *applications/oauth2*)
        echo '[]'
        ;;
    esac
  }
  export -f curl

  run setup_woodpecker
  [ "$status" -eq 0 ]
  [[ "$output" == *"configured"* ]]

  run grep "WOODPECKER_GITEA_CLIENT=new-cid" "$TEST_ENV"
  [ "$status" -eq 0 ]
  run grep "WOODPECKER_GITEA_SECRET=new-csec" "$TEST_ENV"
  [ "$status" -eq 0 ]
}

# ── setup_woodpecker — app exists, secret in .env ─────────────────────────

@test "setup_woodpecker: reuses credentials when app exists and secret in .env" {
  echo "WOODPECKER_GITEA_SECRET=existing-secret" >> "$TEST_ENV"

  docker() {
    case "$*" in
      *inspect*)
        echo "172.18.0.5"
        ;;
    esac
  }
  export -f docker

  curl() {
    case "$*" in
      *applications/oauth2*)
        echo '[{"id":42,"name":"Woodpecker CI","client_id":"existing-cid"}]'
        ;;
    esac
  }
  export -f curl

  run setup_woodpecker
  [ "$status" -eq 0 ]
  [[ "$output" == *"already configured"* ]]
}

# ── setup_woodpecker — app exists, no secret ──────────────────────────────

@test "setup_woodpecker: errors when app exists but no secret in .env" {
  docker() {
    case "$*" in
      *inspect*)
        echo "172.18.0.5"
        ;;
    esac
  }
  export -f docker

  curl() {
    case "$*" in
      *applications/oauth2*)
        echo '[{"id":42,"name":"Woodpecker CI","client_id":"existing-cid"}]'
        ;;
    esac
  }
  export -f curl

  run setup_woodpecker
  [ "$status" -ne 0 ]
  [[ "$output" == *"--force"* ]]
}

# ── setup_woodpecker — force recreate ─────────────────────────────────────

@test "setup_woodpecker: --force deletes and recreates app" {
  FORCE="true"

  docker() {
    case "$*" in
      *inspect*)
        echo "172.18.0.5"
        ;;
    esac
  }
  export -f docker

  curl() {
    case "$*" in
      *DELETE*applications/oauth2*)
        ;;
      *POST*applications/oauth2*)
        echo '{"client_id":"fresh-cid","client_secret":"fresh-csec"}'
        ;;
      *applications/oauth2*)
        echo '[{"id":42,"name":"Woodpecker CI","client_id":"old-cid"}]'
        ;;
    esac
  }
  export -f curl

  run setup_woodpecker
  [ "$status" -eq 0 ]
  [[ "$output" == *"configured"* ]]

  run grep "WOODPECKER_GITEA_CLIENT=fresh-cid" "$TEST_ENV"
  [ "$status" -eq 0 ]
  run grep "WOODPECKER_GITEA_SECRET=fresh-csec" "$TEST_ENV"
  [ "$status" -eq 0 ]
}

# ── setup_woodpecker — dry-run ────────────────────────────────────────────

@test "setup_woodpecker: dry-run does not create app or write env" {
  DRY_RUN="true"

  docker() {
    case "$*" in
      *inspect*)
        echo "172.18.0.5"
        ;;
    esac
  }
  export -f docker

  curl() {
    case "$*" in
      *applications/oauth2*)
        echo '[]'
        ;;
    esac
  }
  export -f curl

  run setup_woodpecker
  [ "$status" -eq 0 ]
  [[ "$output" == *"dry-run"* ]]

  run grep "WOODPECKER_GITEA_CLIENT" "$TEST_ENV"
  [ "$status" -ne 0 ]
}

# ── Redirect URL derives from DOMAIN ──────────────────────────────────────

@test "setup_woodpecker: redirect URL uses DOMAIN" {
  [ "$WOODPECKER_REDIRECT" = "https://ci.test.example.com/authorize" ]
}
