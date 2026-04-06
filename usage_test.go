package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestUsageStore_AddAndGet(t *testing.T) {
	path := filepath.Join(t.TempDir(), "usage.json")
	store, err := NewUsageStore(path)
	if err != nil {
		t.Fatalf("NewUsageStore: %v", err)
	}

	store.AddUsedTime("bob", 3600)
	ut := store.GetUserTime("bob", 18000) // 5h limit

	if ut.UsedSeconds != 3600 {
		t.Errorf("UsedSeconds = %d, want 3600", ut.UsedSeconds)
	}
	if ut.RemainingSeconds != 14400 {
		t.Errorf("RemainingSeconds = %d, want 14400", ut.RemainingSeconds)
	}
}

func TestUsageStore_BonusTime(t *testing.T) {
	path := filepath.Join(t.TempDir(), "usage.json")
	store, err := NewUsageStore(path)
	if err != nil {
		t.Fatalf("NewUsageStore: %v", err)
	}

	store.AddUsedTime("bob", 18000) // used all 5h
	store.AddBonusTime("bob", 900)  // +15m bonus

	ut := store.GetUserTime("bob", 18000)
	if ut.RemainingSeconds != 900 {
		t.Errorf("RemainingSeconds = %d, want 900", ut.RemainingSeconds)
	}
}

func TestUsageStore_SetRemainingTime(t *testing.T) {
	path := filepath.Join(t.TempDir(), "usage.json")
	store, err := NewUsageStore(path)
	if err != nil {
		t.Fatalf("NewUsageStore: %v", err)
	}

	store.AddUsedTime("bob", 10000)
	store.SetRemainingTime("bob", 1800, 18000) // set to 30m remaining

	ut := store.GetUserTime("bob", 18000)
	if ut.RemainingSeconds != 1800 {
		t.Errorf("RemainingSeconds = %d, want 1800", ut.RemainingSeconds)
	}
}

func TestUsageStore_Override(t *testing.T) {
	path := filepath.Join(t.TempDir(), "usage.json")
	store, err := NewUsageStore(path)
	if err != nil {
		t.Fatalf("NewUsageStore: %v", err)
	}

	if store.HasOverride("bob") {
		t.Error("expected no override initially")
	}

	store.SetOverride("bob", time.Now().Add(30*time.Minute))
	if !store.HasOverride("bob") {
		t.Error("expected override to be active")
	}

	store.SetOverride("bob", time.Now().Add(-1*time.Minute))
	if store.HasOverride("bob") {
		t.Error("expected expired override to be inactive")
	}
}

func TestUsageStore_Notifications(t *testing.T) {
	path := filepath.Join(t.TempDir(), "usage.json")
	store, err := NewUsageStore(path)
	if err != nil {
		t.Fatalf("NewUsageStore: %v", err)
	}

	if store.AlreadyNotified("bob", 15) {
		t.Error("expected not notified initially")
	}

	store.MarkNotified("bob", 15)
	if !store.AlreadyNotified("bob", 15) {
		t.Error("expected notified after marking")
	}
	if store.AlreadyNotified("bob", 5) {
		t.Error("expected other threshold not notified")
	}
}

func TestUsageStore_ExpiryHandledResetsWhenTimeIsAdded(t *testing.T) {
	path := filepath.Join(t.TempDir(), "usage.json")
	store, err := NewUsageStore(path)
	if err != nil {
		t.Fatalf("NewUsageStore: %v", err)
	}

	store.SetExpiryHandled("bob", true)
	store.AddBonusTime("bob", 300)

	if store.IsExpiryHandled("bob") {
		t.Error("expected expiry handled flag to reset when bonus time is added")
	}
}

func TestUsageStore_Persistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "usage.json")
	store, err := NewUsageStore(path)
	if err != nil {
		t.Fatalf("NewUsageStore: %v", err)
	}

	store.AddUsedTime("bob", 5000)
	if err := store.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	store2, err := NewUsageStore(path)
	if err != nil {
		t.Fatalf("NewUsageStore (reload): %v", err)
	}

	ut := store2.GetUserTime("bob", 18000)
	if ut.UsedSeconds != 5000 {
		t.Errorf("UsedSeconds after reload = %d, want 5000", ut.UsedSeconds)
	}
}

func TestUsageStore_DayReset(t *testing.T) {
	path := filepath.Join(t.TempDir(), "usage.json")
	store, err := NewUsageStore(path)
	if err != nil {
		t.Fatalf("NewUsageStore: %v", err)
	}

	store.AddUsedTime("bob", 5000)

	// Simulate a day change by modifying the date
	store.mu.Lock()
	store.data.Date = "2000-01-01"
	store.mu.Unlock()

	if !store.ResetIfNewDay() {
		t.Error("expected reset on day change")
	}

	ut := store.GetUserTime("bob", 18000)
	if ut.UsedSeconds != 0 {
		t.Errorf("UsedSeconds after reset = %d, want 0", ut.UsedSeconds)
	}
}

func TestUsageStore_RemainingFloor(t *testing.T) {
	path := filepath.Join(t.TempDir(), "usage.json")
	store, err := NewUsageStore(path)
	if err != nil {
		t.Fatalf("NewUsageStore: %v", err)
	}

	store.AddUsedTime("bob", 20000) // over the 18000 limit
	ut := store.GetUserTime("bob", 18000)
	if ut.RemainingSeconds != 0 {
		t.Errorf("RemainingSeconds = %d, want 0 (should floor at 0)", ut.RemainingSeconds)
	}
}

func TestUsageStore_MissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent", "usage.json")
	_, err := NewUsageStore(path)
	if err != nil {
		t.Fatalf("NewUsageStore should handle missing file: %v", err)
	}

	// Make sure the parent dir is needed for save
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
}
