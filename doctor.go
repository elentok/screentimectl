package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func runDoctor() {
	cfg, cfgErr := loadConfig(configPath)

	check("timekpra binary exists", func() error {
		_, err := exec.LookPath("timekpra")
		return err
	})

	check("config file exists and parses", func() error {
		return cfgErr
	})

	check("systemd service installed", func() error {
		if _, err := os.Stat(servicePath); err != nil {
			return fmt.Errorf("%s not found", servicePath)
		}
		return nil
	})

	check("sudoers rule installed", func() error {
		if _, err := os.Stat(sudoersPath); err != nil {
			return fmt.Errorf("%s not found", sudoersPath)
		}
		return nil
	})

	if cfgErr == nil {
		for _, u2 := range cfg.Users {
			name := u2.Name
			check(fmt.Sprintf("system user %q exists", name), func() error {
				_, err := user.Lookup(name)
				return err
			})
		}

		check("telegram bot token valid", func() error {
			api, err := tgbotapi.NewBotAPI(cfg.Telegram.BotToken)
			if err != nil {
				return err
			}
			_ = api
			return nil
		})
	}
}

func check(name string, fn func() error) {
	err := fn()
	if err != nil {
		fmt.Printf("[FAIL] %s: %v\n", name, err)
	} else {
		fmt.Printf("[OK]   %s\n", name)
	}
}
