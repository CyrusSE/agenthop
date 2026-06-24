package tui

import "testing"

func TestPageSizeForHeight(t *testing.T) {
	tests := []struct {
		height int
		want   int
	}{
		{0, minPageSize},
		{11, minPageSize},
		{24, minPageSize},
		{56, 50},
		{80, 74},
		{600, maxPageSize},
	}
	for _, tc := range tests {
		if got := pageSizeForHeight(tc.height); got != tc.want {
			t.Errorf("pageSizeForHeight(%d) = %d, want %d", tc.height, got, tc.want)
		}
	}
}
