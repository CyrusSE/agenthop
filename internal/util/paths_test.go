package util_test

import (
	"path/filepath"
	"testing"

	"github.com/CyrusSE/agenthop/internal/util"
)

func TestEncodeClaudeProjectPath(t *testing.T) {
	got := util.EncodeClaudeProjectPath("/home/user/proj")
	want := "-home-user-proj"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestDecodeCursorProjectPath(t *testing.T) {
	got := util.DecodeCursorProjectPath("home-cyrus-Documents-test-miggrate")
	want := "/home/cyrus/Documents/test/miggrate"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestMatchID(t *testing.T) {
	id := "01234567-89ab-cdef-0123-456789abcdef"
	if !util.MatchID(id, "89abcdef") {
		t.Fatal("suffix match failed")
	}
	if !util.MatchID(id, id) {
		t.Fatal("exact match failed")
	}
}

func TestProjectCWDUseDescendants(t *testing.T) {
	home := util.HomeDir()
	if home == "" {
		t.Skip("no home dir")
	}
	if util.ProjectCWDUseDescendants(home) {
		t.Fatal("home should not use descendant matching")
	}
	if !util.ProjectCWDUseDescendants(filepath.Join(home, "proj")) {
		t.Fatal("project dir should include descendant sessions")
	}
}
