package migrate_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CyrusSE/agenthop/internal/migrate"
	"github.com/CyrusSE/agenthop/internal/model"
	"github.com/CyrusSE/agenthop/internal/providers/codex"
)

func TestFindExistingMigration(t *testing.T) {
	dir := t.TempDir()
	conv := &model.Conversation{
		ID: "src-1", Provider: "claude-code",
		Messages: []model.Message{{Role: model.RoleUser, Content: "hello dedup"}},
	}
	meta := model.NewMigrationMeta(conv)
	path := filepath.Join(dir, "rollout-test-abc.jsonl")
	line := `{"type":"agenthop_migration","data":` + mustJSON(meta) + `}`
	if err := os.WriteFile(path, []byte(line+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, ok := migrate.FindExistingMigration(path, meta.OriginDigest)
	if !ok || got != path {
		t.Fatalf("FindExistingMigration = %q ok=%v", got, ok)
	}
}

func TestFindDuplicateProvider(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sessions", "2026", "06", "24")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	conv := &model.Conversation{
		ID: "src-2", Provider: "claude-code",
		Messages: []model.Message{
			{Role: model.RoleUser, Content: "migrate me"},
			{Role: model.RoleAssistant, Content: "ok"},
		},
	}
	meta := model.NewMigrationMeta(conv)
	path := filepath.Join(sub, "rollout-2026-06-24-deadbeef.jsonl")
	lines := strings.Join([]string{
		`{"type":"session_meta","session_id":"deadbeef"}`,
		`{"type":"agenthop_migration","data":` + mustJSON(meta) + `}`,
	}, "\n") + "\n"
	if err := os.WriteFile(path, []byte(lines), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CODEX_HOME", dir)
	p := codex.New()
	wr, ok := migrate.FindDuplicate(p, conv)
	if !ok {
		t.Fatal("expected duplicate")
	}
	if wr.SessionID != "deadbeef" {
		t.Fatalf("session id = %q", wr.SessionID)
	}
	_ = p
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}
