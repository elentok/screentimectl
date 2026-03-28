# Implementation Plan

## Tech Stack

- **Language:** Go
- **Telegram:** `go-telegram-bot-api/telegram-bot-api/v5`
- **YAML:** `gopkg.in/yaml.v3`
- **HTTP:** stdlib `net/http`
- **CLI:** stdlib `os.Args` subcommands (setup, doctor, logs, run)
- **Build:** `go build`

## Project Structure

```
screentimectl/
├── main.go              # entrypoint, subcommand dispatch
├── config.go            # config loading from /etc/screentimectl/config.yaml
├── timekpr.go           # timekpra CLI wrapper (exec)
├── telegram.go          # bot setup, command parsing, message sending
├── http.go              # HTTP API server
├── setup.go             # `screentimectl setup` command
├── doctor.go            # `screentimectl doctor` command
├── go.mod
├── go.sum
└── docs/
    ├── prd.md
    └── initial-implementation.md
```

## Implementation Steps

### Step 1: Project scaffolding

- `go mod init screentimectl`
- Add dependencies
- Create `main.go` with subcommand dispatch (`run`, `setup`, `doctor`, `logs`)

### Step 2: Config

- Parse `/etc/screentimectl/config.yaml`
- Struct:
  ```go
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
  ```
- Validate: bot token present, at least one user, at least one chat ID

### Step 3: timekpra wrapper

- Functions that shell out to `timekpra`:
  - `GetUserTime(user string) (remaining time, used time, error)` — parse `timekpra --userinfo <user>`
  - `AddTime(user string, minutes int) error` — `sudo timekpra --settimeleft <user> + <seconds>`
  - `SetTime(user string, minutes int) error` — `sudo timekpra --settimeleft <user> <seconds>`
- All commands run via `exec.Command` with `sudo`
- Parse timekpra output to extract remaining/used time

### Step 4: Telegram bot

- Long-poll loop using `tgbotapi.NewUpdate`
- Filter incoming messages: only process from `allowed_chat_ids`
- Command parsing:
  - `/give {user} {duration}` → validate user in config, parse duration (e.g. `15`, `15m`, `1h`), call `AddTime`, reply with new remaining
  - `/lock {user}` → call `SetTime(user, 0)`, reply confirmation
  - `/lock {user} {duration}` → call `SetTime(user, minutes)`, reply with remaining
  - `/status {user}` → call `GetUserTime`, reply with remaining + used
- Unknown user → reply `"Unknown user: X"`
- Errors → reply `"Failed to apply command: ..."`
- Helper: `sendMessage(chatID, text)` for sending Telegram messages (used by both bot commands and HTTP handler)

### Step 5: HTTP API

- `POST /request-more-time?user=guy&minutes=15`
- Validate user exists in config
- Get current usage via `GetUserTime`
- Send Telegram message to all `allowed_chat_ids`:
  `"{User} has already used the computer for {used}. {User} is asking for more time (suggested: {minutes}m)"`
- Respond 200 OK
- Listen on `config.Server.ListenAddr` (default `127.0.0.1:3847`)

### Step 6: `run` command (main daemon)

- Start Telegram bot polling in a goroutine
- Start HTTP server in a goroutine
- Block on signal (SIGINT/SIGTERM) for graceful shutdown

### Step 7: `setup` command

- Must run as root
- Create system user `screentimectl` (if not exists)
- Create config dir `/etc/screentimectl/` with example config
- Create sudoers rule `/etc/sudoers.d/screentimectl` allowing the system user to run `timekpra` without password
- Install systemd service file to `/etc/systemd/system/screentimectl.service`
- `systemctl daemon-reload`

### Step 8: `doctor` command

- Check timekpra binary exists → `[OK]`/`[FAIL]`
- Check config file exists and parses → `[OK]`/`[FAIL]`
- Check systemd service exists → `[OK]`/`[FAIL]`
- Check sudoers rule exists → `[OK]`/`[FAIL]`
- Check each configured user exists on system → `[OK]`/`[FAIL]`
- Check Telegram bot token is valid (getMe API call) → `[OK]`/`[FAIL]`

### Step 9: `logs` command

- Exec `journalctl -u screentimectl -f` (follow mode)

## Duration Parsing

Accept formats: `15` (minutes), `15m` (minutes), `1h` (hours), `1h30m`. Default unit is minutes.

## Error Handling

- Config errors → exit with message
- timekpra errors → return error string to Telegram
- Telegram API errors → log and continue
- HTTP errors → log and return 500

## Systemd Service File

```ini
[Unit]
Description=screentimectl daemon
After=network.target

[Service]
Type=simple
User=screentimectl
ExecStart=/usr/local/bin/screentimectl run
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```
