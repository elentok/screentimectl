package main

import "testing"

func TestBotDefaultUser(t *testing.T) {
	restore := stubSessionFuncs()
	defer restore()

	b := &Bot{cfg: &Config{Users: []UserConfig{{Name: "bob"}}}}
	user, err := b.defaultUser()
	if err != nil {
		t.Fatalf("defaultUser single user: %v", err)
	}
	if user != "bob" {
		t.Fatalf("defaultUser single user = %q, want bob", user)
	}

	b = &Bot{cfg: &Config{Users: []UserConfig{{Name: "bob"}, {Name: "alice"}}}}
	getUserSessionStatusFunc = func(user string) string {
		if user == "alice" {
			return "active"
		}
		return "locked"
	}
	user, err = b.defaultUser()
	if err != nil {
		t.Fatalf("defaultUser active user: %v", err)
	}
	if user != "alice" {
		t.Fatalf("defaultUser active user = %q, want alice", user)
	}

	getUserSessionStatusFunc = func(string) string { return "locked" }
	if _, err := b.defaultUser(); err == nil {
		t.Fatal("defaultUser with no active user succeeded, want error")
	}

	getUserSessionStatusFunc = func(string) string { return "active" }
	if _, err := b.defaultUser(); err == nil {
		t.Fatal("defaultUser with multiple active users succeeded, want error")
	}
}

func TestBotParsesOmittedUserCommands(t *testing.T) {
	b := &Bot{cfg: &Config{Users: []UserConfig{{Name: "bob"}}}}

	user, duration, err := b.parseUserDuration([]string{"15m"})
	if err != nil {
		t.Fatalf("parseUserDuration: %v", err)
	}
	if user != "bob" || duration != "15m" {
		t.Fatalf("parseUserDuration = %q, %q; want bob, 15m", user, duration)
	}

	user, duration, err = b.parseUserOptionalDuration(nil)
	if err != nil {
		t.Fatalf("parseUserOptionalDuration no args: %v", err)
	}
	if user != "bob" || duration != "" {
		t.Fatalf("parseUserOptionalDuration no args = %q, %q; want bob, empty", user, duration)
	}

	user, duration, err = b.parseUserOptionalDuration([]string{"15m"})
	if err != nil {
		t.Fatalf("parseUserOptionalDuration duration: %v", err)
	}
	if user != "bob" || duration != "15m" {
		t.Fatalf("parseUserOptionalDuration duration = %q, %q; want bob, 15m", user, duration)
	}

	user, hours, err := b.parseUserOptionalHours([]string{"8-20"})
	if err != nil {
		t.Fatalf("parseUserOptionalHours: %v", err)
	}
	if user != "bob" || hours != "8-20" {
		t.Fatalf("parseUserOptionalHours = %q, %q; want bob, 8-20", user, hours)
	}

	user, msg, err := b.parseUserMessage([]string{"hello", "there"})
	if err != nil {
		t.Fatalf("parseUserMessage: %v", err)
	}
	if user != "bob" || msg != "hello there" {
		t.Fatalf("parseUserMessage = %q, %q; want bob, hello there", user, msg)
	}
}

func TestBotParsesOmittedUserWithActiveFallback(t *testing.T) {
	restore := stubSessionFuncs()
	defer restore()

	b := &Bot{cfg: &Config{Users: []UserConfig{{Name: "bob"}, {Name: "alice"}}}}
	getUserSessionStatusFunc = func(user string) string {
		if user == "alice" {
			return "active"
		}
		return "locked"
	}

	user, duration, err := b.parseUserDuration([]string{"15m"})
	if err != nil {
		t.Fatalf("parseUserDuration active fallback: %v", err)
	}
	if user != "alice" || duration != "15m" {
		t.Fatalf("parseUserDuration active fallback = %q, %q; want alice, 15m", user, duration)
	}
}

func TestBotParsesExplicitUserCommands(t *testing.T) {
	b := &Bot{cfg: &Config{Users: []UserConfig{{Name: "bob"}, {Name: "alice"}}}}

	user, duration, err := b.parseUserDuration([]string{"alice", "15m"})
	if err != nil {
		t.Fatalf("parseUserDuration explicit: %v", err)
	}
	if user != "alice" || duration != "15m" {
		t.Fatalf("parseUserDuration explicit = %q, %q; want alice, 15m", user, duration)
	}

	user, duration, err = b.parseUserOptionalDuration([]string{"alice"})
	if err != nil {
		t.Fatalf("parseUserOptionalDuration explicit: %v", err)
	}
	if user != "alice" || duration != "" {
		t.Fatalf("parseUserOptionalDuration explicit = %q, %q; want alice, empty", user, duration)
	}

	user, hours, err := b.parseUserOptionalHours([]string{"alice", "8-20"})
	if err != nil {
		t.Fatalf("parseUserOptionalHours explicit: %v", err)
	}
	if user != "alice" || hours != "8-20" {
		t.Fatalf("parseUserOptionalHours explicit = %q, %q; want alice, 8-20", user, hours)
	}

	user, msg, err := b.parseUserMessage([]string{"alice", "hello"})
	if err != nil {
		t.Fatalf("parseUserMessage explicit: %v", err)
	}
	if user != "alice" || msg != "hello" {
		t.Fatalf("parseUserMessage explicit = %q, %q; want alice, hello", user, msg)
	}
}

func TestBotParseUserMessageRequiresMessageForExplicitUser(t *testing.T) {
	b := &Bot{cfg: &Config{Users: []UserConfig{{Name: "bob"}}}}

	if _, _, err := b.parseUserMessage([]string{"bob"}); err == nil {
		t.Fatal("parseUserMessage explicit user without message succeeded, want error")
	}
}

func TestBotParseUserDurationRequiresDurationForExplicitUser(t *testing.T) {
	b := &Bot{cfg: &Config{Users: []UserConfig{{Name: "bob"}}}}

	if _, _, err := b.parseUserDuration([]string{"bob"}); err == nil {
		t.Fatal("parseUserDuration explicit user without duration succeeded, want error")
	}
}
