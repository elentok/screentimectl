package main

import "fmt"

type UserTime struct {
	RemainingSeconds int
	UsedSeconds      int
	SessionStatus    string // "active", "locked", "idle", "offline"
}

func (t UserTime) RemainingStr() string {
	return formatDuration(t.RemainingSeconds)
}

func (t UserTime) UsedStr() string {
	return formatDuration(t.UsedSeconds)
}

func formatDuration(seconds int) string {
	if seconds < 0 {
		seconds = 0
	}
	h := seconds / 3600
	m := (seconds % 3600) / 60
	if h > 0 && m > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	} else if h > 0 {
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dm", m)
}
