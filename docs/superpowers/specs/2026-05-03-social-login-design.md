# Social Login (GitHub & Google) for Authentik

## Problem

The Authentik SSO gate protects all homelab services, but users can only log in with a local Authentik username/password. Adding GitHub and Google as federation sources lets users "Sign in with GitHub" or "Sign in with Google" on the Authentik login page.

## Scope

Add a credential resolution chain for GitHub and Google OAuth credentials, then feed them into the existing `social.Configurator` which already creates Authentik federation sources via the API.

**Out of scope:** Other social providers (Apple, Microsoft, etc.), custom OIDC federation sources, Authentik enrollment flows for new social users.

## Design

### Credential Resolution Chain

Social login credentials come from external provider dashboards (GitHub Settings, Google Cloud Console) and cannot be auto-generated. The resolution order:

1. **YAML config** — `homelab.yml` has `social.github.client_id` / `social.google.client_id`
2. **1Password** — keys `GITHUB_OAUTH_CLIENT_ID`, `GITHUB_OAUTH_CLIENT_SECRET`, `GOOGLE_OAUTH_CLIENT_ID`, `GOOGLE_OAUTH_CLIENT_SECRET` in the Homelab vault
3. **TUI prompt** — interactive paste during install, stored to 1Password for subsequent runs
4. **Skip** — blank input skips the provider entirely

### Secret Keys

| Key | Source | Required |
|-----|--------|----------|
| `GITHUB_OAUTH_CLIENT_ID` | GitHub OAuth App | Optional |
| `GITHUB_OAUTH_CLIENT_SECRET` | GitHub OAuth App | Optional |
| `GOOGLE_OAUTH_CLIENT_ID` | Google Cloud Console | Optional |
| `GOOGLE_OAUTH_CLIENT_SECRET` | Google Cloud Console | Optional |

### Files to Modify

1. **`internal/secrets/store.go`** — Add `Optional bool` field to `SecretDef`. Add the 4 social secret keys as `Prompt: true, Optional: true`.

2. **`internal/install/install.go`** — Add `ResolveSocialCredentials(ctx, promptFn)` method. Checks config first, then 1Password, then prompts. Populates `inst.Config.Social` with resolved values.

3. **`internal/tui/app.go`** — Wire `ResolveSocialCredentials` into the TUI flow between secrets generation and service configuration. Handle the optional prompts.

4. **`internal/services/social/social.go`** — No changes. Already skips providers with empty credentials and creates Authentik sources via `UpsertSource()`.

5. **`internal/services/authentik/sources.go`** — No changes. `GetSourcePK()` and `UpsertSource()` already handle create/update.

### TUI Flow

After the standard secrets phase, before service configuration:

```
Social Login (optional)
  GitHub Client ID: [paste or Enter to skip]
  GitHub Client Secret: ****
  Google Client ID: [paste or Enter to skip]
  Google Client Secret: ****
```

If the user skips both, the social login step reports "Skipped — no credentials provided" and the installer continues.

### Authentik Callback URLs

Users need these when creating OAuth apps on the provider side:

- **GitHub:** `https://auth.{domain}/source/oauth/callback/github/`
- **Google:** `https://auth.{domain}/source/oauth/callback/google/`

The TUI should display these before prompting for credentials so the user knows what to paste into the provider dashboard.

### Existing Code That Handles the Rest

- `social.Configurator.Configure()` calls `AK.UpsertSource()` for each provider with credentials
- `UpsertSource()` does POST to create or PATCH to update at `/api/v3/sources/oauth/`
- `CheckConfigured()` verifies sources exist in Authentik via `GetSourcePK()`
- The installer already registers the social configurator in `BuildServices()`
