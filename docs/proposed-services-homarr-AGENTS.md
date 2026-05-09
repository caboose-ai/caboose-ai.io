# Repository Guidelines

## Scope

These instructions apply to `services/homarr/`.

## Project Map

- `homarr.go`: Homarr OIDC configurator, onboarding automation, default board seeding, app icon selection, app slugging, and Docker-backed SQLite updates.
- `homarr_test.go`: Unit tests for secret writes, first-run initialization, default board seeding, app filtering behavior, and generated seed data.
- `service.yaml`: Service manifest for CLI, MCP, docs, compose, secrets, smoke, and health surfaces.
- `README.md`: Service-specific operational notes.

## Build, Test, and Validation

- `go test ./services/homarr -v`: Run Homarr configurator and seed tests.
- `go test ./internal/servicebuilder ./services/homarr -v`: Validate dashboard app selection plus Homarr seed behavior together.
- `docker exec homarr node -e '<query>'`: Use only for live diagnosis or verified live repair. Keep scripts small and avoid printing secrets.
- `mise run sso:check-quick`: Use after Authentik/Homarr OIDC changes when live Authentik config must be verified.

## Coding Conventions

- Keep Homarr setup idempotent. `Configure` should be safe to run repeatedly and should repair managed board state when credentials are already configured.
- Treat app IDs prefixed with `homelab_app_` and item IDs prefixed with `homelab_item_` as installer-owned dashboard records.
- Keep dashboard app curation in `internal/servicebuilder` and seed rendering in `services/homarr`; do not hardcode duplicate app lists in CLI or smoke tests.
- Preserve the Open WebUI dashboard URL override to `/oauth/oidc/login` unless a test proves the root URL no longer emits unauthenticated noise.
- Add tests for new app filtering, slugging, icon mapping, or seed layout changes.
- Prefer structured JSON and prepared SQLite statements in the embedded Node script. Avoid ad hoc shell interpolation.

## Security and Data Handling

- Do not print Homarr OIDC client secrets, `.env` values, or Authentik tokens.
- The Docker exec seed path writes directly to `/appdata/db/db.sqlite`; treat it as live state mutation.
- Avoid deleting non-homelab-managed dashboard rows. Cleanup should stay scoped to `homelab_app_%` and `homelab_item_%` records unless the user explicitly requests broader cleanup.

## Agent Workflow

- Before changing dashboard contents, inspect `internal/servicebuilder/builder.go` and `services/homarr/homarr_test.go` together.
- For live dashboard fixes, make the durable code/test change first, then apply the smallest direct SQLite repair needed to bring the running Homarr instance into parity.
- After live repair, verify both the DB rows and the public dashboard behavior when feasible.
- If Homarr schema assumptions change, update tests before relying on a live container query.
