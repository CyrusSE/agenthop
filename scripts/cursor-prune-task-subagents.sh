#!/usr/bin/env bash
# Prune Cursor Task/subagent (task-*) entries from state.vscdb.
# Clears the "359 Working" panel source (composerHeaders + membership + KV blobs).
# Does NOT touch ~/.cursor/projects transcripts or ~/.codex sessions.
set -euo pipefail

DB="${CURSOR_STATE_DB:-$HOME/.config/Cursor/User/globalStorage/state.vscdb}"

if [[ ! -f "$DB" ]]; then
  echo "Not found: $DB" >&2
  exit 1
fi

if pgrep -x cursor >/dev/null 2>&1; then
  echo "Close Cursor completely (all windows), then run this script again." >&2
  exit 1
fi
if ps -eo args= | rg -q '^/usr/share/cursor/cursor( |$)'; then
  echo "Close Cursor completely (all windows), then run this script again." >&2
  exit 1
fi

before=$(stat -c%s "$DB")
echo "DB size before: $(numfmt --to=iec-i --suffix=B "$before")"

python3 <<'PY'
import json
import os
import sqlite3

db = os.path.expanduser(os.environ.get("CURSOR_STATE_DB", "~/.config/Cursor/User/globalStorage/state.vscdb"))
conn = sqlite3.connect(db, timeout=120)
conn.execute("PRAGMA journal_mode=DELETE")
conn.execute("PRAGMA busy_timeout=120000")


def is_task_id(cid: str) -> bool:
    s = str(cid or "")
    return s.startswith("task-") or s.startswith("task-tool_")


# composer.composerHeaders — drives the Working panel list
row = conn.execute("SELECT value FROM ItemTable WHERE key='composer.composerHeaders'").fetchone()
if row and row[0]:
    headers = json.loads(row[0])
    before = len(headers.get("allComposers", []))
    headers["allComposers"] = [
        c for c in headers.get("allComposers", []) if not is_task_id(c.get("composerId"))
    ]
    after = len(headers["allComposers"])
    conn.execute(
        "UPDATE ItemTable SET value=? WHERE key='composer.composerHeaders'",
        (json.dumps(headers),),
    )
    print(f"composerHeaders: {before} -> {after} (removed {before - after})")

# glass.localAgentProjectMembership.v1
row = conn.execute("SELECT value FROM ItemTable WHERE key='glass.localAgentProjectMembership.v1'").fetchone()
if row and row[0]:
    mem = json.loads(row[0])
    before = len(mem)
    mem = {k: v for k, v in mem.items() if not is_task_id(k)}
    conn.execute(
        "UPDATE ItemTable SET value=? WHERE key='glass.localAgentProjectMembership.v1'",
        (json.dumps(mem),),
    )
    print(f"localAgentProjectMembership: {before} -> {len(mem)} (removed {before - len(mem)})")

# cursorDiskKV task blobs
patterns = [
    "bubbleId:task-%",
    "composerData:task-%",
    "checkpointId:task-%",
    "messageRequestContext:task-%",
    "ofsContent:task-%",
    "composerVirtualRowHeights:task-%",
]
total = 0
for pat in patterns:
    while True:
        cur = conn.execute("DELETE FROM cursorDiskKV WHERE key LIKE ? LIMIT 10000", (pat,))
        n = cur.rowcount
        conn.commit()
        total += n
        if n == 0:
            break
print(f"cursorDiskKV task-* deleted: {total}")

conn.commit()
conn.execute("VACUUM")
conn.close()
PY

after=$(stat -c%s "$DB")
echo "DB size after:  $(numfmt --to=iec-i --suffix=B "$after")"
echo "Freed approx:   $(numfmt --to=iec-i --suffix=B "$((before - after))")"
echo "Done. Reopen Cursor — the Working panel should be empty."
