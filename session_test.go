package main

import (
	"path/filepath"
	"testing"
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
	sendTTSFunc = func(string, string) { tts++ }
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

	return func() {
		findUserSessionsFunc = prevFindSessions
		getUserSessionStatusFunc = prevGetStatus
		lockOutUserFunc = prevLockOut
		unlockAccountFunc = prevUnlock
		sendNotificationFunc = prevNotify
		sendTTSFunc = prevTTS
		isWithinAllowedHoursFunc = prevAllowed
	}
}
