#!/usr/bin/env bash
# Usage: ./run_tests.sh [--unit | --integration | --all]
set -euo pipefail

TESTS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MODE="${1:---all}"

run_unit() {
  echo "==> Running unit tests..."
  bats "$TESTS_DIR/unit/"
}

run_integration() {
  echo "==> Running integration tests..."
  bats "$TESTS_DIR/integration/"
}

case "$MODE" in
  --unit)        run_unit ;;
  --integration) run_integration ;;
  --all)         run_unit; run_integration ;;
  *) echo "Usage: $0 [--unit | --integration | --all]"; exit 1 ;;
esac
