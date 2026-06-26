package tui

import (
	"testing"

	"github.com/CyrusSE/agenthop/internal/index"
)

func TestListOptsForHereFilter(t *testing.T) {
	m := modelState{
		cwdMode:       true,
		cwd:           "/home/user/proj",
		showAllOnPage: true,
		pageSize:      55,
		providerFilter: "codex",
	}
	opts := listOptsFor(m)
	if opts.ProjectCWD != "/home/user/proj" {
		t.Fatalf("ProjectCWD = %q, want exact cwd", opts.ProjectCWD)
	}
	if opts.Limit != maxShowAllPage {
		t.Fatalf("Limit = %d, want maxShowAllPage %d when showAllOnPage", opts.Limit, maxShowAllPage)
	}
	if opts.Provider != "codex" {
		t.Fatalf("Provider = %q", opts.Provider)
	}
}

func TestListOptsForEverywhere(t *testing.T) {
	m := modelState{cwdMode: false, cwd: "/home/user", showAllOnPage: true}
	opts := listOptsFor(m)
	if opts.ProjectCWD != "" {
		t.Fatalf("ProjectCWD should be empty when cwdMode false, got %q", opts.ProjectCWD)
	}
}

func TestListOptsPaginationWhenNotShowAll(t *testing.T) {
	m := modelState{
		cwdMode:       true,
		cwd:           "/tmp",
		showAllOnPage: false,
		pageSize:      50,
		pageOffset:    100,
	}
	opts := listOptsFor(m)
	if opts.Limit != 50 || opts.Offset != 100 {
		t.Fatalf("got limit=%d offset=%d, want 50/100", opts.Limit, opts.Offset)
	}
	var _ index.ListOpts = opts
}
