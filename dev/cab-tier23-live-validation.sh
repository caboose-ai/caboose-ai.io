#!/usr/bin/env bash
set -euo pipefail

changed_services="${1:-}"
out_dir="${2:-docs/evidence/cab-8}"
ts="$(date -u +%Y%m%dT%H%M%SZ)"
mkdir -p "$out_dir"
log="$out_dir/tier23-live-$ts.log"
summary="$out_dir/tier23-live-$ts-summary.md"

run() {
  local cmd="$1"
  echo "\$ $cmd" | tee -a "$log"
  set +e
  bash -lc "$cmd" 2>&1 | tee -a "$log"
  local rc=${PIPESTATUS[0]}
  set -e
  if [ $rc -eq 0 ]; then
    echo "PASS|$cmd" >> "$status_file"
  else
    echo "FAIL|$cmd" >> "$status_file"
  fi
}

status_file="$(mktemp)"
trap 'rm -f "$status_file"' EXIT

run "mise run sso:check-quick"

if [ -n "$changed_services" ]; then
  IFS=',' read -r -a services <<< "$changed_services"
  for svc in "${services[@]}"; do
    svc="$(echo "$svc" | xargs)"
    [ -n "$svc" ] || continue
    run "mise run service:smoke -- $svc"
  done
else
  echo "SKIP|tier3-services|no changed services provided" >> "$status_file"
fi

run "mise run paperclip:smoke"

{
  echo "## CAB-8 Tier 2/3 Live Validation Evidence"
  echo "- Timestamp (UTC): $(date -u +%Y-%m-%dT%H:%M:%SZ)"
  echo "- Changed services input: ${changed_services:-<none>}"
  echo "- Raw log: \`$log\`"
  echo
  echo "### Results"
  while IFS='|' read -r result command rest; do
    if [ "$result" = "SKIP" ]; then
      echo "- $command: SKIP (${rest:-})"
    else
      echo "- $command: $result"
    fi
  done < "$status_file"
} > "$summary"

if grep -q '^FAIL|' "$status_file"; then
  echo "One or more checks failed. See: $summary"
  exit 1
fi

echo "Evidence summary: $summary"
echo "Evidence log: $log"
