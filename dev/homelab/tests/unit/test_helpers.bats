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

@test "upsert_env_var: adds new key when not present" {
  upsert_env_var "$TEST_ENV" "NEW_KEY" "newvalue"
  run grep "^NEW_KEY=newvalue" "$TEST_ENV"
  [ "$status" -eq 0 ]
}

@test "upsert_env_var: replaces existing key value" {
  echo "EXISTING_KEY=oldvalue" >> "$TEST_ENV"
  upsert_env_var "$TEST_ENV" "EXISTING_KEY" "newvalue"
  run grep "^EXISTING_KEY=newvalue" "$TEST_ENV"
  [ "$status" -eq 0 ]
  run grep "oldvalue" "$TEST_ENV"
  [ "$status" -ne 0 ]
}

@test "upsert_env_var: does not duplicate key" {
  echo "DUP_KEY=value1" >> "$TEST_ENV"
  upsert_env_var "$TEST_ENV" "DUP_KEY" "value2"
  count=$(grep -c "^DUP_KEY=" "$TEST_ENV")
  [ "$count" -eq 1 ]
}

@test "get_forgejo_oauth_app_id: returns empty string when app not found" {
  curl() { echo "[]"; }
  export -f curl
  result=$(get_forgejo_oauth_app_id "http://fake:3000" "user:pass" "Woodpecker CI")
  [ "$result" = "" ]
}

@test "get_forgejo_oauth_app_id: returns id when app exists" {
  curl() { echo '[{"id": 42, "name": "Woodpecker CI", "client_id": "abc"}]'; }
  export -f curl
  result=$(get_forgejo_oauth_app_id "http://fake:3000" "user:pass" "Woodpecker CI")
  [ "$result" = "42" ]
}

@test "get_forgejo_oauth_app_id: returns empty when different app exists" {
  curl() { echo '[{"id": 99, "name": "Other App", "client_id": "xyz"}]'; }
  export -f curl
  result=$(get_forgejo_oauth_app_id "http://fake:3000" "user:pass" "Woodpecker CI")
  [ "$result" = "" ]
}
