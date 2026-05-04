# TUI Status Updates, Admin Credentials, Captcha & Least Privilege

**Date:** 2026-05-03
**Status:** Approved

## Overview

Four related enhancements to the homelab installer:

1. **TUI milestone status updates** — show what's happening during silent phases
2. **Auth-admin credentials + recovery link** — display password and one-time TOTP setup URL at summary
3. **Cloudflare Turnstile captcha** — gate social sign-ins with a captcha stage
4. **Least privilege for social users** — create social sign-up accounts as inactive until manually activated

## 1. TUI Milestone Status Updates

### Problem

Phases 0 (secrets) and 2 (services) have per-item status lines, but phase 1 is mostly silent: compose health checks, Authentik init, provider provisioning, and outpost setup show only a spinner with no detail.

### Design

Add a single in-place status line to the TUI that updates with milestone messages during the quiet phases. Use the existing `progressFn func(string)` callback pattern already present in `ProvisionProviders` and `ProvisionOutpost`.

| Phase | New Milestones |
|-------|---------------|
| Compose (1) | "Starting containers...", "Waiting for Authentik health...", "Waiting for Forgejo health..." |
| Authentik init | "Renaming admin user to auth-admin...", "Creating Turnstile captcha stage...", "Configuring enrollment flow...", "Generating admin recovery link..." |
| Providers | "Creating forgejo OAuth provider...", "Creating grafana OAuth provider...", etc. |
| Outpost | "Creating dashboard proxy provider...", "Binding providers to embedded outpost..." |

Secrets (phase 0) and services (phase 2) already have granular status — no changes needed there.

### Implementation Approach

- Define a `progressMsg string` Bubbletea message type
- The TUI app model renders it as a single updating line below the stepper
- Install functions emit progress via a `progressFn` that returns `tea.Cmd` sending `progressMsg`
- The existing `progressFn` parameters in `ProvisionProviders` and `ProvisionOutpost` are already wired; extend to compose health, Authentik init, and admin setup phases

## 2. Auth-Admin Credentials & Recovery Link at Summary

### Problem

After install, the user has to dig into 1Password to find the admin password and has no easy way to set up TOTP.

### Design

At install completion, generate a one-time recovery link via `POST /api/v3/core/users/{id}/recovery/` and display it in the summary view alongside the password.

Summary output:

```
──────────────────────────────────────
  Auth Admin Credentials
──────────────────────────────────────
  Username:  auth-admin
  Password:  <32-char password>

  One-time setup link (set up TOTP):
    https://auth.<domain>/if/flow/default-recovery-flow/?token=<token>

  Stored in 1Password:
    op read "op://Homelab/AUTHENTIK_BOOTSTRAP_PASSWORD/password"
──────────────────────────────────────
```

### Implementation Approach

- Add `GenerateRecoveryLink(ctx, userPK)` method to Authentik client — `POST /api/v3/core/users/{id}/recovery/`
- Call it during Authentik init phase (after admin rename, user PK is known)
- Pass recovery link and password through installer state to summary view
- Recovery link is generated on every fresh install (always valid after a `reset`)

### New Authentik API Method

- `GenerateRecoveryLink(ctx context.Context, userPK int) (string, error)` — POST to `/api/v3/core/users/{id}/recovery/`, returns the full recovery URL

## 3. Cloudflare Turnstile Captcha for Social Sign-Ins

### Problem

Anyone with a GitHub or Google account can sign up freely via social login with no friction.

### Design

Insert a Cloudflare Turnstile captcha stage into the default authentication flow so social sign-ins must pass a challenge.

### Secret Definitions

Two new prompted secrets (config-or-prompt, same pattern as social OAuth creds):

- `TURNSTILE_SITE_KEY` — Cloudflare Turnstile site key
- `TURNSTILE_SECRET_KEY` — Cloudflare Turnstile secret key

Both are optional. If neither is provided, captcha setup is skipped (same as social creds behavior).

### Authentik Configuration

During Authentik init phase:

1. Create a `CaptchaStage` via `POST /api/v3/stages/captcha/`:
   - `name`: `"turnstile-captcha"`
   - `public_key`: site key
   - `private_key`: secret key
   - `js_url`: `https://challenges.cloudflare.com/turnstile/v0/api.js`
   - `api_url`: `https://challenges.cloudflare.com/turnstile/v0/siteverify`
2. Bind to `default-authentication-flow` via `POST /api/v3/flows/bindings/`:
   - `target`: flow PK
   - `stage`: captcha stage PK
   - `order`: before the user login stage (early in the flow)

### Scope Note

Binding to `default-authentication-flow` means the captcha fires for **all** logins, not just social. The admin will also see it when logging in with a password. This is intentional — it also prevents brute-force password attacks.

### Idempotency

- `GetCaptchaStage(ctx, "turnstile-captcha")` — skip creation if exists
- `GetFlowStageBinding(ctx, flowPK, stagePK)` — skip binding if exists

### New Authentik API Methods

- `CreateCaptchaStage(ctx, params)` — POST `/api/v3/stages/captcha/`
- `GetCaptchaStage(ctx, name)` — GET `/api/v3/stages/captcha/?name=...`
- `CreateFlowStageBinding(ctx, flowPK, stagePK, order)` — POST `/api/v3/flows/bindings/`
- `GetFlowStageBinding(ctx, flowPK, stagePK)` — GET `/api/v3/flows/bindings/?target=...&stage=...`

## 4. Least Privilege — Inactive Social Sign-Up Accounts

### Problem

Social sign-ups create active users who immediately have default access.

### Design

Patch the user-write stage in Authentik's default source enrollment flow to set `create_users_as_inactive: true`. Social users complete the OAuth + captcha flow but their account is created inactive. They cannot log in until an admin manually activates them.

### Authentik Configuration

During Authentik init phase:

1. Get the `default-source-enrollment` flow
2. List its stage bindings via `GET /api/v3/flows/bindings/?flow_slug=default-source-enrollment`
3. Find the binding whose stage is of type `user_write`
4. Patch that stage via `PATCH /api/v3/stages/user_write/{id}/` with `create_users_as_inactive: true`

### Why This Approach

- Single field on an existing stage — no custom groups, policies, or per-app authorization rules needed
- User simply cannot log in until activated
- The admin (you) is the only email account and uses the bootstrap password, so this doesn't affect admin access
- **Unconditional** — this applies regardless of whether Turnstile creds are provided. Even without captcha, social users are still created inactive

### New Authentik API Methods

- `ListFlowStageBindings(ctx, flowSlug)` — GET `/api/v3/flows/bindings/?flow_slug=...`
- `GetUserWriteStage(ctx, stagePK)` — GET `/api/v3/stages/user_write/{id}/`
- `PatchUserWriteStage(ctx, stagePK, params)` — PATCH with `create_users_as_inactive: true`

## Installer Flow (Updated)

The updated install sequence with all four features integrated:

```
Secrets (phase 0) — unchanged, plus TURNSTILE_SITE_KEY / TURNSTILE_SECRET_KEY prompts
  ↓
Social credential resolution — unchanged
  ↓
Compose up + health checks — now with milestone status messages
  ↓
Authentik init (expanded):
  1. Rename admin user → auth-admin              [milestone]
  2. Generate recovery link for auth-admin        [milestone]
  3. If Turnstile creds: create captcha stage     [milestone]
  4. If Turnstile creds: bind to auth flow        [milestone]
  5. Patch enrollment user-write stage → inactive  [milestone]
  ↓
Provider provisioning — with milestone status messages
  ↓
Outpost provisioning — with milestone status messages
  ↓
Service configuration (phase 2) — unchanged
  ↓
Restart services — unchanged
  ↓
Summary — now includes admin credentials block + recovery link
```

## New Files

- `internal/services/authentik/stages.go` — captcha stage and user-write stage API methods
- `internal/services/authentik/bindings.go` — flow stage binding API methods
- `internal/services/authentik/recovery.go` — recovery link generation

## Modified Files

- `internal/secrets/store.go` — add Turnstile secret definitions
- `internal/install/install.go` — expand Authentik init phase, pass progress callbacks
- `internal/install/social.go` — resolve Turnstile credentials (same pattern as social OAuth)
- `internal/tui/app.go` — handle `progressMsg`, pass it to active view
- `internal/tui/views/summary.go` — render credentials block with password + recovery link
- `internal/install/state.go` — add fields for recovery link and admin password
- `internal/config/config.go` — add Turnstile config fields (if not already supporting arbitrary keys)
