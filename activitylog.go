package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const logDir = "/var/lib/screentimectl/log"

type LogEntry struct {
	Time   string `json:"time"`
	Status string `json:"status"`
}

type ActivityLog struct {
	baseDir string
}

func NewActivityLog(baseDir string) *ActivityLog {
	return &ActivityLog{baseDir: baseDir}
}

func (l *ActivityLog) AppendEntry(user, status string) error {
	dir := filepath.Join(l.baseDir, user)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating log dir: %w", err)
	}

	path := filepath.Join(dir, today()+".log")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("opening log file: %w", err)
	}
	defer f.Close()

	entry := LogEntry{
		Time:   time.Now().Format("15:04:05"),
		Status: status,
	}
	return json.NewEncoder(f).Encode(entry)
}

func (l *ActivityLog) ReadDay(user, date string) ([]LogEntry, error) {
	path := filepath.Join(l.baseDir, user, date+".log")
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var entries []LogEntry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var entry LogEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}
		entries = append(entries, entry)
	}
	return entries, scanner.Err()
}

func FormatTimeline(entries []LogEntry) string {
	if len(entries) == 0 {
		return "  No activity today"
	}

	var result string
	for i, entry := range entries {
		endTime := ""
		if i+1 < len(entries) {
			endTime = entries[i+1].Time
		} else {
			endTime = time.Now().Format("15:04:05")
		}

		dur := timelineDuration(entry.Time, endTime)
		result += fmt.Sprintf("  %s-%s (%s) - %s\n", entry.Time[:5], endTime[:5], dur, entry.Status)
	}
	return result
}

func timelineDuration(start, end string) string {
	t1, err1 := time.Parse("15:04:05", start)
	t2, err2 := time.Parse("15:04:05", end)
	if err1 != nil || err2 != nil {
		return "?"
	}
	d := t2.Sub(t1)
	if d < 0 {
		return "?"
	}
	return formatDuration(int(d.Seconds()))
}
