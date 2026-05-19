#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
out_file="${1:-$repo_root/docs/cab-technical-baseline-latest.md}"
out_json="${2:-$repo_root/docs/cab-technical-baseline-latest.json}"
generated_at="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"

entry_points=$(find "$repo_root/cmd" -type f -name main.go | wc -l | tr -d ' ')
service_manifests=$(find "$repo_root/services" -type f -name service.yaml | wc -l | tr -d ' ')
go_test_files=$(find "$repo_root" -type f -name '*_test.go' | wc -l | tr -d ' ')
docs_files=$(find "$repo_root/docs" -maxdepth 1 -type f -name '*.md' | wc -l | tr -d ' ')
workflows=$(find "$repo_root/.github/workflows" -type f -name '*.yml' | wc -l | tr -d ' ')
workflow_names=$(
  find "$repo_root/.github/workflows" -type f -name '*.yml' -printf '%f\n' \
    | sed 's/\.yml$//' \
    | sort -u \
    | awk 'BEGIN{ORS=""; first=1} {if (!first) printf ", "; printf "%s", $0; first=0}'
)

go_version="unknown"
if [ -f "$repo_root/go.mod" ]; then
  go_version=$(awk '/^go / {print $2; exit}' "$repo_root/go.mod")
fi

ci_go_versions="none"
if [ -d "$repo_root/.github/workflows" ]; then
  ci_go_versions=$(
    {
      (rg -n "go-version:" "$repo_root/.github/workflows" -g '*.yml' || true) \
        | sed -E 's/.*go-version:[[:space:]]*//' \
        | tr -d '"'
      (rg -n "go-version-file:" "$repo_root/.github/workflows" -g '*.yml' || true) \
        | sed -E 's/.*go-version-file:[[:space:]]*//' \
        | tr -d '"' \
        | while read -r version_file; do
          version_file="${version_file#"${version_file%%[![:space:]]*}"}"
          version_file="${version_file%"${version_file##*[![:space:]]}"}"
          if [ -f "$repo_root/$version_file" ]; then
            version="$(awk '/^go / {print $2; exit}' "$repo_root/$version_file")"
            printf '%s (%s)\n' "$version_file" "${version:-unknown}"
          else
            printf '%s (missing)\n' "$version_file"
          fi
        done
    } | awk 'NF && !seen[$0]++ { values[++n] = $0 } END { if (n == 0) print "none"; for (i = 1; i <= n; i++) printf "%s%s", (i > 1 ? ", " : ""), values[i]; if (n > 0) print "" }'
  )
  ci_go_versions=${ci_go_versions:-none}
fi

cat > "$out_file" <<REPORT
# CAB Technical Baseline Snapshot (Latest)

Generated: $generated_at (UTC)
Generator: \`dev/generate-technical-baseline-report.sh\`

## Repository Metrics

- Go version from \`go.mod\`: \`$go_version\`
- CI \`setup-go\` versions detected: \`$ci_go_versions\`
- CLI/MCP entry points (\`cmd/*/main.go\`): $entry_points
- Service manifests (\`services/*/service.yaml\`): $service_manifests
- Go test files (\`*_test.go\`): $go_test_files
- Markdown docs in \`docs/\`: $docs_files
- CI workflows (\`.github/workflows/*.yml\`): $workflows

## Current Validation Gates (Declared)

- \`go test ./...\`
- \`go build ./...\`
- \`mise run lint\`
- \`mise run vulncheck\`
- \`mise run sso:check-quick\` (environment-dependent)

## Freshness Contract

- Owner: Architect (CAB-3)
- Update cadence: refresh on CAB technical-program heartbeat and before plan confirmations.
- Stale threshold: 7 days. If this file is older, regenerate before using it for planning decisions.
REPORT

echo "wrote $out_file"

cat > "$out_json" <<JSON
{
  "generated_at": "$generated_at",
  "generator": "dev/generate-technical-baseline-report.sh",
  "snapshot": {
    "go_version": "$go_version",
    "ci_go_versions": "$ci_go_versions",
    "cmd_main_entrypoints": $entry_points,
    "service_manifests": $service_manifests,
    "go_test_files": $go_test_files,
    "docs_markdown_files": $docs_files,
    "ci_workflows": $workflows,
    "workflow_names": "$workflow_names"
  },
  "validation_gates": [
    "go test ./...",
    "go build ./...",
    "mise run lint",
    "mise run vulncheck",
    "mise run sso:check-quick"
  ],
  "freshness_contract": {
    "owner": "Architect (CAB-3)",
    "update_cadence": "refresh on CAB technical-program heartbeat and before plan confirmations",
    "stale_threshold_days": 7
  }
}
JSON

echo "wrote $out_json"
