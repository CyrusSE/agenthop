package codex_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/CyrusSE/agenthop/internal/provider"
	"github.com/CyrusSE/agenthop/internal/providers/codex"
)

func TestLoadFixture(t *testing.T) {
	p := codex.New()
	path := filepath.Join("..", "..", "..", "testdata", "codex", "sample.jsonl")
	conv, err := p.Load(context.Background(), provider.SessionRef{
		ID: "test-session-001", StoragePath: path,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(conv.Messages) != 2 {
		t.Fatalf("messages = %d", len(conv.Messages))
	}
	if conv.Messages[0].PlainText() != "Hello from codex fixture" {
		t.Fatalf("user msg = %q", conv.Messages[0].PlainText())
	}
}
