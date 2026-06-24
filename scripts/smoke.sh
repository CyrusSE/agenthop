#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BIN="$ROOT/bin/agenthop"
[[ -x "$BIN" ]] || { echo "build first: make build"; exit 1; }

echo "== providers =="
"$BIN" providers | head -8

echo "== index status =="
"$BIN" index status

echo "== list (cached) =="
"$BIN" list --provider claude-code --limit 3

echo "== doctor =="
"$BIN" providers doctor | head -8

echo "== import dry-run =="
TMP=$(mktemp)
"$BIN" export $($BIN" list --provider claude-code --limit 1 2>/dev/null | awk 'NR==2{print $1}') --provider claude-code -o "$TMP" 2>/dev/null || true
if [[ -s "$TMP" ]]; then
  "$BIN" import "$TMP" --to codex --dry-run -y
fi
rm -f "$TMP"

echo "OK"
