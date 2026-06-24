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

func TestFindByIDAmbiguousSuffix(t *testing.T) {
	dir := t.TempDir()
	store, err := index.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	now := time.Now()
	for _, id := range []string{"aaa-deadbeef", "bbb-deadbeef"} {
		if err := store.Upsert(model.Summary{
			ID: id, Provider: "codex", Title: id,
			UpdatedAt: now, StoragePath: "/tmp/" + id, SourceMtime: now.Unix(),
		}); err != nil {
			t.Fatal(err)
		}
	}
	_, err = store.FindByID("deadbeef")
	if err == nil {
		t.Fatal("expected ambiguous error")
	}
}

func TestMigrationDedup(t *testing.T) {
	dir := t.TempDir()
	store, err := index.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.RecordMigration("opencode", "digest-abc", "ses_123", "/db#ses_123"); err != nil {
		t.Fatal(err)
	}
	sid, path, ok := store.FindMigration("opencode", "digest-abc")
	if !ok || sid != "ses_123" || path != "/db#ses_123" {
		t.Fatalf("FindMigration = %q %q ok=%v", sid, path, ok)
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
