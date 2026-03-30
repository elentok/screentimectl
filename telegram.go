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
	if len(args) != 2 {
		b.send(chatID, "Usage: /give {user} {duration}")
		return
	}
	user, durStr := args[0], args[1]

	if !b.cfg.isValidUser(user) {
		b.send(chatID, fmt.Sprintf("Unknown user: %s", user))
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
	if len(args) == 0 || len(args) > 2 {
		b.send(chatID, "Usage: /lock {user} [duration]")
		return
	}
	user := args[0]

	if !b.cfg.isValidUser(user) {
		b.send(chatID, fmt.Sprintf("Unknown user: %s", user))
		return
	}

	minutes := 0
	if len(args) == 2 {
		var err error
		minutes, err = parseDurationMinutes(args[1])
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
	if len(args) != 1 {
		b.send(chatID, "Usage: /status {user}")
		return
	}
	user := args[0]

	if !b.cfg.isValidUser(user) {
		b.send(chatID, fmt.Sprintf("Unknown user: %s", user))
		return
	}

	ut, err := b.mgr.GetUserTime(user)
	if err != nil {
		b.send(chatID, fmt.Sprintf("Failed to apply command: %v", err))
		return
	}

	u := b.cfg.getUser(user)
	b.send(chatID, fmt.Sprintf("%s has %s remaining (used %s today)\nAllowed hours: %dam - %dpm",
		capitalize(user), ut.RemainingStr(), ut.UsedStr(),
		u.AllowedHours.Start, u.AllowedHours.End%12))
}

func (b *Bot) handleHours(chatID int64, args []string) {
	if len(args) < 1 || len(args) > 2 {
		b.send(chatID, "Usage: /hours {user} [start-end]")
		return
	}
	user := args[0]

	if !b.cfg.isValidUser(user) {
		b.send(chatID, fmt.Sprintf("Unknown user: %s", user))
		return
	}

	u := b.cfg.getUser(user)

	if len(args) == 1 {
		b.send(chatID, fmt.Sprintf("Allowed hours for %s: %dam - %dpm",
			capitalize(user), u.AllowedHours.Start, u.AllowedHours.End%12))
		return
	}

	start, end, err := parseHoursRange(args[1])
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
	if len(args) < 2 {
		b.send(chatID, "Usage: /say {user} {message}")
		return
	}
	user := args[0]

	if !b.cfg.isValidUser(user) {
		b.send(chatID, fmt.Sprintf("Unknown user: %s", user))
		return
	}

	msg := strings.Join(args[1:], " ")
	sendTTS(user, msg)
	sendNotification(user, msg)
	b.send(chatID, fmt.Sprintf("Sent to %s: %q", capitalize(user), msg))
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
