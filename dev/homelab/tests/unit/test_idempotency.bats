#!/usr/bin/env bats

setup() {
  TEST_ENV=$(mktemp)
  echo "GITEA_ADMIN_PASSWORD=testpass123" > "$TEST_ENV"
  export HOMELAB_ENV="$TEST_ENV"
  export HOMELAB_COMPOSE="/dev/null"
  export AUTHENTIK_TOKEN="dummy"
  export AUTHENTIK_URL="https://fake.example.com"
  export PORTAINER_ADMIN_PASS="dummy"
  export FORGEJO_CONTAINER="forgejo"
  export FINISH_SSO_LIB_ONLY=true
  source "$(dirname "$BATS_TEST_FILENAME")/../../finish-sso.sh"
}

teardown() {
  rm -f "$TEST_ENV"
}

@test "upsert_env_var: calling twice with same value is safe" {
  upsert_env_var "$TEST_ENV" "IDEM_KEY" "value"
  upsert_env_var "$TEST_ENV" "IDEM_KEY" "value"
  count=$(grep -c "^IDEM_KEY=" "$TEST_ENV")
  [ "$count" -eq 1 ]
}

@test "get_forgejo_oauth_app_id: skips creation when app already exists" {
  # When app exists, function returns its id (non-empty) so caller skips POST
  curl() { echo '[{"id": 7, "name": "Woodpecker CI", "client_id": "existing_id"}]'; }
  export -f curl
  result=$(get_forgejo_oauth_app_id "http://fake:3000" "user:pass" "Woodpecker CI")
  [ -n "$result" ]  # non-empty means "exists, skip creation"
}
