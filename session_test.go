package main

import (
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestSessionManagerPollUserHandlesExpiryOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "usage.json")
	store, err := NewUsageStore(path)
	if err != nil {
		t.Fatalf("NewUsageStore: %v", err)
	}

	cfg := &Config{
		Users: []UserConfig{{
			Name:              "bob",
			DailyLimitMinutes: 0,
			AllowedHours:      AllowedHours{Start: 0, End: 23},
		}},
	}

	restore := stubSessionFuncs()
	defer restore()

	var notifications, tts, locks int
	findUserSessionsFunc = func(string) []string { return []string{"1"} }
	getUserSessionStatusFunc = func(string) string { return "active" }
	lockOutUserFunc = func(string, []string) error {
		locks++
		return nil
	}
	sendNotificationFunc = func(string, string) { notifications++ }
	sendTTSFunc = func(string, string, []string) { tts++ }
	isWithinAllowedHoursFunc = func(AllowedHours) bool { return true }

	logDir := t.TempDir()
	mgr := NewSessionManager(cfg, store, nil, NewActivityLog(logDir))

	mgr.pollUser(cfg.Users[0])
	mgr.pollUser(cfg.Users[0])

	if notifications != 1 {
		t.Fatalf("notifications = %d, want 1", notifications)
	}
	if tts != 1 {
		t.Fatalf("tts = %d, want 1", tts)
	}
	if locks != 1 {
		t.Fatalf("locks = %d, want 1", locks)
	}
	if !store.IsExpiryHandled("bob") {
		t.Fatal("expected expiry to be marked handled")
	}

	entries, err := mgr.actLog.ReadDay("bob", today())
	if err != nil {
		t.Fatalf("ReadDay: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("activity entries = %d, want 1", len(entries))
	}
	if entries[0].Status != "time-expired" {
		t.Fatalf("activity status = %q, want %q", entries[0].Status, "time-expired")
	}
}

func stubSessionFuncs() func() {
	prevFindSessions := findUserSessionsFunc
	prevGetStatus := getUserSessionStatusFunc
	prevLockOut := lockOutUserFunc
	prevUnlock := unlockAccountFunc
	prevNotify := sendNotificationFunc
	prevTTS := sendTTSFunc
	prevAllowed := isWithinAllowedHoursFunc
	prevNewLockCmd := newLockAccountCmdFunc
	prevNewUnlockCmd := newUnlockAccountCmdFunc

	return func() {
		findUserSessionsFunc = prevFindSessions
		getUserSessionStatusFunc = prevGetStatus
		lockOutUserFunc = prevLockOut
		unlockAccountFunc = prevUnlock
		sendNotificationFunc = prevNotify
		sendTTSFunc = prevTTS
		isWithinAllowedHoursFunc = prevAllowed
		newLockAccountCmdFunc = prevNewLockCmd
		newUnlockAccountCmdFunc = prevNewUnlockCmd
	}
}

func TestAccountCommandsUseChageExpiry(t *testing.T) {
	restore := stubSessionFuncs()
	defer restore()

	lockCmd := newLockAccountCmdFunc("bob")
	if got, want := lockCmd.Args, []string{"sudo", "chage", "-E", "0", "bob"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("lock cmd = %v, want %v", got, want)
	}

	unlockCmd := newUnlockAccountCmdFunc("bob")
	if got, want := unlockCmd.Args, []string{"sudo", "chage", "-E", "-1", "bob"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unlock cmd = %v, want %v", got, want)
	}

	newLockAccountCmdFunc = func(username string) *exec.Cmd {
		return exec.Command("true")
	}
	newUnlockAccountCmdFunc = func(username string) *exec.Cmd {
		return exec.Command("true")
	}

	if err := lockAccount("bob"); err != nil {
		t.Fatalf("lockAccount: %v", err)
	}
	if err := unlockAccount("bob"); err != nil {
		t.Fatalf("unlockAccount: %v", err)
	}
}

func TestTTSCommandUsesConfiguredVoice(t *testing.T) {
	cmd := newTTSCommand("bob", "gmw/en-US", "hello")
	want := []string{"sudo", "--preserve-env=XDG_RUNTIME_DIR", "-u", "bob", "espeak-ng", "-v", "gmw/en-US", "hello"}
	if got := cmd.Args; !reflect.DeepEqual(got, want) {
		t.Fatalf("tts cmd = %v, want %v", got, want)
	}
}

func TestSessionManagerSetTimeUnlocksPositiveTime(t *testing.T) {
	path := filepath.Join(t.TempDir(), "usage.json")
	store, err := NewUsageStore(path)
	if err != nil {
		t.Fatalf("NewUsageStore: %v", err)
	}

	cfg := &Config{
		Users: []UserConfig{{
			Name:              "bob",
			DailyLimitMinutes: 60,
			AllowedHours:      AllowedHours{Start: 8, End: 18},
		}},
	}

	restore := stubSessionFuncs()
	defer restore()

	var unlocks int
	unlockAccountFunc = func(string) error {
		unlocks++
		return nil
	}
	isWithinAllowedHoursFunc = func(AllowedHours) bool { return false }

	mgr := NewSessionManager(cfg, store, nil, nil)
	if _, err := mgr.SetTime("bob", 15); err != nil {
		t.Fatalf("SetTime: %v", err)
	}

	if unlocks != 1 {
		t.Fatalf("unlocks = %d, want 1", unlocks)
	}
	if !store.HasOverride("bob") {
		t.Fatal("expected SetTime to create override outside allowed hours")
	}
	if store.IsExpiryHandled("bob") {
		t.Fatal("expected positive SetTime to clear expiry handled")
	}
}

func TestAdminCommandsUnlockSetsPositiveTime(t *testing.T) {
	path := filepath.Join(t.TempDir(), "usage.json")
	store, err := NewUsageStore(path)
	if err != nil {
		t.Fatalf("NewUsageStore: %v", err)
	}

	cfg := &Config{
		Users: []UserConfig{{
			Name:              "bob",
			DailyLimitMinutes: 60,
			AllowedHours:      AllowedHours{Start: 8, End: 18},
		}},
	}

	restore := stubSessionFuncs()
	defer restore()

	var unlocks int
	unlockAccountFunc = func(string) error {
		unlocks++
		return nil
	}
	isWithinAllowedHoursFunc = func(AllowedHours) bool { return true }

	mgr := NewSessionManager(cfg, store, nil, nil)
	text, err := NewAdminCommands(cfg, mgr).Unlock([]string{"bob", "15m"})
	if err != nil {
		t.Fatalf("Unlock: %v", err)
	}
	if text != "Bob now has 15m remaining" {
		t.Fatalf("Unlock text = %q, want %q", text, "Bob now has 15m remaining")
	}
	if unlocks != 1 {
		t.Fatalf("unlocks = %d, want 1", unlocks)
	}

	ut := store.GetUserTime("bob", 60*60)
	if ut.RemainingSeconds != 15*60 {
		t.Fatalf("remaining = %d, want %d", ut.RemainingSeconds, 15*60)
	}
}

func TestStartupUnlock(t *testing.T) {
	cases := []struct {
		name          string
		withinHours   bool
		usedSeconds   int
		limitSeconds  int
		hasOverride   bool
		wantUnlocks   int
	}{
		{name: "within hours with time remaining", withinHours: true, usedSeconds: 0, limitSeconds: 3600, wantUnlocks: 1},
		{name: "outside hours no override", withinHours: false, usedSeconds: 0, limitSeconds: 3600, wantUnlocks: 0},
		{name: "within hours no time remaining", withinHours: true, usedSeconds: 3600, limitSeconds: 3600, wantUnlocks: 0},
		{name: "override active regardless of hours or time", withinHours: false, usedSeconds: 3600, limitSeconds: 3600, hasOverride: true, wantUnlocks: 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "usage.json")
			store, err := NewUsageStore(path)
			if err != nil {
				t.Fatalf("NewUsageStore: %v", err)
			}

			cfg := &Config{
				Users: []UserConfig{{
					Name:              "bob",
					DailyLimitMinutes: tc.limitSeconds / 60,
					AllowedHours:      AllowedHours{Start: 8, End: 18},
				}},
			}

			restore := stubSessionFuncs()
			defer restore()

			store.AddUsedTime("bob", tc.usedSeconds)
			if tc.hasOverride {
				store.SetOverride("bob", time.Now().Add(time.Hour))
			}

			var unlocks int
			unlockAccountFunc = func(string) error {
				unlocks++
				return nil
			}
			isWithinAllowedHoursFunc = func(AllowedHours) bool { return tc.withinHours }

			mgr := NewSessionManager(cfg, store, nil, nil)
			mgr.startupUnlock()

			if unlocks != tc.wantUnlocks {
				t.Fatalf("unlocks = %d, want %d", unlocks, tc.wantUnlocks)
			}
		})
	}
}

func TestAdminCommandsLockPositiveDurationCompatibilityAlias(t *testing.T) {
	path := filepath.Join(t.TempDir(), "usage.json")
	store, err := NewUsageStore(path)
	if err != nil {
		t.Fatalf("NewUsageStore: %v", err)
	}

	cfg := &Config{
		Users: []UserConfig{{
			Name:              "bob",
			DailyLimitMinutes: 60,
			AllowedHours:      AllowedHours{Start: 8, End: 18},
		}},
	}

	restore := stubSessionFuncs()
	defer restore()

	var unlocks int
	unlockAccountFunc = func(string) error {
		unlocks++
		return nil
	}
	isWithinAllowedHoursFunc = func(AllowedHours) bool { return true }

	mgr := NewSessionManager(cfg, store, nil, nil)
	text, err := NewAdminCommands(cfg, mgr).Lock([]string{"bob", "15m"})
	if err != nil {
		t.Fatalf("Lock positive duration: %v", err)
	}
	if text != "Bob now has 15m remaining" {
		t.Fatalf("Lock text = %q, want %q", text, "Bob now has 15m remaining")
	}
	if unlocks != 1 {
		t.Fatalf("unlocks = %d, want 1", unlocks)
	}
}
