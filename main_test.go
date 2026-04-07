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

func TestParseStatusArgs(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantCompact bool
		wantUser    string
		wantErr     bool
	}{
		{name: "none"},
		{name: "compact", args: []string{"--compact"}, wantCompact: true},
		{name: "user", args: []string{"bob"}, wantUser: "bob"},
		{name: "user compact", args: []string{"bob", "--compact"}, wantCompact: true, wantUser: "bob"},
		{name: "compact user", args: []string{"--compact", "bob"}, wantCompact: true, wantUser: "bob"},
		{name: "too many users", args: []string{"bob", "alice"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCompact, gotUser, err := parseStatusArgs(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatal("parseStatusArgs succeeded, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseStatusArgs: %v", err)
			}
			if gotCompact != tt.wantCompact || gotUser != tt.wantUser {
				t.Fatalf("parseStatusArgs = %v, %q; want %v, %q",
					gotCompact, gotUser, tt.wantCompact, tt.wantUser)
			}
		})
	}
}
