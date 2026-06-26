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

func TestProjectPathMatchesCWD(t *testing.T) {
	home := util.HomeDir()
	if home == "" {
		t.Skip("no home dir")
	}
	proj := filepath.Join(home, "proj")
	sub := filepath.Join(proj, "sub")
	if util.ProjectPathMatchesCWD(home, home) {
		t.Fatal("exact home path should not match home filter")
	}
	if !util.ProjectPathMatchesCWD(proj, home) {
		t.Fatal("project under home should match home filter")
	}
	if util.ProjectPathMatchesCWD(sub, proj) {
		t.Fatal("subdir should not match exact project filter")
	}
	if util.ProjectPathMatchesCWD("/other", proj) {
		t.Fatal("unrelated path should not match")
	}
}

func TestProjectPathMatchesCWDExact(t *testing.T) {
	home := util.HomeDir()
	if home == "" {
		t.Skip("no home dir")
	}
	proj := filepath.Join(home, "proj")
	if !util.ProjectPathMatchesCWD(proj, proj) {
		t.Fatal("exact path should match")
	}
}
