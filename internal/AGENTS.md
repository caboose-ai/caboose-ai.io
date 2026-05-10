# Internal Package Guidelines

## Scope

These instructions apply to `internal/`, except where a nested `AGENTS.md`
provides more specific guidance.

## Patterns

- Keep package boundaries clear: config parses and derives settings, install
  orchestrates setup, service owns manifests/contracts, servicebuilder wires
  configurators, runner wraps side effects, and CLI/MCP adapt those packages.
- External effects must pass through interfaces or small client types. Tests
  should fake `SecretStore`, `CommandRunner`, `HTTPClient`, Docker wrappers, or
  Authentik clients rather than touching live state.
- Avoid package-level mutable state. Thread state through structs, options, or
  contexts.
- Return errors with useful context and let CLI/TUI layers decide how to render
  them.
- Destructive or live-state mutations need explicit user confirmation in the
  command surface and tests for guardrail behavior.

## Good And Bad Examples

Good:

```go
type Checker struct {
    HTTP runner.HTTPClient
}

func (c Checker) Check(ctx context.Context, url string) error {
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
    if err != nil {
        return err
    }
    _, err = c.HTTP.Do(req)
    return err
}
```

Bad:

```go
func Check(url string) error {
    resp, err := http.Get(url)
    if err != nil {
        log.Println(err)
    }
    _ = resp
    return nil
}
```

Good:

```go
if !opts.Yes && !opts.DryRun {
    return fmt.Errorf("reset requires --yes")
}
```

Bad:

```go
_ = os.RemoveAll(composeDir)
_ = secrets.DeleteAll(ctx)
```

## Validation

- `go test ./internal/...`
- For service wiring changes: `go test ./internal/service ./internal/servicebuilder`
- For installer or reset changes: `go test ./internal/install ./internal/cli`
