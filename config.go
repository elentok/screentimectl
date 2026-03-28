package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

const configPath = "/etc/screentimectl/config.yaml"

type Config struct {
	MachineName string         `yaml:"machine_name"`
	Telegram    TelegramConfig `yaml:"telegram"`
	Server      ServerConfig   `yaml:"server"`
	Users       []UserConfig   `yaml:"users"`
}

type TelegramConfig struct {
	BotToken       string  `yaml:"bot_token"`
	AllowedChatIDs []int64 `yaml:"allowed_chat_ids"`
}

type ServerConfig struct {
	ListenAddr string `yaml:"listen_addr"`
}

type UserConfig struct {
	Name string `yaml:"name"`
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	if cfg.Telegram.BotToken == "" {
		return nil, fmt.Errorf("telegram.bot_token is required")
	}
	if len(cfg.Telegram.AllowedChatIDs) == 0 {
		return nil, fmt.Errorf("telegram.allowed_chat_ids must have at least one entry")
	}
	if len(cfg.Users) == 0 {
		return nil, fmt.Errorf("users must have at least one entry")
	}
	if cfg.Server.ListenAddr == "" {
		cfg.Server.ListenAddr = "127.0.0.1:3847"
	}

	return &cfg, nil
}

func (c *Config) isAllowedChat(chatID int64) bool {
	for _, id := range c.Telegram.AllowedChatIDs {
		if id == chatID {
			return true
		}
	}
	return false
}

func (c *Config) isValidUser(name string) bool {
	for _, u := range c.Users {
		if u.Name == name {
			return true
		}
	}
	return false
}
