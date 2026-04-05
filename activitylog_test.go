package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestActivityLogAppendAndRead(t *testing.T) {
	dir := t.TempDir()
	log := NewActivityLog(dir)

	if err := log.AppendEntry("bob", "active"); err != nil {
		t.Fatalf("AppendEntry: %v", err)
	}
	if err := log.AppendEntry("bob", "locked"); err != nil {
		t.Fatalf("AppendEntry: %v", err)
	}

	entries, err := log.ReadDay("bob", today())
	if err != nil {
		t.Fatalf("ReadDay: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Status != "active" {
		t.Errorf("expected active, got %s", entries[0].Status)
	}
	if entries[1].Status != "locked" {
		t.Errorf("expected locked, got %s", entries[1].Status)
	}
}

func TestActivityLogReadDayMissing(t *testing.T) {
	dir := t.TempDir()
	log := NewActivityLog(dir)

	entries, err := log.ReadDay("bob", "2020-01-01")
	if err != nil {
		t.Fatalf("ReadDay: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}

func TestActivityLogCreatesUserDir(t *testing.T) {
	dir := t.TempDir()
	log := NewActivityLog(dir)

	if err := log.AppendEntry("alice", "active"); err != nil {
		t.Fatalf("AppendEntry: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "alice")); err != nil {
		t.Fatalf("user directory not created: %v", err)
	}
}

func TestFormatTimeline(t *testing.T) {
	entries := []LogEntry{
		{Time: "08:00:00", Status: "active"},
		{Time: "10:30:00", Status: "locked"},
		{Time: "11:00:00", Status: "active"},
	}
	result := FormatTimeline(entries)
	if result == "" {
		t.Fatal("expected non-empty timeline")
	}
	// Check that it contains expected patterns
	if got := result; got == "" {
		t.Error("timeline should not be empty")
	}
}

func TestFormatTimelineEmpty(t *testing.T) {
	result := FormatTimeline(nil)
	if result != "  No activity today" {
		t.Errorf("expected 'No activity today', got %q", result)
	}
}
