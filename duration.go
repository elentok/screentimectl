package main

import (
	"fmt"
	"regexp"
	"strconv"
)

var reDuration = regexp.MustCompile(`^(?:(\d+)h)?(?:(\d+)m?)?$`)

// parseDurationMinutes parses a duration string into minutes.
// Accepts: "15", "15m", "1h", "1h30m", "1h30"
func parseDurationMinutes(s string) (int, error) {
	m := reDuration.FindStringSubmatch(s)
	if m == nil || (m[1] == "" && m[2] == "") {
		return 0, fmt.Errorf("invalid duration %q: use formats like 15, 15m, 1h, 1h30m", s)
	}

	var total int
	if m[1] != "" {
		h, _ := strconv.Atoi(m[1])
		total += h * 60
	}
	if m[2] != "" {
		mins, _ := strconv.Atoi(m[2])
		total += mins
	}

	if total <= 0 {
		return 0, fmt.Errorf("duration must be greater than 0")
	}
	return total, nil
}
