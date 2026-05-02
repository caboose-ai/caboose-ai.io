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

  source "$(dirname "$BATS_TEST_FILENAME")/../../lib/common.sh"
  source "$(dirname "$BATS_TEST_FILENAME")/../../lib/authentik.sh"
  source "$(dirname "$BATS_TEST_FILENAME")/../../lib/forgejo.sh"
}

teardown() {
  rm -f "$TEST_ENV"
}

# ── forgejo_admin_password ─────────────────────────────────────────────────

@test "forgejo_admin_password: reads from HOMELAB_ENV" {
  result=$(forgejo_admin_password)
  [ "$result" = "testpass123" ]
}

@test "forgejo_admin_password: fails when key missing" {
  echo "" > "$TEST_ENV"
  run forgejo_admin_password
  [ "$status" -ne 0 ]
  [[ "$output" == *"GITEA_ADMIN_PASSWORD"* ]]
}

# ── forgejo_auth_source_id ─────────────────────────────────────────────────

@test "forgejo_auth_source_id: extracts ID from gitea auth list" {
  docker() {
    cat <<'AUTHLIST'
ID   Name       Type
1    Authentik   OAuth2
AUTHLIST
  }
  export -f docker

  result=$(forgejo_auth_source_id "Authentik")
  [ "$result" = "1" ]
}

@test "forgejo_auth_source_id: returns empty when source not found" {
  docker() {
    cat <<'AUTHLIST'
ID   Name       Type
1    GitHub      OAuth2
AUTHLIST
  }
  export -f docker

  result=$(forgejo_auth_source_id "Authentik")
  [ "$result" = "" ]
}

@test "forgejo_auth_source_id: handles no auth sources" {
  docker() {
    echo "ID   Name   Type"
  }
  export -f docker

  result=$(forgejo_auth_source_id "Authentik")
  [ "$result" = "" ]
}

# ── setup_forgejo — fresh install ──────────────────────────────────────────

@test "setup_forgejo: creates OIDC when no auth source exists" {
  local add_called=false

  curl() {
    echo '{"results":[{"name":"forgejo","client_id":"fid123","client_secret":"fsec456"}]}'
  }
  export -f curl

  docker() {
    case "$*" in
      *"auth list"*)
        echo "ID   Name   Type"
        ;;
      *"auth add-oauth"*)
        add_called=true
        [[ "$*" == *"--key"*"fid123"* ]] || { echo "bad client_id"; return 1; }
        [[ "$*" == *"--secret"*"fsec456"* ]] || { echo "bad client_secret"; return 1; }
        [[ "$*" == *"openid-configuration"* ]] || { echo "bad discovery url"; return 1; }
        ;;
    esac
  }
  export -f docker

  run setup_forgejo
  [ "$status" -eq 0 ]
  [[ "$output" == *"created"* ]]
}

# ── setup_forgejo — idempotent update ──────────────────────────────────────

@test "setup_forgejo: updates in place when auth source exists" {
  curl() {
    echo '{"results":[{"name":"forgejo","client_id":"fid123","client_secret":"fsec456"}]}'
  }
  export -f curl

  docker() {
    case "$*" in
      *"auth list"*)
        cat <<'AUTHLIST'
ID   Name       Type
5    Authentik   OAuth2
AUTHLIST
        ;;
      *"auth update-oauth"*)
        [[ "$*" == *"--id"*"5"* ]] || { echo "wrong id"; return 1; }
        ;;
      *"auth delete"*)
        echo "SHOULD NOT DELETE"; return 1
        ;;
    esac
  }
  export -f docker

  run setup_forgejo
  [ "$status" -eq 0 ]
  [[ "$output" == *"updated"* ]]
}

# ── setup_forgejo — force recreate ─────────────────────────────────────────

@test "setup_forgejo: --force deletes and re-creates" {
  FORCE="true"

  curl() {
    echo '{"results":[{"name":"forgejo","client_id":"fid123","client_secret":"fsec456"}]}'
  }
  export -f curl

  local deleted=false
  local added=false

  docker() {
    case "$*" in
      *"auth list"*)
        cat <<'AUTHLIST'
ID   Name       Type
5    Authentik   OAuth2
AUTHLIST
        ;;
      *"auth delete"*)
        deleted=true
        ;;
      *"auth add-oauth"*)
        added=true
        ;;
      *"auth update-oauth"*)
        echo "SHOULD NOT UPDATE"; return 1
        ;;
    esac
  }
  export -f docker

  run setup_forgejo
  [ "$status" -eq 0 ]
  [[ "$output" == *"re-created"* ]]
}

# ── setup_forgejo — provider missing ───────────────────────────────────────

@test "setup_forgejo: fails when provider not in Authentik" {
  curl() {
    echo '{"results":[]}'
  }
  export -f curl

  run setup_forgejo
  [ "$status" -ne 0 ]
  [[ "$output" == *"not found"* ]]
}

# ── setup_forgejo — dry-run ────────────────────────────────────────────────

@test "setup_forgejo: dry-run does not call docker exec" {
  DRY_RUN="true"

  curl() {
    echo '{"results":[{"name":"forgejo","client_id":"fid123","client_secret":"fsec456"}]}'
  }
  export -f curl

  docker() {
    case "$*" in
      *"auth list"*)
        echo "ID   Name   Type"
        ;;
      *"auth add-oauth"*)
        echo "SHOULD NOT RUN IN DRY RUN"; return 1
        ;;
    esac
  }
  export -f docker

  run setup_forgejo
  [ "$status" -eq 0 ]
  [[ "$output" == *"dry-run"* ]]
}

# ── Discovery URL uses DOMAIN ─────────────────────────────────────────────

@test "setup_forgejo: discovery URL derives from DOMAIN" {
  curl() {
    echo '{"results":[{"name":"forgejo","client_id":"fid","client_secret":"fsec"}]}'
  }
  export -f curl

  local captured_url=""
  docker() {
    case "$*" in
      *"auth list"*)
        echo "ID   Name   Type"
        ;;
      *"auth add-oauth"*)
        captured_url="$*"
        ;;
    esac
  }
  export -f docker

  run setup_forgejo
  [ "$status" -eq 0 ]
  [[ "$output" == *"auth.test.example.com"* ]] || [[ "$output" == *"created"* ]]
}
