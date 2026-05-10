# Smoke Test Guidelines

## Scope

These instructions apply to `internal/smoketest/`.

## Patterns

- Keep live tests behind the `integration` build tag.
- Use config/API checks for Authentik provider, application, source, outpost,
  and endpoint contracts before relying on browser flows.
- Add browser flows only for real native login, first-run, handoff, or
  proxy-gated behavior that must be proven end to end.
- `smoke_flow` values in service manifests should resolve to executable flow
  names here. Leave health-only infrastructure services without a flow.
- Redact passwords, bootstrap tokens, OAuth secrets, and Turnstile secrets from
  logs and evidence.
- Treat Turnstile swaps, bootstrap token recovery, and browser evidence writes
  as live-test side effects.

## Good And Bad Examples

Good:

```go
flow, ok := SmokeFlowByName(name)
if !ok {
    t.Fatalf("unknown smoke flow %q", name)
}
runBrowserFlow(t, flow)
```

Bad:

```go
// Every service just runs the full config suite, regardless of smoke_flow.
go test -tags integration ./internal/smoketest -run TestSSO_Config
```

Good:

```go
evidence.RecordInput("authentik-password", "<redacted>")
```

Bad:

```go
t.Logf("using token %s and password %s", token, password)
```

## Validation

- `go test ./internal/smoketest`
- `go test -tags integration ./internal/smoketest/ -run TestSSO_Config -v`
- For one targeted flow: `go test -tags integration ./internal/smoketest/ -run TestSSO_ServiceSmoke -v -args -smoke-flow <name>`
