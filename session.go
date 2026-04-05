package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/user"
	"strings"
	"time"
)

const pollInterval = 10 * time.Second

type SessionManager struct {
	cfg   *Config
	store *UsageStore
	bot   *Bot
}

func NewSessionManager(cfg *Config, store *UsageStore, bot *Bot) *SessionManager {
	return &SessionManager{cfg: cfg, store: store, bot: bot}
}

func (m *SessionManager) Run(ctx context.Context) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.poll()
		}
	}
}

func (m *SessionManager) poll() {
	newDay := m.store.ResetIfNewDay()
	if newDay {
		// Unlock accounts at day reset if within allowed hours
		for _, u := range m.cfg.Users {
			if isWithinAllowedHours(u.AllowedHours) {
				if err := unlockAccount(u.Name); err != nil {
					log.Printf("session: unlock %s on day reset: %v", u.Name, err)
				}
			}
		}
	}

	for _, u := range m.cfg.Users {
		m.pollUser(u)
	}

	if err := m.store.Save(); err != nil {
		log.Printf("session: save usage: %v", err)
	}
}

func (m *SessionManager) pollUser(u UserConfig) {
	limitSeconds := u.DailyLimitMinutes * 60
	hasOverride := m.store.HasOverride(u.Name)

	// Check allowed hours
	if !isWithinAllowedHours(u.AllowedHours) && !hasOverride {
		sessions := findUserSessions(u.Name)
		if len(sessions) > 0 {
			log.Printf("session: %s outside allowed hours, locking", u.Name)
			lockOutUser(u.Name, sessions)
		}
		return
	}

	sessions := findUserSessions(u.Name)
	active := false
	for _, sid := range sessions {
		if isSessionActive(sid) {
			active = true
			break
		}
	}

	if active {
		m.store.AddUsedTime(u.Name, int(pollInterval.Seconds()))
	}

	ut := m.store.GetUserTime(u.Name, limitSeconds)

	// Check notification thresholds
	remainingMinutes := ut.RemainingSeconds / 60
	for _, threshold := range m.cfg.Notifications.Thresholds {
		if remainingMinutes <= threshold && !m.store.AlreadyNotified(u.Name, threshold) {
			m.store.MarkNotified(u.Name, threshold)
			msg := fmt.Sprintf("You have %d minutes of screen time remaining", threshold)
			sendNotification(u.Name, msg)
			sendTTS(u.Name, msg)
			m.sendAll(fmt.Sprintf("%s has %s remaining", capitalize(u.Name), ut.RemainingStr()))
		}
	}

	// Time expired
	if ut.RemainingSeconds <= 0 && len(sessions) > 0 {
		log.Printf("session: %s time expired, locking", u.Name)
		msg := "Your screen time is up!"
		sendNotification(u.Name, msg)
		sendTTS(u.Name, msg)
		lockOutUser(u.Name, sessions)
		m.sendAll(fmt.Sprintf("%s's screen time has expired", capitalize(u.Name)))
	}
}

// GetUserTime returns the current time info for a user.
func (m *SessionManager) GetUserTime(user string) (UserTime, error) {
	u := m.cfg.getUser(user)
	if u == nil {
		return UserTime{}, fmt.Errorf("unknown user: %s", user)
	}
	return m.store.GetUserTime(user, u.DailyLimitMinutes*60), nil
}

// AddTime adds bonus minutes and sets an override if outside allowed hours.
func (m *SessionManager) AddTime(user string, minutes int) (UserTime, error) {
	u := m.cfg.getUser(user)
	if u == nil {
		return UserTime{}, fmt.Errorf("unknown user: %s", user)
	}
	m.store.AddBonusTime(user, minutes*60)
	if !isWithinAllowedHours(u.AllowedHours) {
		m.store.SetOverride(user, time.Now().Add(time.Duration(minutes)*time.Minute))
	}
	if err := unlockAccount(user); err != nil {
		log.Printf("session: unlock %s after AddTime: %v", user, err)
	}
	if err := m.store.Save(); err != nil {
		log.Printf("session: save after AddTime: %v", err)
	}
	return m.store.GetUserTime(user, u.DailyLimitMinutes*60), nil
}

// SetTime sets the remaining time to exactly the given minutes.
func (m *SessionManager) SetTime(user string, minutes int) (UserTime, error) {
	u := m.cfg.getUser(user)
	if u == nil {
		return UserTime{}, fmt.Errorf("unknown user: %s", user)
	}
	m.store.SetRemainingTime(user, minutes*60, u.DailyLimitMinutes*60)
	if err := m.store.Save(); err != nil {
		log.Printf("session: save after SetTime: %v", err)
	}
	return m.store.GetUserTime(user, u.DailyLimitMinutes*60), nil
}

// LockUser terminates all sessions and locks the account.
func (m *SessionManager) LockUser(user string) error {
	return lockOutUser(user, findUserSessions(user))
}

// UnlockUser unlocks the account.
func (m *SessionManager) UnlockUser(user string) error {
	return unlockAccount(user)
}

// sendAll sends a message to all Telegram chats if the bot is connected.
func (m *SessionManager) sendAll(text string) {
	if m.bot != nil {
		m.bot.sendAll(text)
	}
}

// loginctl helpers

func findUserSessions(username string) []string {
	out, err := exec.Command("loginctl", "list-sessions", "--no-legend").Output()
	if err != nil {
		return nil
	}
	var sessions []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Fields(line)
		// Format: SESSION UID USER SEAT TTY STATE ...
		if len(fields) >= 3 && fields[2] == username {
			sessions = append(sessions, fields[0])
		}
	}
	return sessions
}

func isSessionActive(sessionID string) bool {
	out, err := exec.Command("loginctl", "show-session", sessionID).Output()
	if err != nil {
		return false
	}
	props := parseProperties(string(out))
	return props["Active"] == "yes" && props["IdleHint"] == "no" && props["LockedHint"] == "no"
}

func parseProperties(output string) map[string]string {
	props := make(map[string]string)
	for _, line := range strings.Split(output, "\n") {
		if k, v, ok := strings.Cut(line, "="); ok {
			props[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	return props
}

func lockOutUser(username string, sessions []string) error {
	for _, sid := range sessions {
		lockSession(sid)
	}
	return lockAccount(username)
}

func lockSession(sessionID string) {
	if err := exec.Command("sudo", "loginctl", "lock-session", sessionID).Run(); err != nil {
		log.Printf("session: lock-session %s: %v", sessionID, err)
	}
}

func lockAccount(username string) error {
	if err := exec.Command("sudo", "passwd", "-l", username).Run(); err != nil {
		return fmt.Errorf("passwd -l %s: %w", username, err)
	}
	return nil
}

func unlockAccount(username string) error {
	if err := exec.Command("sudo", "passwd", "-u", username).Run(); err != nil {
		return fmt.Errorf("passwd -u %s: %w", username, err)
	}
	return nil
}

func isWithinAllowedHours(hours AllowedHours) bool {
	h := time.Now().Hour()
	return h >= hours.Start && h < hours.End
}

// Notification helpers

func sendNotification(username string, msg string) {
	uid := resolveUID(username)
	if uid == "" {
		return
	}
	xdg := fmt.Sprintf("XDG_RUNTIME_DIR=/run/user/%s", uid)
	cmd := exec.Command("sudo", "--preserve-env=XDG_RUNTIME_DIR", "-u", username, "notify-send", "Screen Time", msg)
	cmd.Env = append(os.Environ(), xdg)
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("session: notify-send for %s: %v\ncmd: %s\noutput: %s", username, err, cmd.Args, out)
	}
}

func sendTTS(username string, msg string) {
	uid := resolveUID(username)
	if uid == "" {
		return
	}
	xdg := fmt.Sprintf("XDG_RUNTIME_DIR=/run/user/%s", uid)
	cmd := exec.Command("sudo", "--preserve-env=XDG_RUNTIME_DIR", "-u", username, "espeak-ng", msg)
	cmd.Env = append(os.Environ(), xdg)
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("session: espeak-ng for %s: %v\ncmd: %s\noutput: %s", username, err, cmd.Args, out)
	}
}

func resolveUID(username string) string {
	u, err := user.Lookup(username)
	if err != nil {
		log.Printf("session: lookup uid for %s: %v", username, err)
		return ""
	}
	return u.Uid
}
