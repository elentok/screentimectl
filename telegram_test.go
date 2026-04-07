package main

import "testing"

func TestAdminCommandsDefaultUser(t *testing.T) {
	restore := stubSessionFuncs()
	defer restore()

	cmd := NewAdminCommands(&Config{Users: []UserConfig{{Name: "bob"}}}, nil)
	user, err := cmd.defaultUser()
	if err != nil {
		t.Fatalf("defaultUser single user: %v", err)
	}
	if user != "bob" {
		t.Fatalf("defaultUser single user = %q, want bob", user)
	}

	cmd = NewAdminCommands(&Config{Users: []UserConfig{{Name: "bob"}, {Name: "alice"}}}, nil)
	getUserSessionStatusFunc = func(user string) string {
		if user == "alice" {
			return "active"
		}
		return "locked"
	}
	user, err = cmd.defaultUser()
	if err != nil {
		t.Fatalf("defaultUser active user: %v", err)
	}
	if user != "alice" {
		t.Fatalf("defaultUser active user = %q, want alice", user)
	}

	getUserSessionStatusFunc = func(string) string { return "locked" }
	if _, err := cmd.defaultUser(); err == nil {
		t.Fatal("defaultUser with no active user succeeded, want error")
	}

	getUserSessionStatusFunc = func(string) string { return "active" }
	if _, err := cmd.defaultUser(); err == nil {
		t.Fatal("defaultUser with multiple active users succeeded, want error")
	}
}

func TestAdminCommandsParsesOmittedUserCommands(t *testing.T) {
	cmd := NewAdminCommands(&Config{Users: []UserConfig{{Name: "bob"}}}, nil)

	user, duration, err := cmd.parseUserDuration([]string{"15m"})
	if err != nil {
		t.Fatalf("parseUserDuration: %v", err)
	}
	if user != "bob" || duration != "15m" {
		t.Fatalf("parseUserDuration = %q, %q; want bob, 15m", user, duration)
	}

	user, duration, err = cmd.parseUserOptionalDuration(nil)
	if err != nil {
		t.Fatalf("parseUserOptionalDuration no args: %v", err)
	}
	if user != "bob" || duration != "" {
		t.Fatalf("parseUserOptionalDuration no args = %q, %q; want bob, empty", user, duration)
	}

	user, duration, err = cmd.parseUserOptionalDuration([]string{"15m"})
	if err != nil {
		t.Fatalf("parseUserOptionalDuration duration: %v", err)
	}
	if user != "bob" || duration != "15m" {
		t.Fatalf("parseUserOptionalDuration duration = %q, %q; want bob, 15m", user, duration)
	}

	user, hours, err := cmd.parseUserOptionalHours([]string{"8-20"})
	if err != nil {
		t.Fatalf("parseUserOptionalHours: %v", err)
	}
	if user != "bob" || hours != "8-20" {
		t.Fatalf("parseUserOptionalHours = %q, %q; want bob, 8-20", user, hours)
	}

	user, msg, err := cmd.parseUserMessage([]string{"hello", "there"})
	if err != nil {
		t.Fatalf("parseUserMessage: %v", err)
	}
	if user != "bob" || msg != "hello there" {
		t.Fatalf("parseUserMessage = %q, %q; want bob, hello there", user, msg)
	}
}

func TestAdminCommandsParsesOmittedUserWithActiveFallback(t *testing.T) {
	restore := stubSessionFuncs()
	defer restore()

	cmd := NewAdminCommands(&Config{Users: []UserConfig{{Name: "bob"}, {Name: "alice"}}}, nil)
	getUserSessionStatusFunc = func(user string) string {
		if user == "alice" {
			return "active"
		}
		return "locked"
	}

	user, duration, err := cmd.parseUserDuration([]string{"15m"})
	if err != nil {
		t.Fatalf("parseUserDuration active fallback: %v", err)
	}
	if user != "alice" || duration != "15m" {
		t.Fatalf("parseUserDuration active fallback = %q, %q; want alice, 15m", user, duration)
	}
}

func TestAdminCommandsParsesExplicitUserCommands(t *testing.T) {
	cmd := NewAdminCommands(&Config{Users: []UserConfig{{Name: "bob"}, {Name: "alice"}}}, nil)

	user, duration, err := cmd.parseUserDuration([]string{"alice", "15m"})
	if err != nil {
		t.Fatalf("parseUserDuration explicit: %v", err)
	}
	if user != "alice" || duration != "15m" {
		t.Fatalf("parseUserDuration explicit = %q, %q; want alice, 15m", user, duration)
	}

	user, duration, err = cmd.parseUserOptionalDuration([]string{"alice"})
	if err != nil {
		t.Fatalf("parseUserOptionalDuration explicit: %v", err)
	}
	if user != "alice" || duration != "" {
		t.Fatalf("parseUserOptionalDuration explicit = %q, %q; want alice, empty", user, duration)
	}

	user, hours, err := cmd.parseUserOptionalHours([]string{"alice", "8-20"})
	if err != nil {
		t.Fatalf("parseUserOptionalHours explicit: %v", err)
	}
	if user != "alice" || hours != "8-20" {
		t.Fatalf("parseUserOptionalHours explicit = %q, %q; want alice, 8-20", user, hours)
	}

	user, msg, err := cmd.parseUserMessage([]string{"alice", "hello"})
	if err != nil {
		t.Fatalf("parseUserMessage explicit: %v", err)
	}
	if user != "alice" || msg != "hello" {
		t.Fatalf("parseUserMessage explicit = %q, %q; want alice, hello", user, msg)
	}
}

func TestAdminCommandsParseUserMessageRequiresMessageForExplicitUser(t *testing.T) {
	cmd := NewAdminCommands(&Config{Users: []UserConfig{{Name: "bob"}}}, nil)

	if _, _, err := cmd.parseUserMessage([]string{"bob"}); err == nil {
		t.Fatal("parseUserMessage explicit user without message succeeded, want error")
	}
}

func TestAdminCommandsParseUserDurationRequiresDurationForExplicitUser(t *testing.T) {
	cmd := NewAdminCommands(&Config{Users: []UserConfig{{Name: "bob"}}}, nil)

	if _, _, err := cmd.parseUserDuration([]string{"bob"}); err == nil {
		t.Fatal("parseUserDuration explicit user without duration succeeded, want error")
	}
}
