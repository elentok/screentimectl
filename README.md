# screentimectl

A daemon that lets parents remotely control screen time on Linux machines via Telegram. Integrates with [timekpr-next](https://launchpad.net/timekpr-next) for enforcement and exposes a local HTTP API so kids can request more time.

## Requirements

- Ubuntu (systemd)
- [timekpr-next](https://launchpad.net/timekpr-next) installed
- A Telegram bot token (create one via [@BotFather](https://t.me/BotFather))

## Install

1. Build or download the binary:
   ```sh
   go build -o screentimectl
   sudo cp screentimectl /usr/local/bin/
   ```

2. Run setup (creates system user, config dir, sudoers rule, and systemd service):
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
    - 111111111   # get this from @userinfobot

server:
  listen_addr: "127.0.0.1:3847"

users:
  - name: "bob"
  - name: "guest"
```

## Telegram commands

| Command | Effect |
|---|---|
| `/give bob 30m` | Add 30 minutes to Bob's time |
| `/give bob 1h30m` | Add 1.5 hours |
| `/lock bob` | Lock Bob out immediately |
| `/lock bob 15m` | Set Bob's remaining time to 15 minutes |
| `/status bob` | Show remaining and used time |

Duration formats: `15`, `15m`, `1h`, `1h30m`.

## HTTP API

Kids can send a time request from the machine, which pings the Telegram chat:

```sh
curl -X POST "http://127.0.0.1:3847/request-more-time?user=bob&minutes=15"
```

This sends to Telegram:
> Bob has already used the computer for 3h 2m. Bob is asking for more time (suggested: 15m)

## Operations

```sh
screentimectl doctor   # check configuration and dependencies
screentimectl logs     # tail the service logs (wraps journalctl)
```

## Logs

```sh
journalctl -u screentimectl -f
```
