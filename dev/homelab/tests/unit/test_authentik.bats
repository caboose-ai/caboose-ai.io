#!/usr/bin/env bats

setup() {
  export DOMAIN="test.example.com"
  export AUTHENTIK_TOKEN="test-token"
  export DRY_RUN="false"
  export FORCE="false"
  export VERBOSE="false"
  export HOMELAB_ENV="/dev/null"

  source "$(dirname "$BATS_TEST_FILENAME")/../../lib/common.sh"
  source "$(dirname "$BATS_TEST_FILENAME")/../../lib/authentik.sh"
}

# ── ak_get_provider ─────────────────────────────────────────────────────────

@test "ak_get_provider: extracts client_id from API response" {
  curl() {
    echo '{"results":[{"name":"portainer","client_id":"abc123","client_secret":"sec456"}]}'
  }
  export -f curl

  result=$(ak_get_provider "portainer" "client_id")
  [ "$result" = "abc123" ]
}

@test "ak_get_provider: extracts client_secret from API response" {
  curl() {
    echo '{"results":[{"name":"Forgejo","client_id":"fid","client_secret":"fsec"}]}'
  }
  export -f curl

  result=$(ak_get_provider "forgejo" "client_secret")
  [ "$result" = "fsec" ]
}

@test "ak_get_provider: returns empty when provider not found" {
  curl() {
    echo '{"results":[]}'
  }
  export -f curl

  result=$(ak_get_provider "nonexistent" "client_id")
  [ "$result" = "" ]
}

@test "ak_get_provider: case-insensitive matching" {
  curl() {
    echo '{"results":[{"name":"Grafana","client_id":"gid","client_secret":"gsec"}]}'
  }
  export -f curl

  result=$(ak_get_provider "GRAFANA" "client_id")
  [ "$result" = "gid" ]
}

@test "ak_get_provider: fails when AUTHENTIK_TOKEN unset" {
  unset AUTHENTIK_TOKEN
  run ak_get_provider "test" "client_id"
  [ "$status" -ne 0 ]
}

# ── ak_get_source_pk ───────────────────────────────────────────────────────

@test "ak_get_source_pk: returns pk when source exists" {
  curl() {
    echo '{"results":[{"pk":"a1b2-c3d4","slug":"github"}]}'
  }
  export -f curl

  result=$(ak_get_source_pk "github")
  [ "$result" = "a1b2-c3d4" ]
}

@test "ak_get_source_pk: returns empty when source missing" {
  curl() {
    echo '{"results":[]}'
  }
  export -f curl

  result=$(ak_get_source_pk "missing")
  [ "$result" = "" ]
}

# ── ak_health_check ────────────────────────────────────────────────────────

@test "ak_health_check: succeeds with valid response" {
  curl() {
    case "$*" in
      *root/config*)
        echo '{"version_current":"2026.2.2"}'
        ;;
      *providers/oauth2*)
        echo '{"results":[{"name":"portainer","client_id":"abc"}]}'
        ;;
    esac
  }
  export -f curl

  run ak_health_check
  [ "$status" -eq 0 ]
  [[ "$output" == *"healthy"* ]]
  [[ "$output" == *"2026.2.2"* ]]
}

@test "ak_health_check: fails when API unreachable" {
  curl() { return 1; }
  export -f curl

  run ak_health_check
  [ "$status" -ne 0 ]
  [[ "$output" == *"Cannot reach"* ]]
}
