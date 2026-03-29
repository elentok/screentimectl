package main

import (
	"testing"
)

func TestParseUserInfo(t *testing.T) {
	// Sample timekpra --userinfo output format
	sample := `
ALLOWED_WEEKDAYS;1;2;3;4;5;6;7
TIME_SPENT_DAY;3720
TIME_LEFT_DAY;1800
`
	ut, err := parseUserInfo(sample)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ut.RemainingSeconds != 1800 {
		t.Errorf("RemainingSeconds = %d, want 1800", ut.RemainingSeconds)
	}
	if ut.UsedSeconds != 3720 {
		t.Errorf("UsedSeconds = %d, want 3720", ut.UsedSeconds)
	}
}

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
