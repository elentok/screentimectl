package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Bot struct {
	api *tgbotapi.BotAPI
	cfg *Config
	mgr *SessionManager
}

func newBot(cfg *Config, mgr *SessionManager) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(cfg.Telegram.BotToken)
	if err != nil {
		return nil, fmt.Errorf("creating telegram bot: %w", err)
	}
	log.Printf("telegram: authorized as @%s", api.Self.UserName)
	b := &Bot{api: api, cfg: cfg, mgr: mgr}
	if mgr != nil {
		mgr.bot = b
	}
	return b, nil
}

func (b *Bot) send(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	if _, err := b.api.Send(msg); err != nil {
		log.Printf("telegram send error: %v", err)
	}
}

// sendAll sends a message to all allowed chat IDs.
func (b *Bot) sendAll(text string) {
	for _, id := range b.cfg.Telegram.AllowedChatIDs {
		b.send(id, text)
	}
}

func (b *Bot) run() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := b.api.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}
		msg := update.Message
		log.Printf("telegram: message from chat %d (@%s): %s", msg.Chat.ID, msg.From.UserName, msg.Text)
		if !b.cfg.isAllowedChat(msg.Chat.ID) {
			log.Printf("telegram: ignoring message from unallowed chat %d", msg.Chat.ID)
			continue
		}
		if !msg.IsCommand() {
			continue
		}
		b.handleCommand(msg)
	}
}

func (b *Bot) handleCommand(msg *tgbotapi.Message) {
	cmd := msg.Command()
	args := strings.Fields(msg.CommandArguments())
	chatID := msg.Chat.ID

	switch cmd {
	case "give":
		b.handleGive(chatID, args)
	case "lock":
		b.handleLock(chatID, args)
	case "status":
		b.handleStatus(chatID, args)
	case "hours":
		b.handleHours(chatID, args)
	case "say":
		b.handleSay(chatID, args)
	default:
		b.send(chatID, "Unknown command. Use /give, /lock, /status, /hours, or /say.")
	}
}

func (b *Bot) handleGive(chatID int64, args []string) {
	user, durStr, err := b.parseUserDuration(args)
	if err != nil {
		b.send(chatID, fmt.Sprintf("Usage: /give [user] {duration} (%v)", err))
		return
	}

	minutes, err := parseDurationMinutes(durStr)
	if err != nil {
		b.send(chatID, fmt.Sprintf("Invalid duration: %v", err))
		return
	}

	ut, err := b.mgr.AddTime(user, minutes)
	if err != nil {
		b.send(chatID, fmt.Sprintf("Failed to apply command: %v", err))
		return
	}

	b.send(chatID, fmt.Sprintf("%s now has %s remaining", capitalize(user), ut.RemainingStr()))
}

func (b *Bot) handleLock(chatID int64, args []string) {
	user, durStr, err := b.parseUserOptionalDuration(args)
	if err != nil {
		b.send(chatID, fmt.Sprintf("Usage: /lock [user] [duration] (%v)", err))
		return
	}

	minutes := 0
	if durStr != "" {
		minutes, err = parseDurationMinutes(durStr)
		if err != nil {
			b.send(chatID, fmt.Sprintf("Invalid duration: %v", err))
			return
		}
	}

	if minutes == 0 {
		if err := b.mgr.LockUser(user); err != nil {
			b.send(chatID, fmt.Sprintf("Failed to lock: %v", err))
			return
		}
		b.send(chatID, fmt.Sprintf("%s has been locked out", capitalize(user)))
	} else {
		ut, err := b.mgr.SetTime(user, minutes)
		if err != nil {
			b.send(chatID, fmt.Sprintf("Failed to apply command: %v", err))
			return
		}
		b.send(chatID, fmt.Sprintf("%s now has %s remaining", capitalize(user), ut.RemainingStr()))
	}
}

func (b *Bot) handleStatus(chatID int64, args []string) {
	user, err := b.parseOptionalUser(args)
	if err != nil {
		b.send(chatID, fmt.Sprintf("Usage: /status [user] (%v)", err))
		return
	}

	ut, err := b.mgr.GetUserTime(user)
	if err != nil {
		b.send(chatID, fmt.Sprintf("Failed to apply command: %v", err))
		return
	}

	u := b.cfg.getUser(user)
	text := fmt.Sprintf("%s has %s remaining (used %s today)\nAllowed hours: %dam - %dpm\nSession: %s",
		capitalize(user), ut.RemainingStr(), ut.UsedStr(),
		u.AllowedHours.Start, u.AllowedHours.End%12, ut.SessionStatus)

	if b.mgr.actLog != nil {
		entries, err := b.mgr.actLog.ReadDay(user, today())
		if err == nil && len(entries) > 0 {
			text += "\n\nToday:\n" + FormatTimeline(entries)
		}
	}

	b.send(chatID, text)
}

func (b *Bot) handleHours(chatID int64, args []string) {
	user, hours, err := b.parseUserOptionalHours(args)
	if err != nil {
		b.send(chatID, fmt.Sprintf("Usage: /hours [user] [start-end] (%v)", err))
		return
	}

	u := b.cfg.getUser(user)

	if hours == "" {
		b.send(chatID, fmt.Sprintf("Allowed hours for %s: %dam - %dpm",
			capitalize(user), u.AllowedHours.Start, u.AllowedHours.End%12))
		return
	}

	start, end, err := parseHoursRange(hours)
	if err != nil {
		b.send(chatID, fmt.Sprintf("Invalid hours: %v", err))
		return
	}

	u.AllowedHours = AllowedHours{Start: start, End: end}
	if err := b.cfg.save(configPath); err != nil {
		b.send(chatID, fmt.Sprintf("Failed to save config: %v", err))
		return
	}

	b.send(chatID, fmt.Sprintf("Updated allowed hours for %s: %dam - %dpm",
		capitalize(user), start, end%12))
}

func (b *Bot) handleSay(chatID int64, args []string) {
	user, msg, err := b.parseUserMessage(args)
	if err != nil {
		b.send(chatID, fmt.Sprintf("Usage: /say [user] {message} (%v)", err))
		return
	}
	sendTTS(user, msg)
	sendNotification(user, msg)
	b.send(chatID, fmt.Sprintf("Sent to %s: %q", capitalize(user), msg))
}

func (b *Bot) parseOptionalUser(args []string) (string, error) {
	switch len(args) {
	case 0:
		return b.defaultUser()
	case 1:
		return b.requireValidUser(args[0])
	default:
		return "", fmt.Errorf("too many arguments")
	}
}

func (b *Bot) parseUserDuration(args []string) (string, string, error) {
	switch len(args) {
	case 1:
		if b.cfg.isValidUser(args[0]) {
			return "", "", fmt.Errorf("missing duration")
		}
		user, err := b.defaultUser()
		return user, args[0], err
	case 2:
		user, err := b.requireValidUser(args[0])
		return user, args[1], err
	default:
		return "", "", fmt.Errorf("expected duration, or user and duration")
	}
}

func (b *Bot) parseUserOptionalDuration(args []string) (string, string, error) {
	switch len(args) {
	case 0:
		user, err := b.defaultUser()
		return user, "", err
	case 1:
		if b.cfg.isValidUser(args[0]) {
			return args[0], "", nil
		}
		user, err := b.defaultUser()
		return user, args[0], err
	case 2:
		user, err := b.requireValidUser(args[0])
		return user, args[1], err
	default:
		return "", "", fmt.Errorf("too many arguments")
	}
}

func (b *Bot) parseUserOptionalHours(args []string) (string, string, error) {
	switch len(args) {
	case 0:
		user, err := b.defaultUser()
		return user, "", err
	case 1:
		if b.cfg.isValidUser(args[0]) {
			return args[0], "", nil
		}
		if _, _, err := parseHoursRange(args[0]); err != nil {
			return "", "", fmt.Errorf("unknown user: %s", args[0])
		}
		user, err := b.defaultUser()
		return user, args[0], err
	case 2:
		user, err := b.requireValidUser(args[0])
		return user, args[1], err
	default:
		return "", "", fmt.Errorf("too many arguments")
	}
}

func (b *Bot) parseUserMessage(args []string) (string, string, error) {
	if len(args) == 0 {
		return "", "", fmt.Errorf("missing message")
	}
	if b.cfg.isValidUser(args[0]) {
		if len(args) == 1 {
			return "", "", fmt.Errorf("missing message")
		}
		return args[0], strings.Join(args[1:], " "), nil
	}
	user, err := b.defaultUser()
	if err != nil {
		return "", "", err
	}
	return user, strings.Join(args, " "), nil
}

func (b *Bot) requireValidUser(name string) (string, error) {
	if !b.cfg.isValidUser(name) {
		return "", fmt.Errorf("unknown user: %s", name)
	}
	return name, nil
}

func (b *Bot) defaultUser() (string, error) {
	if len(b.cfg.Users) == 1 {
		return b.cfg.Users[0].Name, nil
	}

	var active []string
	for _, u := range b.cfg.Users {
		if getUserSessionStatusFunc(u.Name) == "active" {
			active = append(active, u.Name)
		}
	}
	switch len(active) {
	case 1:
		return active[0], nil
	case 0:
		return "", fmt.Errorf("no active configured user; specify a user")
	default:
		return "", fmt.Errorf("multiple active configured users (%s); specify a user", strings.Join(active, ", "))
	}
}

func parseHoursRange(s string) (int, int, error) {
	parts := strings.SplitN(s, "-", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("use format start-end (e.g. 8-18)")
	}
	start, err := strconv.Atoi(parts[0])
	if err != nil || start < 0 || start > 23 {
		return 0, 0, fmt.Errorf("invalid start hour: %s", parts[0])
	}
	end, err := strconv.Atoi(parts[1])
	if err != nil || end < 0 || end > 23 {
		return 0, 0, fmt.Errorf("invalid end hour: %s", parts[1])
	}
	if start >= end {
		return 0, 0, fmt.Errorf("start must be before end")
	}
	return start, end, nil
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
