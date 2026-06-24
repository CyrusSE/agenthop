# agenthop

**Hop AI coding sessions between agents** — list, show, migrate, and resume across Claude Code, Codex, Cursor, OpenCode, CommandCode, Hermes, and more.

Inspired by [ctxmv](https://github.com/Ryu0118/ctxmv), but with:

- **Indexed discovery** — no full-disk scan on every lookup (`~/.cache/agenthop/index.db`)
- **Provider-first TUI** — pick agent → filter sessions → preview → migrate
- **Extensible registry** — add new providers via one package + registry line

## Install

```bash
# From source
git clone https://github.com/CyrusSE/agenthop.git
cd agenthop
make install

# Or direct install (after first release)
go install github.com/CyrusSE/agenthop/cmd/agenthop@latest

# Install script (releases)
curl -fsSL https://raw.githubusercontent.com/CyrusSE/agenthop/main/scripts/install.sh | bash
```

## Quick start

```bash
# Interactive TUI (default)
agenthop

# List sessions (uses index)
agenthop list
agenthop list --provider codex --limit 20

# Show messages
agenthop show <session-id>
agenthop show 67417609 --provider claude-code --limit 10

# Migrate when one agent hits rate limits
agenthop migrate <session-id> --to codex
agenthop migrate <session-id> --from cursor --to claude-code -y

# Index management
agenthop index rebuild          # full rebuild (first run may take ~30-60s for 2000+ Codex files)
agenthop index update           # incremental (default before list)
agenthop index status

# Export / import portable JSON
agenthop export <id> -o session.agenthop.json
agenthop import session.agenthop.json --to opencode
```

## Supported providers

| Provider | ID | Storage | Resume command |
|----------|-----|---------|----------------|
| Claude Code | `claude-code` | `~/.claude/projects/<encoded>/*.jsonl` | `claude --resume <id>` |
| Codex | `codex` | `~/.codex/sessions/**/rollout-*.jsonl` | `codex resume <id>` |
| Cursor CLI | `cursor` | `~/.cursor/chats/*/store.db` + transcripts | `cursor-agent --resume <id>` |
| OpenCode | `opencode` | `~/.local/share/opencode/opencode.db` | `opencode --session <id>` |
| CommandCode | `commandcode` | `~/.commandcode/projects/*.jsonl` | `commandcode --resume <id>` |
| Hermes | `hermes` | `~/.hermes/state.db` | `hermes --session <id>` |
| Stubs | `devin`, `windsurf`, `gemini-cli`, `aider` | documented paths | contribute a provider |

Check installation: `agenthop providers doctor`

## Performance vs ctxmv

ctxmv rescans all session files when resolving short IDs. agenthop maintains a local SQLite index with incremental mtime updates:

1. `agenthop list` → index query only
2. `--provider codex` → never touches Claude's 600+ files
3. Full UUID → direct index hit, no tree walk
4. TUI selects provider before loading sessions

## TUI keys

| Key | Action |
|-----|--------|
| Enter | Select provider / session / migrate target |
| `m` | Migrate selected session |
| `r` | Refresh index |
| `/` | Filter sessions (built-in list filter) |
| Esc | Go back |
| `q` | Quit |

## Adding a provider

See [docs/adding-a-provider.md](docs/adding-a-provider.md).

## Limitations

- Cursor **GUI** sessions differ from `cursor-agent` CLI storage
- Tool/system messages may not round-trip on all targets
- Claude Code resume requires correct project directory (`cd` hint printed)
- Some transforms are lossy (path encoding, metadata)

## License

MIT — see [LICENSE](LICENSE).
