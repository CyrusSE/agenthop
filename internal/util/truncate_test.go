package util

import "testing"

func TestTruncateRunes(t *testing.T) {
	if got := TruncateRunes("hello", 10); got != "hello" {
		t.Fatalf("got %q", got)
	}
	if got := TruncateRunes("café au lait", 6); got != "café a…" {
		t.Fatalf("got %q", got)
	}
}

func TestEscapeLike(t *testing.T) {
	if got := EscapeLike(`100%_done`); got != `100\%\_done` {
		t.Fatalf("got %q", got)
	}
}
