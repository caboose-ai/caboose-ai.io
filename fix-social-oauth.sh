#!/usr/bin/env bash
set -euo pipefail

TOKEN=$(grep AUTHENTIK_BOOTSTRAP_TOKEN /home/caboose/dev/caboose-ai.io/dev/homelab/.env | cut -d= -f2)
API="http://127.0.0.1:9002/api/v3"

echo "=== Fix Social OAuth Credentials ==="
echo ""

# Current state
STATIC_VAULT="${OP_STATIC_VAULT:-Homelab - Static}"

GITHUB_ID=$(op item get GITHUB_OAUTH_CLIENT_ID --vault "$STATIC_VAULT" --fields password --reveal 2>/dev/null || echo "")
GITHUB_SECRET=$(op item get GITHUB_OAUTH_CLIENT_SECRET --vault "$STATIC_VAULT" --fields password --reveal 2>/dev/null || echo "")
GOOGLE_ID=$(op item get GOOGLE_OAUTH_CLIENT_ID --vault "$STATIC_VAULT" --fields password --reveal 2>/dev/null || echo "")
GOOGLE_SECRET=$(op item get GOOGLE_OAUTH_CLIENT_SECRET --vault "$STATIC_VAULT" --fields password --reveal 2>/dev/null || echo "")

echo "Current credentials in 1Password:"
echo "  GitHub ID:     ${GITHUB_ID:-(empty)}"
echo "  GitHub Secret: ${GITHUB_SECRET:+${GITHUB_SECRET:0:8}...}"
echo "  Google ID:     ${GOOGLE_ID:-(empty)}"
echo "  Google Secret: ${GOOGLE_SECRET:+${GOOGLE_SECRET:0:8}...}"
echo ""

# Validate GitHub — ID should start with Ov23li
if [[ "$GITHUB_ID" == Ov23li* ]]; then
    echo "✓ GitHub Client ID looks valid"
else
    echo "✗ GitHub Client ID looks wrong (expected Ov23li... prefix)"
    read -rp "  Enter correct GitHub Client ID (or press Enter to skip): " new_id
    if [[ -n "$new_id" ]]; then
        op item edit GITHUB_OAUTH_CLIENT_ID --vault "$STATIC_VAULT" "password=$new_id" >/dev/null
        GITHUB_ID="$new_id"
        echo "  ✓ Updated"
    fi
fi

# Validate Google — ID should end with .apps.googleusercontent.com
if [[ "$GOOGLE_ID" == *"apps.googleusercontent.com"* ]]; then
    echo "✓ Google Client ID looks valid"
else
    echo "✗ Google Client ID is INVALID: '$GOOGLE_ID'"
    echo "  Get it from: https://console.cloud.google.com/apis/credentials"
    echo "  It looks like: 123456789-xxxxx.apps.googleusercontent.com"
    echo ""
    read -rp "  Paste your Google OAuth Client ID: " new_id
    if [[ -n "$new_id" ]]; then
        op item edit GOOGLE_OAUTH_CLIENT_ID --vault "$STATIC_VAULT" "password=$new_id" >/dev/null
        GOOGLE_ID="$new_id"
        echo "  ✓ Updated"
    else
        echo "  Skipping Google"
    fi
fi

# Validate Google secret
if [[ "$GOOGLE_SECRET" == GOCSPX-* ]]; then
    echo "✓ Google Client Secret looks valid"
else
    echo "✗ Google Client Secret looks wrong (expected GOCSPX-... prefix)"
    read -rp "  Enter correct Google Client Secret (or press Enter to skip): " new_secret
    if [[ -n "$new_secret" ]]; then
        op item edit GOOGLE_OAUTH_CLIENT_SECRET --vault "$STATIC_VAULT" "password=$new_secret" >/dev/null
        GOOGLE_SECRET="$new_secret"
        echo "  ✓ Updated"
    fi
fi

echo ""
echo "Updating Authentik sources..."

# Get flow PKs
AUTH_FLOW=$(curl -s "$API/flows/instances/?designation=authentication&slug=default-source-authentication" \
    -H "Authorization: Bearer $TOKEN" | jq -r '.results[0].pk // empty')
ENROLL_FLOW=$(curl -s "$API/flows/instances/?designation=enrollment&slug=default-source-enrollment" \
    -H "Authorization: Bearer $TOKEN" | jq -r '.results[0].pk // empty')

update_source() {
    local slug="$1" name="$2" provider_type="$3" client_id="$4" client_secret="$5"

    if [[ -z "$client_id" || -z "$client_secret" ]]; then
        echo "  – $name: skipped (missing credentials)"
        return
    fi

    local payload
    payload=$(jq -n \
        --arg name "$name" \
        --arg slug "$slug" \
        --arg pt "$provider_type" \
        --arg key "$client_id" \
        --arg secret "$client_secret" \
        --arg af "${AUTH_FLOW:-}" \
        --arg ef "${ENROLL_FLOW:-}" \
        '{name: $name, slug: $slug, enabled: true, provider_type: $pt,
          consumer_key: $key, consumer_secret: $secret} +
         (if $af != "" then {authentication_flow: $af} else {} end) +
         (if $ef != "" then {enrollment_flow: $ef} else {} end)')

    local http_code
    http_code=$(curl -s -o /dev/null -w '%{http_code}' \
        -X PATCH "$API/sources/oauth/$slug/" \
        -H "Authorization: Bearer $TOKEN" \
        -H "Content-Type: application/json" \
        -d "$payload")

    if [[ "$http_code" == "200" ]]; then
        echo "  ✓ $name updated"
    else
        http_code=$(curl -s -o /dev/null -w '%{http_code}' \
            -X POST "$API/sources/oauth/" \
            -H "Authorization: Bearer $TOKEN" \
            -H "Content-Type: application/json" \
            -d "$payload")
        if [[ "$http_code" == "201" ]]; then
            echo "  ✓ $name created"
        else
            echo "  ✗ $name failed (HTTP $http_code)"
        fi
    fi
}

update_source "github" "GitHub" "github" "$GITHUB_ID" "$GITHUB_SECRET"
update_source "google" "Google" "google" "$GOOGLE_ID" "$GOOGLE_SECRET"

echo ""
echo "Also make sure these callback URLs are registered in your OAuth apps:"
echo "  GitHub: https://auth.caboose-ai.io/source/oauth/callback/github/"
echo "  Google: https://auth.caboose-ai.io/source/oauth/callback/google/"
echo ""
echo "Done. Test at: https://auth.caboose-ai.io/"
