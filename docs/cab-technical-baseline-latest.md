# CAB Technical Baseline Snapshot (Latest)

Generated: 2026-05-18T03:03:00Z (UTC)
Generator: `dev/generate-technical-baseline-report.sh`

## Repository Metrics

- Go version from `go.mod`: `1.26.3`
- CI `setup-go` versions detected: `go.mod (1.26.3)`
- CLI/MCP entry points (`cmd/*/main.go`): 4
- Service manifests (`services/*/service.yaml`): 18
- Go test files (`*_test.go`): 68
- Markdown docs in `docs/`: 21
- CI workflows (`.github/workflows/*.yml`): 8

## Current Validation Gates (Declared)

- `go test ./...`
- `go build ./...`
- `mise run lint`
- `mise run vulncheck`
- `mise run sso:check-quick` (environment-dependent)

## Freshness Contract

- Owner: Architect (CAB-3)
- Update cadence: refresh on CAB technical-program heartbeat and before plan confirmations.
- Stale threshold: 7 days. If this file is older, regenerate before using it for planning decisions.
