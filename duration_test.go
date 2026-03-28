package main

import (
	"testing"
)

func TestParseDurationMinutes(t *testing.T) {
	cases := []struct {
		input   string
		want    int
		wantErr bool
	}{
		{"15", 15, false},
		{"15m", 15, false},
		{"1h", 60, false},
		{"1h30m", 90, false},
		{"1h30", 90, false},
		{"2h", 120, false},
		{"0", 0, true},
		{"0m", 0, true},
		{"abc", 0, true},
		{"", 0, true},
	}

	for _, tc := range cases {
		got, err := parseDurationMinutes(tc.input)
		if tc.wantErr {
			if err == nil {
				t.Errorf("parseDurationMinutes(%q) = %d, want error", tc.input, got)
			}
		} else {
			if err != nil {
				t.Errorf("parseDurationMinutes(%q) error: %v", tc.input, err)
			} else if got != tc.want {
				t.Errorf("parseDurationMinutes(%q) = %d, want %d", tc.input, got, tc.want)
			}
		}
	}
}
