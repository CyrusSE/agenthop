package hermes

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/CyrusSE/agenthop/internal/provider"
	_ "modernc.org/sqlite"
)

func TestDiscoverNullTitleSessions(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "state.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	schema := `
CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    source TEXT NOT NULL,
    started_at REAL NOT NULL,
    message_count INTEGER DEFAULT 0,
    title TEXT,
    cwd TEXT,
    archived INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL,
    role TEXT NOT NULL,
    content TEXT,
    active INTEGER NOT NULL DEFAULT 1
);`
	if _, err := db.Exec(schema); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO sessions (id, source, started_at, message_count, title) VALUES
		('with-title', 'cli', 1000, 1, 'Has title'),
		('null-title', 'cli', 2000, 2, NULL)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO messages (session_id, role, content, active) VALUES
		('null-title', 'user', 'First user prompt', 1),
		('null-title', 'assistant', 'Reply', 1)`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	p := &Provider{dbPath: dbPath}
	sums, err := p.Discover(context.Background(), provider.DiscoverOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if len(sums) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sums))
	}
	byID := map[string]string{}
	for _, s := range sums {
		byID[s.ID] = s.Title
	}
	if byID["with-title"] != "Has title" {
		t.Fatalf("with-title: %q", byID["with-title"])
	}
	if byID["null-title"] != "First user prompt" {
		t.Fatalf("null-title: %q", byID["null-title"])
	}

	conv, err := p.Load(context.Background(), provider.SessionRef{ID: "null-title", StoragePath: dbPath + "#null-title"})
	if err != nil {
		t.Fatal(err)
	}
	if conv.Title != "First user prompt" {
		t.Fatalf("load title: %q", conv.Title)
	}
	if len(conv.Messages) != 2 {
		t.Fatalf("messages: %d", len(conv.Messages))
	}
}

func TestDiscoverSkipsArchived(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "state.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE sessions (
		id TEXT PRIMARY KEY, source TEXT NOT NULL, started_at REAL NOT NULL,
		message_count INTEGER DEFAULT 0, title TEXT, cwd TEXT, archived INTEGER NOT NULL DEFAULT 0
	); CREATE TABLE messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT, session_id TEXT NOT NULL, role TEXT NOT NULL,
		content TEXT, active INTEGER NOT NULL DEFAULT 1
	);`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO sessions (id, source, started_at, message_count, title, archived) VALUES
		('live', 'cli', 1, 0, 'live', 0),
		('gone', 'cli', 2, 0, 'gone', 1)`); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()

	p := &Provider{dbPath: dbPath}
	sums, err := p.Discover(context.Background(), provider.DiscoverOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if len(sums) != 1 || sums[0].ID != "live" {
		t.Fatalf("got %+v", sums)
	}
}

func TestInstalled(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "state.db")
	if err := os.WriteFile(dbPath, []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}
	p := &Provider{dbPath: dbPath}
	if !p.Installed() {
		t.Fatal("expected installed")
	}
}
