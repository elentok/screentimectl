# screentimectl

A daemon that lets parents remotely control screen time on Linux machines via Telegram. Tracks active session time via `loginctl`, enforces limits by locking the screen and account, and sends desktop notifications and TTS alerts when time is running low.

## Requirements

- Ubuntu (systemd + systemd-logind)
- A Telegram bot token (create one via [@BotFather](https://t.me/BotFather))
- Runtime apt dependencies are installed by `setup`: `sudo`, `libnotify-bin`, `espeak-ng`, `gnome-shell-extension-appindicator`, `python3-gi`, `gir1.2-gtk-3.0`, `gir1.2-ayatanaappindicator3-0.1`

## Install

1. Build or download the binary:
   ```sh
   go build -o screentimectl
   sudo cp screentimectl /usr/local/bin/
   ```

2. Run setup (creates system user, config, sudoers, PAM rule, and systemd service):
   ```sh
   sudo screentimectl setup
   ```

3. Edit the config:
   ```sh
   sudo nano /etc/screentimectl/config.yaml
   ```

4. Enable and start:
   ```sh
   sudo systemctl enable --now screentimectl
   ```

## Configuration

`/etc/screentimectl/config.yaml`:

```yaml
machine_name: "Bob-PC"

telegram:
  bot_token: "TOKEN"
  allowed_chat_ids:
    - 111111111   # get this from scripts/get-chat-id.sh

server:
  listen_addr: "127.0.0.1:3847"

notifications:
  thresholds: [30, 15, 5, 1]  # minutes remaining

users:
  - name: "bob"
    daily_limit_minutes: 300   # 5 hours
    allowed_hours:
      start: 8                 # 8am
      end: 18                  # 6pm
```

## Telegram Commands

| Command | Effect |
|---|---|
| `/give [bob] 30m` | Add 30 minutes to Bob's time |
| `/give [bob] 1h30m` | Add 1.5 hours |
| `/lock [bob]` | Lock Bob's screen and account immediately |
| `/unlock [bob] 15m` | Set Bob's remaining time to 15 minutes and allow login |
| `/status [bob]` | Show remaining time, used time, allowed hours, and activity timeline |
| `/hours [bob]` | Show Bob's allowed hours |
| `/hours [bob] 8-20` | Set allowed hours to 8am-8pm |
| `/say [bob] Time for dinner` | Speak a message to Bob via TTS |

Duration formats: `15`, `15m`, `1h`, `1h30m`.

The user argument can be omitted when there is one configured user, or when exactly one configured user is active.

`/lock [bob] 15m` still works as a compatibility alias for `/unlock [bob] 15m`, but `/unlock` is preferred.

Using `/give` outside allowed hours automatically creates a temporary override so the child can log in.

## User Commands

These commands are for the child to run on their own machine:

```sh
screentimectl status   # show remaining screen time, allowed hours, and today's activity
screentimectl status --compact  # show only remaining screen time
screentimectl ask      # request more time (notifies parents via Telegram)
screentimectl ask 30   # request 30 minutes specifically
```

## Admin Commands

```sh
screentimectl run          # start the daemon (normally via systemd)
screentimectl setup        # install system dependencies (run as root)
screentimectl doctor       # check configuration and dependencies
screentimectl logs         # tail the service logs
screentimectl give bob 30m # add 30 minutes for bob
screentimectl lock bob     # lock bob's screen and account immediately
screentimectl unlock bob 15m  # set bob's remaining time to 15 minutes and allow login
screentimectl status bob   # show bob's remaining time and activity timeline
screentimectl hours bob    # show allowed hours for bob
screentimectl hours bob 8-20  # set allowed hours
screentimectl say bob "Time for dinner"  # send a desktop notification and TTS message
```

For SSH/admin use, the user argument can be omitted for `give`, `lock`, `unlock`, `status`, `hours`, and `say` when there is one configured user, or when exactly one configured user is active.

## HTTP API

```sh
# Request more time (used by `screentimectl ask`)
curl -X POST "http://127.0.0.1:3847/request-more-time?user=bob&minutes=15"

# Check status (used by `screentimectl status`)
curl "http://127.0.0.1:3847/status?user=bob"
```

## How It Works

- The daemon polls `loginctl` every 10 seconds to check session state
- Time only counts when the session is active (not locked or idle)
- Daily usage is stored in `/var/lib/screentimectl/usage.json` and resets at midnight
- Activity transitions (active/locked/idle/offline) are logged to `/var/lib/screentimectl/log/{user}/YYYY-MM-DD.log`
- When time runs out: screen locks, account access is disabled via `chage -E 0`, and parents are notified
- When time is granted via `/give`, the child receives a desktop notification and TTS announcement
- `setup` installs `/usr/local/bin/screentimectl-tray` and autostarts it for configured users to show remaining time from `status --compact`
- Login is enforced via PAM (`pam_exec`) -- the child cannot log in outside allowed hours or with no time remaining
- Parents can grant time or adjust hours at any time via Telegram

## Deploy

```sh
./scripts/deploy.sh myserver.example.com
```

## Logs

```sh
journalctl -u screentimectl -f
```
