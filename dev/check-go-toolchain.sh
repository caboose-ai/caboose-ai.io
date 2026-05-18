#!/usr/bin/env bash
set -euo pipefail

if command -v go >/dev/null 2>&1; then
  GO_BIN="$(command -v go)"
elif [ -x /home/caboose/.local/go/bin/go ]; then
  GO_BIN="/home/caboose/.local/go/bin/go"
else
  echo "go executable not found in PATH or /home/caboose/.local/go/bin/go" >&2
  exit 1
fi

expected="$(awk '$1 == "go" { print $2; exit }' go.mod)"
if [ -z "$expected" ]; then
  echo "failed to read Go version from go.mod" >&2
  exit 1
fi

mise_go="$(awk -F'"' '$1 ~ /^[[:space:]]*go[[:space:]]*=[[:space:]]*$/ { print $2; exit }' mise.toml)"
if [ -z "$mise_go" ]; then
  echo "failed to read go tool version from mise.toml" >&2
  exit 1
fi

if [ "$mise_go" != "$expected" ]; then
  echo "Go toolchain contract mismatch: go.mod=$expected but mise.toml=$mise_go" >&2
  exit 1
fi

installed_raw="$($GO_BIN version)"
installed="$(printf '%s\n' "$installed_raw" | awk '{print $3}' | sed 's/^go//')"

if [ "$installed" != "$expected" ]; then
  echo "Go toolchain mismatch: expected $expected from go.mod, got $installed ($installed_raw)" >&2
  exit 1
fi

echo "Go toolchain OK: $installed"
