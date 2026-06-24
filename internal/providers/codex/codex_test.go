package codex_test

import (
	"context"
	"path/filepath"
	"strings"
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

func TestLoadV2Fixture(t *testing.T) {
	p := codex.New()
	path := filepath.Join("..", "..", "..", "testdata", "codex", "sample-v2.jsonl")
	conv, err := p.Load(context.Background(), provider.SessionRef{
		StoragePath: path,
	})
	if err != nil {
		t.Fatal(err)
	}
	if conv.ID != "019d0304-afe0-7001-b42e-69d2028e34d1" {
		t.Fatalf("id = %q", conv.ID)
	}
	if conv.ProjectPath != "/home/cyrus/Documents/demo" {
		t.Fatalf("project = %q", conv.ProjectPath)
	}
	if len(conv.Messages) < 2 {
		t.Fatalf("messages = %d", len(conv.Messages))
	}
	if conv.Title != "Fix the auth bug in login handler" {
		t.Fatalf("title = %q", conv.Title)
	}
}

func TestSummarizeV2Fixture(t *testing.T) {
	p := codex.New()
	path := filepath.Join("..", "..", "..", "testdata", "codex", "sample-v2.jsonl")
	sum, err := p.SummarizeFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Title != "Fix the auth bug in login handler" {
		t.Fatalf("title = %q", sum.Title)
	}
	if sum.ProjectPath != "/home/cyrus/Documents/demo" {
		t.Fatalf("project = %q", sum.ProjectPath)
	}
}

func TestSummarizeEmptyUsesProjectPath(t *testing.T) {
	p := codex.New()
	path := filepath.Join("..", "..", "..", "testdata", "codex", "sample-empty.jsonl")
	sum, err := p.SummarizeFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sum.Title, "demo") {
		t.Fatalf("title = %q, want project path fallback", sum.Title)
	}
}
