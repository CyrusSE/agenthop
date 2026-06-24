# Adding a provider

1. Create `internal/providers/<name>/<name>.go`
2. Implement `provider.Provider`:

```go
type Provider interface {
    ID() string
    DisplayName() string
    DefaultPaths() []PathSpec
    Installed() bool
    Discover(ctx, opts) ([]model.Summary, error)
    Load(ctx, ref) (*model.Conversation, error)
    Write(ctx, conv, opts) (*WriteResult, error)
    SupportsResume() bool
    ResumeCommand(result WriteResult) string
}
```

3. Register in `internal/registry/registry.go`:

```go
myprovider.New(),
```

4. Add tests in `testdata/<name>/` with golden fixtures
5. Document storage format in `docs/providers/<name>.md`

## Checklist

- [ ] `Discover` is lightweight (title + counts, not full parse)
- [ ] `Load` handles user + assistant messages
- [ ] `Write` embeds `agenthop_migration` metadata for dedup
- [ ] `ResumeCommand` returns a working CLI command
- [ ] `Installed()` checks real storage paths
- [ ] Run `agenthop providers doctor` to verify

## Stub providers

For agents not yet implemented, use `provider.NewStub()` — shows in `providers` list with `stub` status and links to CONTRIBUTING.
