package index_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/CyrusSE/agenthop/internal/index"
	"github.com/CyrusSE/agenthop/internal/model"
)

func TestStoreUpsertList(t *testing.T) {
	dir := t.TempDir()
	store, err := index.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	now := time.Now()
	sm := model.Summary{
		ID: "abc-123", Provider: "codex", Title: "test session",
		UpdatedAt: now, MessageCount: 5, StoragePath: "/tmp/x.jsonl", SourceMtime: now.Unix(),
	}
	if err := store.Upsert(sm); err != nil {
		t.Fatal(err)
	}
	items, err := store.List(index.ListOpts{Provider: "codex"})
	if err != nil || len(items) != 1 {
		t.Fatalf("list: %d items err=%v", len(items), err)
	}
	if items[0].ID != "abc-123" {
		t.Fatalf("id = %q", items[0].ID)
	}
}

func TestNeedsRefresh(t *testing.T) {
	dir := t.TempDir()
	store, err := index.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	_ = store.Upsert(model.Summary{
		ID: "x", Provider: "codex", StoragePath: "/a.jsonl", SourceMtime: 100,
	})
	need, err := store.NeedsRefresh("codex", "/a.jsonl", 100)
	if err != nil || need {
		t.Fatalf("should not need refresh: need=%v err=%v", need, err)
	}
	need, _ = store.NeedsRefresh("codex", "/a.jsonl", 200)
	if !need {
		t.Fatal("should need refresh after mtime change")
	}
}

func TestOpenCreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "cache")
	dbPath := filepath.Join(dir, "index.db")
	store, err := index.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	store.Close()
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatal(err)
	}
	_ = context.Background()
}
