# screentimectl PRD (MVP)

## Overview
screentimectl is a local daemon that enables parents to remotely control and extend screen time on Linux machines via Telegram.

It integrates with timekpr-next for enforcement and exposes a local HTTP API for kids to request more time.

---

## Goals
- Remote control via Telegram
- Per-machine daemon
- Multiple users per machine
- Local HTTP API for requests
- Simple Ubuntu + systemd setup

---

## Architecture
- One daemon per machine
- One Telegram bot per machine
- Uses timekpra CLI
- Stateless

---

## Configuration
Path: /etc/screentimectl/config.yaml

Example:

machine_name: "Guy-PC"

telegram:
  bot_token: "TOKEN"
  allowed_chat_ids:
    - 111111111

server:
  listen_addr: "127.0.0.1:3847"

users:
  - name: "guy"
  - name: "guest"

---

## Commands

/give {user} {duration}
→ adds time

/lock {user}
→ sets time to 0

/lock {user} {duration}
→ sets remaining time

/status {user}

---

## Responses

"Guy now has 23 minutes remaining"
"Unknown user: X"
"Failed to apply command: ..."

---

## HTTP API

POST /request-more-time
POST /request-more-time?user=guy&minutes=15

Telegram message:
"Guy has already used the computer for 3h 2m. Guy is asking for more time (suggested: 15m)"

---

## Setup

Command:
screentimectl setup

Creates:
- system user
- config dir
- sudoers rule
- systemd service

---

## Doctor

Command:
screentimectl doctor

Outputs checks:
[OK]/[FAIL]

---

## Logging

journalctl -u screentimectl

CLI:
screentimectl logs

---

## Systemd

/etc/systemd/system/screentimectl.service

---

## Install

1. Copy binary
2. Run:
   sudo screentimectl setup

---

## Milestones

MVP:
- Telegram commands
- HTTP API
- setup/doctor

Future:
- rate limiting
- error hiding
- Telegram buttons
