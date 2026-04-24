package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"time"
)

const pollInterval = 10 * time.Second

var (
	findUserSessionsFunc     = findUserSessions
	getUserSessionStatusFunc = getUserSessionStatus
	lockOutUserFunc          = lockOutUser
	unlockAccountFunc        = unlockAccount
	sendNotificationFunc     = sendNotification
	sendTTSFunc              = sendTTS
	isWithinAllowedHoursFunc = isWithinAllowedHours
	newLockAccountCmdFunc    = func(username string) *exec.Cmd {
		return exec.Command("sudo", "chage", "-E", "0", username)
	}
	newUnlockAccountCmdFunc = func(username string) *exec.Cmd {
		return exec.Command("sudo", "chage", "-E", "-1", username)
	}
)

type SessionManager struct {
	cfg        *Config
	store      *UsageStore
	bot        *Bot
	actLog     *ActivityLog
	lastStatus map[string]string
}

func NewSessionManager(cfg *Config, store *UsageStore, bot *Bot, actLog *ActivityLog) *SessionManager {
	return &SessionManager{
		cfg:        cfg,
		store:      store,
		bot:        bot,
		actLog:     actLog,
		lastStatus: make(map[string]string),
	}
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
			if isWithinAllowedHoursFunc(u.AllowedHours) {
				if err := unlockAccountFunc(u.Name); err != nil {
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
	if !isWithinAllowedHoursFunc(u.AllowedHours) && !hasOverride {
		sessions := findUserSessionsFunc(u.Name)
		if len(sessions) > 0 {
			log.Printf("session: %s outside allowed hours, locking", u.Name)
			lockOutUserFunc(u.Name, sessions)
			m.logTransition(u.Name, "locked")
		}
		return
	}

	sessions := findUserSessionsFunc(u.Name)
	status := getUserSessionStatusFunc(u.Name)

	active := status == "active"

	if active {
		m.store.AddUsedTime(u.Name, int(pollInterval.Seconds()))
	}

	ut := m.store.GetUserTime(u.Name, limitSeconds)
	expired := ut.RemainingSeconds <= 0 && len(sessions) > 0
	if expired {
		if !m.store.IsExpiryHandled(u.Name) {
			log.Printf("session: %s time expired, locking", u.Name)
			msg := "Your screen time is up!"
			sendNotificationFunc(u.Name, msg)
			sendTTSFunc(u.Name, msg, m.cfg.TTSModel())
			lockOutUserFunc(u.Name, sessions)
			m.store.SetExpiryHandled(u.Name, true)
			m.logTransition(u.Name, "time-expired")
			m.sendAll(fmt.Sprintf("%s's screen time has expired", capitalize(u.Name)))
		}
		return
	}
	m.store.SetExpiryHandled(u.Name, false)
	m.logTransition(u.Name, status)

	// Check notification thresholds
	remainingMinutes := ut.RemainingSeconds / 60
	for _, threshold := range m.cfg.Notifications.Thresholds {
		if remainingMinutes <= threshold && !m.store.AlreadyNotified(u.Name, threshold) {
			m.store.MarkNotified(u.Name, threshold)
			msg := fmt.Sprintf("You have %d minutes of screen time remaining", threshold)
			sendNotificationFunc(u.Name, msg)
			sendTTSFunc(u.Name, msg, m.cfg.TTSModel())
			m.sendAll(fmt.Sprintf("%s has %s remaining", capitalize(u.Name), ut.RemainingStr()))
		}
	}
}

// GetUserTime returns the current time info for a user.
func (m *SessionManager) GetUserTime(user string) (UserTime, error) {
	u := m.cfg.getUser(user)
	if u == nil {
		return UserTime{}, fmt.Errorf("unknown user: %s", user)
	}
	ut := m.store.GetUserTime(user, u.DailyLimitMinutes*60)
	ut.SessionStatus = getUserSessionStatusFunc(user)
	return ut, nil
}

func getUserSessionStatus(username string) string {
	sessions := findUserSessions(username)
	if len(sessions) == 0 {
		return "offline"
	}
	for _, sid := range sessions {
		out, err := exec.Command("loginctl", "show-session", sid).Output()
		if err != nil {
			continue
		}
		props := parseProperties(string(out))
		if props["Active"] != "yes" {
			continue
		}
		if props["LockedHint"] == "yes" {
			return "locked"
		}
		if props["IdleHint"] == "yes" {
			return "idle"
		}
		return "active"
	}
	return "offline"
}

// AddTime adds bonus minutes and sets an override if outside allowed hours.
func (m *SessionManager) AddTime(user string, minutes int) (UserTime, error) {
	u := m.cfg.getUser(user)
	if u == nil {
		return UserTime{}, fmt.Errorf("unknown user: %s", user)
	}
	m.store.AddBonusTime(user, minutes*60)
	if !isWithinAllowedHoursFunc(u.AllowedHours) {
		m.store.SetOverride(user, time.Now().Add(time.Duration(minutes)*time.Minute))
	}
	if err := unlockAccountFunc(user); err != nil {
		log.Printf("session: unlock %s after AddTime: %v", user, err)
	}
	if err := m.store.Save(); err != nil {
		log.Printf("session: save after AddTime: %v", err)
	}
	ut := m.store.GetUserTime(user, u.DailyLimitMinutes*60)
	msg := fmt.Sprintf("You got %d more minutes! You now have %s remaining", minutes, ut.RemainingStr())
	sendNotificationFunc(user, msg)
	sendTTSFunc(user, msg, m.cfg.TTSModel())
	return ut, nil
}

// SetTime sets the remaining time to exactly the given minutes.
func (m *SessionManager) SetTime(user string, minutes int) (UserTime, error) {
	u := m.cfg.getUser(user)
	if u == nil {
		return UserTime{}, fmt.Errorf("unknown user: %s", user)
	}
	m.store.SetRemainingTime(user, minutes*60, u.DailyLimitMinutes*60)
	if minutes > 0 {
		if !isWithinAllowedHoursFunc(u.AllowedHours) {
			m.store.SetOverride(user, time.Now().Add(time.Duration(minutes)*time.Minute))
		}
		if err := unlockAccountFunc(user); err != nil {
			log.Printf("session: unlock %s after SetTime: %v", user, err)
		}
	}
	if err := m.store.Save(); err != nil {
		log.Printf("session: save after SetTime: %v", err)
	}
	return m.store.GetUserTime(user, u.DailyLimitMinutes*60), nil
}

// LockUser terminates all sessions and locks the account.
func (m *SessionManager) LockUser(user string) error {
	return lockOutUserFunc(user, findUserSessionsFunc(user))
}

// UnlockUser unlocks the account.
func (m *SessionManager) UnlockUser(user string) error {
	return unlockAccountFunc(user)
}

// logTransition logs a status change for the user if it differs from the last known status.
func (m *SessionManager) logTransition(user, status string) {
	if m.actLog == nil {
		return
	}
	if m.lastStatus[user] == status {
		return
	}
	m.lastStatus[user] = status
	if err := m.actLog.AppendEntry(user, status); err != nil {
		log.Printf("session: activity log for %s: %v", user, err)
	}
}

// startupUnlock unlocks user accounts that are currently eligible to log in.
// This recovers from the case where the computer was shut down while a user
// was locked (e.g. ran out of time), leaving the OS-level account lock in
// place across reboots.
func (m *SessionManager) startupUnlock() {
	for _, u := range m.cfg.Users {
		if !m.canUserLogin(u) {
			continue
		}
		log.Printf("session: unlock %s on startup", u.Name)
		if err := unlockAccountFunc(u.Name); err != nil {
			log.Printf("session: startup unlock %s: %v", u.Name, err)
		}
	}
}

// canUserLogin returns true if the user is currently allowed to log in:
// they have an active override, or they are within allowed hours with time remaining.
// This mirrors the logic in runCheckLogin (PAM gate) to prevent drift.
func (m *SessionManager) canUserLogin(u UserConfig) bool {
	if m.store.HasOverride(u.Name) {
		return true
	}
	if !isWithinAllowedHoursFunc(u.AllowedHours) {
		return false
	}
	ut := m.store.GetUserTime(u.Name, u.DailyLimitMinutes*60)
	return ut.RemainingSeconds > 0
}

// LogShutdown writes a shutdown entry for all configured users.
func (m *SessionManager) LogShutdown() {
	if m.actLog == nil {
		return
	}
	for _, u := range m.cfg.Users {
		if err := m.actLog.AppendEntry(u.Name, "shutdown"); err != nil {
			log.Printf("session: shutdown log for %s: %v", u.Name, err)
		}
	}
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
	if err := newLockAccountCmdFunc(username).Run(); err != nil {
		return fmt.Errorf("chage -E 0 %s: %w", username, err)
	}
	return nil
}

func unlockAccount(username string) error {
	if err := newUnlockAccountCmdFunc(username).Run(); err != nil {
		return fmt.Errorf("chage -E -1 %s: %w", username, err)
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

const (
	defaultTTSModel = "en_US-lessac-medium"
	piperBin        = "/usr/local/lib/piper-tts/bin/piper"
	piperVoicesDir  = "/var/lib/screentimectl/piper-voices"
	ttsCacheDir     = "/var/lib/screentimectl/tts-cache"
)

func ttsCachePath(model, msg string) string {
	sum := sha256.Sum256([]byte(model + "\x00" + msg))
	return filepath.Join(ttsCacheDir, fmt.Sprintf("%x.wav", sum))
}

func sendTTS(username string, msg string, model string) {
	uid := resolveUID(username)
	if uid == "" {
		return
	}

	wavPath := ttsCachePath(model, msg)
	if _, err := os.Stat(wavPath); err != nil {
		if err := generateTTS(msg, model, wavPath); err != nil {
			log.Printf("session: piper for %s: %v", username, err)
			return
		}
	}

	xdg := fmt.Sprintf("XDG_RUNTIME_DIR=/run/user/%s", uid)
	playCmd := exec.Command("sudo", "--preserve-env=XDG_RUNTIME_DIR", "-u", username, "paplay", wavPath)
	playCmd.Env = append(os.Environ(), xdg)
	if out, err := playCmd.CombinedOutput(); err != nil {
		log.Printf("session: paplay for %s: %v\noutput: %s", username, err, out)
	}
}

func generateTTS(msg, model, dst string) error {
	if err := os.MkdirAll(ttsCacheDir, 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(ttsCacheDir, "tts-*.wav.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	tmp.Close()

	modelPath := filepath.Join(piperVoicesDir, model+".onnx")
	cmd := exec.Command(piperBin, "--model", modelPath, "--output_file", tmpPath)
	cmd.Stdin = strings.NewReader(msg)
	if out, err := cmd.CombinedOutput(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("%w\noutput: %s", err, out)
	}
	if err := os.Chmod(tmpPath, 0644); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return os.Rename(tmpPath, dst)
}

func resolveUID(username string) string {
	u, err := user.Lookup(username)
	if err != nil {
		log.Printf("session: lookup uid for %s: %v", username, err)
		return ""
	}
	return u.Uid
}
