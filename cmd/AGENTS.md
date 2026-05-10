# Command Entrypoint Guidelines

## Scope

These instructions apply to `cmd/`.

## Patterns

- Keep binary entrypoints thin. Parse flags and wire dependencies here, then
  delegate behavior to `internal/cli`, `internal/install`, `internal/mcp`, or
  another internal package.
- Return actionable errors from internal packages and let the command layer own
  process exit, display, and usage text.
- Reuse existing config loading, runner, Docker, and service registry helpers.
- Keep command defaults aligned with `mise.toml` and README examples.

## Good And Bad Examples

Good:

```go
root := cli.NewServiceCommand(cli.ServiceCommandOptions{
    Domain:     domain,
    ComposeDir: composeDir,
})
return root.Execute(ctx, args)
```

Bad:

```go
cmd := exec.Command("docker", "compose", "restart", "forgejo")
cmd.Stdout = os.Stdout
return cmd.Run()
```

Good:

```go
if err := run(ctx); err != nil {
    fmt.Fprintln(os.Stderr, err)
    os.Exit(1)
}
```

Bad:

```go
log.Printf("failed but continuing: %v", err)
return nil
```

## Validation

- `go test ./cmd/... ./internal/cli ./internal/mcp`
- `go run ./cmd/homelab service --help`
- `go run ./cmd/homelab oauth-setup --domain caboose-ai.io`
