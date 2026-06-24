package util

import (
	"testing"
	"time"
)

func TestFormatRelative(t *testing.T) {
	now := time.Now()
	cases := []struct {
		at   time.Time
		want string
	}{
		{time.Time{}, "unknown"},
		{now.Add(-30 * time.Second), "just now"},
		{now.Add(-5 * time.Minute), "5m ago"},
		{now.Add(-2 * time.Hour), "2h ago"},
		{now.Add(-3 * 24 * time.Hour), "3d ago"},
		{now.Add(-14 * 24 * time.Hour), "2w ago"},
		{now.Add(-400 * 24 * time.Hour), now.Add(-400 * 24 * time.Hour).Format("Jan 2, 2006")},
	}
	for _, tc := range cases {
		got := FormatRelative(tc.at)
		if got != tc.want {
			t.Errorf("FormatRelative(%v) = %q, want %q", tc.at, got, tc.want)
		}
	}
}
