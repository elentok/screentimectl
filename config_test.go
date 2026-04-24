package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigDefaultsTTSModel(t *testing.T) {
	cfg := loadTestConfig(t, `
machine_name: "test"
telegram:
  bot_token: "token"
  allowed_chat_ids: [123]
users:
  - name: "bob"
`)
	if got, want := cfg.TTSModel(), defaultTTSModel; got != want {
		t.Fatalf("TTSModel = %q, want %q", got, want)
	}
}

func TestLoadConfigCustomTTSModel(t *testing.T) {
	cfg := loadTestConfig(t, `
machine_name: "test"
telegram:
  bot_token: "token"
  allowed_chat_ids: [123]
tts:
  model: "en_US-ryan-medium"
users:
  - name: "bob"
`)
	if got, want := cfg.TTSModel(), "en_US-ryan-medium"; got != want {
		t.Fatalf("TTSModel = %q, want %q", got, want)
	}
}

func loadTestConfig(t *testing.T, content string) *Config {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	return cfg
}
