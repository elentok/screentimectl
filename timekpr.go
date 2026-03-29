package main

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

type UserTime struct {
	RemainingSeconds int
	UsedSeconds      int
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

// GetUserTime returns remaining and used seconds for a user.
// Parses output of: timekpra --userinfo <user>
func GetUserTime(user string) (UserTime, error) {
	out, err := exec.Command("sudo", "timekpra", "--userinfo", user).Output()
	if err != nil {
		return UserTime{}, fmt.Errorf("timekpra --userinfo %s: %w", user, err)
	}

	return parseUserInfo(string(out))
}

// timekpra --userinfo output contains lines like:
//
//	TIME_LEFT_TODAY_ALL;600
//	TIME_SPENT_BALANCE;1200
//
// We look for TIME_LEFT_TODAY_ALL and TIME_SPENT_BALANCE (or TIME_SPENT_TODAY).
var reTimekprField = regexp.MustCompile(`(?m)^(\w+)\s*[;:]\s*(-?\d+)`)

func parseUserInfo(output string) (UserTime, error) {
	fields := make(map[string]int)
	for _, match := range reTimekprField.FindAllStringSubmatch(output, -1) {
		v, _ := strconv.Atoi(match[2])
		fields[strings.ToUpper(match[1])] = v
	}

	// Try keys in order of preference
	remaining, hasRemaining := lookupFirst(fields, "TIME_LEFT_DAY", "TIME_LEFT_TODAY_ALL", "TIMELEFT_TOTAL")
	if !hasRemaining {
		return UserTime{}, fmt.Errorf("could not parse remaining time from timekpra output:\n%s", output)
	}

	used, _ := lookupFirst(fields, "TIME_SPENT_DAY", "TIME_SPENT_BALANCE", "TIME_SPENT_TODAY")

	return UserTime{
		RemainingSeconds: remaining,
		UsedSeconds:      used,
	}, nil
}

func lookupFirst(fields map[string]int, keys ...string) (int, bool) {
	for _, k := range keys {
		if v, ok := fields[k]; ok {
			return v, true
		}
	}
	return 0, false
}

// AddTime adds minutes to the user's remaining time.
func AddTime(user string, minutes int) (UserTime, error) {
	arg := fmt.Sprintf("%dm", minutes)
	cmd := exec.Command("sudo", "timekpra", "--settimeleft", user, "+", arg)
	if out, err := cmd.CombinedOutput(); err != nil {
		return UserTime{}, fmt.Errorf("timekpra --settimeleft %s + %s: %w\n%s", user, arg, err, out)
	}
	return GetUserTime(user)
}

// SetTime sets the user's remaining time to exactly minutes (0 = lock).
func SetTime(user string, minutes int) (UserTime, error) {
	arg := fmt.Sprintf("%dm", minutes)
	cmd := exec.Command("sudo", "timekpra", "--settimeleft", user, "=", arg)
	if out, err := cmd.CombinedOutput(); err != nil {
		return UserTime{}, fmt.Errorf("timekpra --settimeleft %s = %s: %w\n%s", user, arg, err, out)
	}
	return GetUserTime(user)
}
