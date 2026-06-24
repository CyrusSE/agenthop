# Architecture

```
cmd/agenthop          CLI entry
internal/cli          cobra commands
internal/tui          bubbletea UI
internal/index          SQLite session index (~/.cache/agenthop/index.db)
internal/registry       provider registry
internal/provider       Provider interface
internal/providers/*    per-agent read/write
internal/model          UnifiedConversation
internal/migrate        migration engine + dedup
```

## Data flow

1. **Discover** — each provider scans its storage paths and returns lightweight `Summary` records.
2. **Index** — summaries stored in SQLite; incremental updates compare source file mtimes.
3. **Load** — full `Conversation` parsed from JSONL or SQLite on demand.
4. **Write** — target provider serializes unified model to its native format.
5. **Dedup** — SHA-256 digest of message stream stored in `agenthop_migration` metadata.

## Index schema

| Column | Purpose |
|--------|---------|
| id, provider | composite primary key |
| project_path, title | display + filter |
| updated_at, message_count | sorting |
| storage_path, source_mtime | load + incremental refresh |

## Extension points

- New provider: implement `provider.Provider`, register in `internal/registry/registry.go`
- Custom paths: env vars per provider (`CODEX_HOME`, `CLAUDE_CONFIG_DIR`, etc.)
- Future: `~/.config/agenthop/config.yaml` for path overrides
