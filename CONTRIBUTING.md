# Contributing

Thanks for helping improve agenthop!

## Development

```bash
make build
make test
./bin/agenthop providers doctor
```

## Adding a provider

See [docs/adding-a-provider.md](docs/adding-a-provider.md).

## Pull requests

- Keep changes focused
- Add tests for parsers and migration logic
- Run `go test ./...` before submitting

## Reporting issues

Include:

- `agenthop providers doctor` output
- Source and target provider IDs
- Whether `agenthop migrate --dry-run` reproduces the issue
