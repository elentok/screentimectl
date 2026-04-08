package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadConfigDefaultsTTSVoices(t *testing.T) {
	cfg := loadTestConfig(t, `
machine_name: "test"
telegram:
  bot_token: "token"
  allowed_chat_ids: [123]
users:
  - name: "bob"
`)

	want := []string{"gmw/en-US", "gmw/en-US-nyc", "gmw/en"}
	if got := cfg.TTSVoices(); !reflect.DeepEqual(got, want) {
		t.Fatalf("TTSVoices = %v, want %v", got, want)
	}
}

func TestLoadConfigCustomTTSVoiceKeepsDefaultFallbacks(t *testing.T) {
	cfg := loadTestConfig(t, `
machine_name: "test"
telegram:
  bot_token: "token"
  allowed_chat_ids: [123]
tts:
  voice: "gmw/en"
users:
  - name: "bob"
`)

	want := []string{"gmw/en", "gmw/en-US-nyc"}
	if got := cfg.TTSVoices(); !reflect.DeepEqual(got, want) {
		t.Fatalf("TTSVoices = %v, want %v", got, want)
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
