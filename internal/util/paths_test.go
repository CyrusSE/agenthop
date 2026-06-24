package util_test

import (
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

func TestMatchID(t *testing.T) {
	id := "01234567-89ab-cdef-0123-456789abcdef"
	if !util.MatchID(id, "89abcdef") {
		t.Fatal("suffix match failed")
	}
	if !util.MatchID(id, id) {
		t.Fatal("exact match failed")
	}
}
