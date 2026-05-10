# Authentik Service Guidelines

## Scope

These instructions apply to `services/authentik/`.

## Patterns

- Keep one Authentik resource type per file where practical.
- Define request and response structs instead of ad hoc maps for API payloads.
- Use the existing `Client` helpers for HTTP verbs, auth headers, and response
  error handling.
- Exact-match slugs after search responses; Authentik search can return broader
  matches.
- Installer-created or repaired applications should use
  `policy_engine_mode=all` so provider-level SSO policy controls access.
- Shared OAuth source upserts should not change `promoted` unless the caller
  explicitly owns promotion or demotion.

## Good And Bad Examples

Good:

```go
var out []Application
if err := c.Get(ctx, "/api/v3/core/applications/?search="+url.QueryEscape(slug), &out); err != nil {
    return nil, err
}
for _, app := range out {
    if app.Slug == slug {
        return &app, nil
    }
}
```

Bad:

```go
// First search result might be a different application.
return results[0], nil
```

Good:

```go
type OAuth2ProviderRequest struct {
    Name             string `json:"name"`
    ClientID         string `json:"client_id"`
    PolicyEngineMode string `json:"policy_engine_mode"`
}
```

Bad:

```go
payload := map[string]any{"name": name, "client_id": id, "secret": secret}
log.Printf("provider payload: %#v", payload)
```

## Validation

- `go test ./services/authentik -v`
- `go test ./internal/install -run 'Provider|Application|Flow|Social|Outpost' -v`
- `mise run sso:check-quick` after live Authentik provisioning changes.
