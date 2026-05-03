# Implementation Plan: TUI Status, Captcha & Least Privilege

**Spec:** `docs/superpowers/specs/2026-05-03-tui-status-captcha-leastpriv-design.md`

## Task 1: Add Authentik API methods for stages, bindings, and recovery

**New files:**
- `internal/services/authentik/stages.go`
- `internal/services/authentik/bindings.go`
- `internal/services/authentik/recovery.go`

**`stages.go`** — Captcha stage and user-write stage methods:

```go
// CaptchaStage represents an Authentik captcha stage.
type CaptchaStage struct {
    PK   string `json:"pk"`
    Name string `json:"name"`
}

// GetCaptchaStage looks up a captcha stage by name. Returns nil if not found.
func (c *Client) GetCaptchaStage(ctx, name) (*CaptchaStage, error)
// GET /api/v3/stages/captcha/?name=<name>

// CreateCaptchaStageParams holds parameters for creating a captcha stage.
type CreateCaptchaStageParams struct {
    Name       string `json:"name"`
    PublicKey  string `json:"public_key"`
    PrivateKey string `json:"private_key"`
    JsURL      string `json:"js_url"`
    ApiURL     string `json:"api_url"`
}

// CreateCaptchaStage creates a new captcha stage in Authentik.
func (c *Client) CreateCaptchaStage(ctx, params) (*CaptchaStage, error)
// POST /api/v3/stages/captcha/

// UserWriteStage represents an Authentik user-write stage.
type UserWriteStage struct {
    PK                     string `json:"pk"`
    Name                   string `json:"name"`
    CreateUsersAsInactive  bool   `json:"create_users_as_inactive"`
}

// GetUserWriteStage retrieves a user-write stage by PK.
func (c *Client) GetUserWriteStage(ctx, pk) (*UserWriteStage, error)
// GET /api/v3/stages/user_write/<pk>/

// PatchUserWriteStage updates a user-write stage.
func (c *Client) PatchUserWriteStage(ctx, pk, inactive bool) error
// PATCH /api/v3/stages/user_write/<pk>/ with {"create_users_as_inactive": true}
```

**`bindings.go`** — Flow stage binding methods:

```go
// FlowStageBinding represents a binding between a flow and a stage.
type FlowStageBinding struct {
    PK    string `json:"pk"`
    Stage struct {
        PK        string `json:"pk"`
        Name      string `json:"name"`
        Component string `json:"component"`       // e.g. "ak-stage-user-write-form"
    } `json:"stage_obj"`
    Order int `json:"order"`
}

// ListFlowStageBindings returns all stage bindings for a flow.
func (c *Client) ListFlowStageBindings(ctx, flowSlug) ([]FlowStageBinding, error)
// GET /api/v3/flows/bindings/?flow_slug=<slug>&page_size=50

// GetFlowStageBinding checks if a specific stage is already bound to a flow.
func (c *Client) GetFlowStageBinding(ctx, flowSlug, stagePK) (*FlowStageBinding, error)
// GET /api/v3/flows/bindings/?flow_slug=<slug>&stage=<stagePK>

// CreateFlowStageBinding binds a stage to a flow at a given order.
func (c *Client) CreateFlowStageBinding(ctx, flowPK, stagePK string, order int) error
// POST /api/v3/flows/bindings/ with {target: flowPK, stage: stagePK, order: order}
```

**`recovery.go`** — Recovery link generation:

```go
// GenerateRecoveryLink creates a one-time recovery link for a user.
// Returns the full URL (e.g., "/if/flow/default-recovery-flow/?token=abc123").
func (c *Client) GenerateRecoveryLink(ctx, userPK int) (string, error)
// POST /api/v3/core/users/<pk>/recovery/
// Response body: {"link": "https://..."}
```

**Tests:** Table-driven tests with mock HTTP client for each method. Test both success and not-found cases.

**Acceptance criteria:**
- All 7 methods implemented following existing client patterns (Get/Post/Patch, JSON unmarshal, error handling)
- Tests pass with `go test ./internal/services/authentik/ -v`

---

## Task 2: Add Turnstile secret definitions and config fields

**Modified files:**
- `internal/secrets/store.go`
- `internal/config/config.go`

**`store.go`** — Add Turnstile secrets to a new `TurnstileSecrets()` function:

```go
func TurnstileSecrets() []SecretDef {
    return []SecretDef{
        {Key: "TURNSTILE_SITE_KEY", Prompt: true, Optional: true},
        {Key: "TURNSTILE_SECRET_KEY", Prompt: true, Optional: true},
    }
}
```

Also add to `DerivedSecretKeys()`:
- `"TURNSTILE_SITE_KEY"`
- `"TURNSTILE_SECRET_KEY"`

**`config.go`** — Add Turnstile config fields:

```go
type TurnstileConfig struct {
    SiteKey   string `yaml:"site_key"`
    SecretKey string `yaml:"secret_key"`
}
```

Add to `Config` struct:
```go
Turnstile TurnstileConfig `yaml:"turnstile"`
```

**Tests:** None needed (data definitions only).

**Acceptance criteria:**
- `TurnstileSecrets()` returns 2 optional prompted secrets
- `Config.Turnstile` can be populated from YAML
- `go build ./...` passes

---

## Task 3: Add Turnstile credential resolution to installer

**New file:**
- `internal/install/turnstile.go`

**Pattern:** Follow exactly the same pattern as `social.go` — config-or-secrets-or-prompt.

```go
func (inst *Installer) ResolveTurnstileCredentials(ctx context.Context, promptFn func(key string) (string, error)) error
```

Logic:
1. Check `inst.Config.Turnstile.SiteKey` and `inst.Config.Turnstile.SecretKey`
2. If both present in config, store to secrets store and return
3. If missing, try `inst.Secrets.Get(ctx, key)`
4. If still missing and promptFn != nil, call promptFn
5. If either key ends up empty, skip (both are optional)
6. If both obtained, store to config and secrets store

**Tests:** Table-driven tests mocking SecretStore, testing:
- Config provides both → stored to secrets
- Secrets store provides both → stored to config
- PromptFn provides both → stored to both
- One missing → skipped (no error)
- Both missing → skipped (no error)

**Acceptance criteria:**
- `go test ./internal/install/ -v -run TestResolveTurnstile` passes
- Follows same code pattern as `ResolveSocialCredentials`

---

## Task 4: Add captcha stage and inactive enrollment to installer

**New file:**
- `internal/install/captcha.go`

Two functions:

```go
// SetupCaptcha creates a Turnstile captcha stage and binds it to the
// default authentication flow. Skipped if Turnstile credentials are not
// available. progressFn reports milestone status.
func (inst *Installer) SetupCaptcha(ctx context.Context, progressFn func(string)) error
```

Logic:
1. Get Turnstile creds from secrets store. If either missing, return nil (skip).
2. `progressFn("Creating Turnstile captcha stage...")`
3. `GetCaptchaStage(ctx, "turnstile-captcha")` — if exists, skip creation
4. `CreateCaptchaStage(ctx, params)` with Turnstile JS/API URLs
5. `progressFn("Binding captcha to authentication flow...")`
6. `GetFlow(ctx, "default-authentication-flow")`
7. `GetFlowStageBinding(ctx, flowSlug, stagePK)` — if exists, skip
8. `CreateFlowStageBinding(ctx, flowPK, stagePK, 0)` — order 0 = first in flow

```go
// SetupInactiveEnrollment patches the user-write stage in the default
// source enrollment flow to create users as inactive. progressFn reports
// milestone status.
func (inst *Installer) SetupInactiveEnrollment(ctx context.Context, progressFn func(string)) error
```

Logic:
1. `progressFn("Configuring enrollment flow for inactive users...")`
2. `ListFlowStageBindings(ctx, "default-source-enrollment")`
3. Find binding where `stage_obj.component` contains "user-write" or stage name contains "user-write"
4. `PatchUserWriteStage(ctx, stagePK, true)`

Both functions are idempotent (safe to run on `--force` re-runs).

**Tests:** Table-driven tests mocking AK client methods. Test:
- SetupCaptcha: no creds → skip, creds present + stage exists → skip creation, creds present + new → creates and binds
- SetupInactiveEnrollment: finds user-write stage → patches it, no user-write stage → returns error

**Acceptance criteria:**
- `go test ./internal/install/ -v -run TestSetupCaptcha` passes
- `go test ./internal/install/ -v -run TestSetupInactiveEnrollment` passes
- Both functions accept `progressFn func(string)` for TUI integration

---

## Task 5: Add recovery link generation and admin credentials to installer

**New file:**
- `internal/install/admin.go`

```go
// GenerateAdminRecoveryLink generates a one-time recovery link for the
// auth-admin user. Returns the full URL. progressFn reports milestone status.
func (inst *Installer) GenerateAdminRecoveryLink(ctx context.Context, progressFn func(string)) (string, error)
```

Logic:
1. `progressFn("Generating admin recovery link...")`
2. `AK.FindUser(ctx, "auth-admin")` — get user PK
3. `AK.GenerateRecoveryLink(ctx, userPK)` — get recovery URL
4. Return the link

**Modified file:**
- `internal/install/state.go` — Add fields to State:

```go
AdminRecoveryLink string
AdminPassword     string
```

**Tests:** Table-driven tests mocking AK client. Test:
- User found → returns recovery link
- User not found → returns error

**Acceptance criteria:**
- `go test ./internal/install/ -v -run TestGenerateAdminRecoveryLink` passes
- State struct has `AdminRecoveryLink` and `AdminPassword` fields

---

## Task 6: Wire progress messages through TUI

**Modified files:**
- `internal/tui/app.go` — Main changes

Add new message type:
```go
type progressMsg string
```

Update `View()` to render `progressMsg` as a single line below the stepper when in phases 1-2.

Add a `currentProgress string` field to `AppModel`.

Update these command functions to emit `progressMsg` via callbacks:

### `runComposeAndHealth()`:
Emit progress messages for each health status received:
- Before ComposeUp: `progressMsg("Starting containers...")`
- For each health status: `progressMsg("Waiting for <name> health...")`

### `runInitAndRename()`:
Expand to include captcha, enrollment, and recovery link:
1. `progressMsg("Renaming admin user to auth-admin...")`
2. Init AK, rename admin
3. `progressMsg("Generating admin recovery link...")`
4. Call `inst.GenerateAdminRecoveryLink(ctx, ...)`
5. Store link and password in `inst.State`
6. Call `inst.SetupCaptcha(ctx, ...)` (has own progress messages)
7. Call `inst.SetupInactiveEnrollment(ctx, ...)` (has own progress messages)
8. Init Forgejo
9. Return `akReadyMsg{}`

### `runProvisionProviders()`:
Wire the existing `progressFn` to emit `progressMsg`:
- "Creating <name> OAuth provider..." or "<name> provider exists"

### `runProvisionOutpost()`:
Wire the existing `progressFn` to emit `progressMsg`:
- "Creating <name> proxy provider..." or "<name> proxy exists"
- "Binding providers to embedded outpost..."

**Challenge:** Bubbletea commands run in goroutines and can only return one message. To emit *multiple* progress messages from a single command, we need to use `tea.Program.Send()` or restructure as a sequence of commands.

**Approach:** Use `p.Send(progressMsg(...))` pattern. Since the TUI already has access to the program via `tea.NewProgram`, we can pass a `progressFn` that calls `p.Send()`. However, the cleaner approach for this codebase is to break the long-running commands into smaller steps that each return a message, chaining them via the Update loop.

**Alternative (simpler):** Since milestone messages don't need to update the screen in real-time *during* a single goroutine's execution (the goroutine blocks the message loop anyway), we can instead:
1. Emit a `progressMsg` before starting each sub-operation
2. Chain operations as a sequence of messages (progressMsg → start op → result msg → progressMsg → start next op → ...)

This is more complex but correct for Bubbletea's architecture. The simplest pragmatic approach: add `progressMsg` handling to Update, and have the long-running commands use `tea.Sequence()` or break into multiple smaller commands chained by intermediate messages.

**Recommended approach:** Add intermediate messages for the init phase:

```go
type initPhaseMsg int
const (
    initPhaseRenameAdmin initPhaseMsg = iota
    initPhaseRecoveryLink
    initPhaseCaptcha
    initPhaseInactiveEnrollment
    initPhaseForgejo
    initPhaseDone
)
```

Each phase handler emits a `progressMsg` and starts the async work. When the work completes, it returns the next `initPhaseMsg`. This keeps the TUI responsive and shows progress between each step.

**Acceptance criteria:**
- Milestone messages appear below the stepper during phases 1-2
- All init sub-phases show progress messages
- Provider and outpost provisioning show per-item progress
- `go build ./...` passes

---

## Task 7: Update summary view with admin credentials

**Modified file:**
- `internal/tui/views/summary.go`

Update `NewSummary` to accept additional params:

```go
type SummaryParams struct {
    Results       []install.ServiceResult
    Domain        string
    AdminPassword string
    RecoveryLink  string
}

func NewSummary(params SummaryParams) SummaryModel
```

Update `View()` to render credentials block between service results and URLs:

```
──────────────────────────────────────
  Auth Admin Credentials
──────────────────────────────────────
  Username:  auth-admin
  Password:  <password>

  One-time setup link (set up TOTP):
    <recovery link>

  Stored in 1Password:
    op read "op://Homelab/AUTHENTIK_BOOTSTRAP_PASSWORD/password"
──────────────────────────────────────
```

Update `app.go` call sites that create `NewSummary` to pass the new params from `inst.State`.

**Acceptance criteria:**
- Summary shows credentials block with password and recovery link
- Summary still shows service results and URLs
- `go build ./...` passes

---

## Task 8: Wire Turnstile prompts into TUI secrets phase

**Modified files:**
- `internal/tui/app.go`

Add `turnstileDefs []secrets.SecretDef` field to `AppModel`, initialized from `secrets.TurnstileSecrets()`.

Update `continueSecretsPhase()` to iterate through `turnstileDefs` after `socialDefs` (same pattern — prompt if not in config or promptValues).

Update `runResolveSocialCredentials()` to also call `inst.ResolveTurnstileCredentials()` with the same promptFn pattern.

Alternatively: resolve Turnstile creds in a new `runResolveTurnstileCredentials()` that chains after `socialReadyMsg` via a new `turnstileReadyMsg`, which then triggers compose. This keeps the flow clean.

**Recommended:** Add a `turnstileReadyMsg` and resolve Turnstile creds between social resolution and compose start:

```
SecretsCompleteMsg → runResolveSocialCredentials → socialReadyMsg
→ runResolveTurnstileCredentials → turnstileReadyMsg → runComposeAndHealth
```

**Acceptance criteria:**
- Turnstile site key and secret key are prompted during secrets phase (after social creds)
- If config provides them, they're not prompted
- If both are skipped (empty), install proceeds without captcha
- `go build ./...` passes

---

## Task 9: Integration test — full build and manual verification

Run:
```bash
go build ./...
go test ./internal/... -v
```

Verify:
- All tests pass
- Binary builds cleanly
- No lint issues

**Acceptance criteria:**
- `go build ./...` exits 0
- `go test ./internal/... -v` all pass
- No compilation errors or warnings

---

## Dependency Order

```
Task 1 (Authentik API methods)
  ↓
Task 2 (Secret definitions + config) ── can run in parallel with Task 1
  ↓
Task 3 (Turnstile credential resolution) ── depends on Task 2
  ↓
Task 4 (Captcha + inactive enrollment) ── depends on Tasks 1, 3
  ↓
Task 5 (Recovery link + admin creds) ── depends on Task 1
  ↓
Task 6 (TUI progress wiring) ── depends on Tasks 4, 5
  ↓
Task 7 (Summary view) ── depends on Task 5
  ↓
Task 8 (TUI Turnstile prompts) ── depends on Tasks 2, 3, 6
  ↓
Task 9 (Integration test) ── depends on all
```
