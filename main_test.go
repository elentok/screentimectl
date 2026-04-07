package main

import "testing"

func TestFormatStatusSummary(t *testing.T) {
	ut := UserTime{
		RemainingSeconds: 90 * 60,
		UsedSeconds:      2 * 60 * 60,
	}

	if got, want := formatStatusSummary(ut, false), "You have 1h 30m remaining (used 2h today)"; got != want {
		t.Fatalf("formatStatusSummary = %q, want %q", got, want)
	}
	if got, want := formatStatusSummary(ut, true), "1h 30m remaining"; got != want {
		t.Fatalf("formatStatusSummary compact = %q, want %q", got, want)
	}
}
