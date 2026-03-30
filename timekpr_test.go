package main

import (
	"testing"
)

func TestFormatDuration(t *testing.T) {
	cases := []struct {
		seconds int
		want    string
	}{
		{0, "0m"},
		{60, "1m"},
		{3600, "1h"},
		{3660, "1h 1m"},
		{5400, "1h 30m"},
		{7200, "2h"},
	}
	for _, tc := range cases {
		got := formatDuration(tc.seconds)
		if got != tc.want {
			t.Errorf("formatDuration(%d) = %q, want %q", tc.seconds, got, tc.want)
		}
	}
}
