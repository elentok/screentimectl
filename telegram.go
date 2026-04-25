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
	case "unlock":
		b.handleUnlock(chatID, args)
	case "status":
		b.handleStatus(chatID, args)
	case "hours":
		b.handleHours(chatID, args)
	case "say":
		b.handleSay(chatID, args)
	default:
		b.send(chatID, "Unknown command. Use /give, /lock, /unlock, /status, /hours, or /say.")
	}
}

func (b *Bot) handleGive(chatID int64, args []string) {
	text, err := NewAdminCommands(b.cfg, b.mgr).Give(args)
	if err != nil {
		b.send(chatID, fmt.Sprintf("Failed to apply command: %v", err))
		return
	}
	b.send(chatID, text)
}

func (b *Bot) handleLock(chatID int64, args []string) {
	text, err := NewAdminCommands(b.cfg, b.mgr).Lock(args)
	if err != nil {
		b.send(chatID, fmt.Sprintf("Failed to apply command: %v", err))
		return
	}
	b.send(chatID, text)
}

func (b *Bot) handleUnlock(chatID int64, args []string) {
	text, err := NewAdminCommands(b.cfg, b.mgr).Unlock(args)
	if err != nil {
		b.send(chatID, fmt.Sprintf("Failed to apply command: %v", err))
		return
	}
	b.send(chatID, text)
}

func (b *Bot) handleStatus(chatID int64, args []string) {
	text, err := NewAdminCommands(b.cfg, b.mgr).Status(args)
	if err != nil {
		b.send(chatID, fmt.Sprintf("Failed to apply command: %v", err))
		return
	}
	b.send(chatID, text)
}

func (b *Bot) handleHours(chatID int64, args []string) {
	text, err := NewAdminCommands(b.cfg, b.mgr).Hours(args)
	if err != nil {
		b.send(chatID, fmt.Sprintf("Failed to apply command: %v", err))
		return
	}
	b.send(chatID, text)
}

func (b *Bot) handleSay(chatID int64, args []string) {
	text, err := NewAdminCommands(b.cfg, b.mgr).Say(args)
	if err != nil {
		b.send(chatID, fmt.Sprintf("Failed to apply command: %v", err))
		return
	}
	b.send(chatID, text)
}

func parseHoursRange(s string) (AllowedHours, error) {
	parts := strings.SplitN(s, "-", 2)
	if len(parts) != 2 {
		return AllowedHours{}, fmt.Errorf("use format start-end (e.g. 8-18 or 8am-6:30pm)")
	}
	startH, startM, err := parseHour(parts[0])
	if err != nil {
		return AllowedHours{}, fmt.Errorf("invalid start: %w", err)
	}
	endH, endM, err := parseHour(parts[1])
	if err != nil {
		return AllowedHours{}, fmt.Errorf("invalid end: %w", err)
	}
	if startH*60+startM >= endH*60+endM {
		return AllowedHours{}, fmt.Errorf("start must be before end")
	}
	return AllowedHours{Start: startH, StartMinute: startM, End: endH, EndMinute: endM}, nil
}

// parseHour parses a single hour value in 24-hour (e.g. "18") or 12-hour
// am/pm format (e.g. "8am", "6:30pm").
func parseHour(s string) (hour, minute int, err error) {
	s = strings.TrimSpace(strings.ToLower(s))
	var isPM, isAM bool
	if strings.HasSuffix(s, "pm") {
		isPM = true
		s = s[:len(s)-2]
	} else if strings.HasSuffix(s, "am") {
		isAM = true
		s = s[:len(s)-2]
	}

	if i := strings.Index(s, ":"); i >= 0 {
		minute, err = strconv.Atoi(s[i+1:])
		if err != nil || minute < 0 || minute > 59 {
			return 0, 0, fmt.Errorf("invalid minutes in %q", s)
		}
		s = s[:i]
	}

	hour, err = strconv.Atoi(s)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid hour %q", s)
	}

	if isPM || isAM {
		if hour < 1 || hour > 12 {
			return 0, 0, fmt.Errorf("hour %d out of range for am/pm", hour)
		}
		if isPM && hour != 12 {
			hour += 12
		} else if isAM && hour == 12 {
			hour = 0
		}
	} else {
		if hour < 0 || hour > 23 {
			return 0, 0, fmt.Errorf("hour %d out of range", hour)
		}
	}
	return hour, minute, nil
}

// formatHour formats an hour+minute pair as a human-readable am/pm string,
// omitting :00 when minutes are zero (e.g. "8am", "6:30pm").
func formatHour(h, m int) string {
	suffix := "am"
	if h >= 12 {
		suffix = "pm"
		if h > 12 {
			h -= 12
		}
	}
	if h == 0 {
		h = 12
	}
	if m == 0 {
		return fmt.Sprintf("%d%s", h, suffix)
	}
	return fmt.Sprintf("%d:%02d%s", h, m, suffix)
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
