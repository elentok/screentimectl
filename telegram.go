package main

import (
	"fmt"
	"log"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Bot struct {
	api *tgbotapi.BotAPI
	cfg *Config
}

func newBot(cfg *Config) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(cfg.Telegram.BotToken)
	if err != nil {
		return nil, fmt.Errorf("creating telegram bot: %w", err)
	}
	log.Printf("telegram: authorized as @%s", api.Self.UserName)
	return &Bot{api: api, cfg: cfg}, nil
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
		if update.Message == nil || !update.Message.IsCommand() {
			continue
		}
		msg := update.Message
		if !b.cfg.isAllowedChat(msg.Chat.ID) {
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
	default:
		b.send(chatID, "Unknown command. Use /give, /lock, or /status.")
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

	ut, err := AddTime(user, minutes)
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

	ut, err := SetTime(user, minutes)
	if err != nil {
		b.send(chatID, fmt.Sprintf("Failed to apply command: %v", err))
		return
	}

	if minutes == 0 {
		b.send(chatID, fmt.Sprintf("%s has been locked out", capitalize(user)))
	} else {
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

	ut, err := GetUserTime(user)
	if err != nil {
		b.send(chatID, fmt.Sprintf("Failed to apply command: %v", err))
		return
	}

	b.send(chatID, fmt.Sprintf("%s has %s remaining (used %s today)", capitalize(user), ut.RemainingStr(), ut.UsedStr()))
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
