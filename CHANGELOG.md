# Changelog

## Unreleased

- Switched account lock/unlock from `passwd -l` / `passwd -u` to `chage -E 0` / `chage -E -1` to avoid unlock failures on accounts without a usable password hash.
- Added `screentimectl status --compact` and a GNOME AppIndicator tray helper installed by `setup` to show remaining screen time from the user's session.

## v0.3.0

Activity logging and status timeline.

### What changed

**Activity log** -- The daemon now tracks status transitions (active, locked, idle, offline) and writes them to per-user daily JSONL files at `/var/lib/screentimectl/log/{user}/YYYY-MM-DD.log`. Only transitions are logged, not every poll tick.

**Timeline in /status** -- The `/status` command (Telegram, CLI, and HTTP) now shows a timeline of the day's activity:
```
Today:
  08:00-10:30 (2h 30m) - active
  10:30-11:00 (30m) - locked
  11:00-14:30 (3h 30m) - active
```

**Shutdown logging** -- When the daemon receives SIGTERM (systemd stop or system poweroff), a `shutdown` entry is written to the activity log.

**Time grant notifications** -- When a parent grants more time via `/give`, the child now receives a desktop notification and TTS announcement with the updated remaining time.

### Upgrade notes

1. Run `sudo screentimectl setup` to create the new `/var/lib/screentimectl/log/` directory
2. Restart the service: `sudo systemctl restart screentimectl`

## v0.2.0

Replace timekpr-next with standalone session management.

### Why

timekpr-next continued counting screen time while the machine was locked, causing the child to lose time while away from the computer. This release removes the timekpr-next dependency entirely and manages screen time directly.

### What changed

**Session tracking** -- The daemon now polls `loginctl` every 10 seconds to detect active sessions. Time only counts when the session is active (not locked, not idle).

**Account locking** -- When time expires or the child is outside allowed hours, the screen is locked via `loginctl lock-session` and the account is locked via `passwd -l` to prevent re-login. Accounts are unlocked on day reset or when parents grant time.

**PAM integration** -- A new `check-login` command is installed as a PAM rule to prevent login when outside allowed hours or when no time remains. Parents can override this via `/give`.

**Notifications and TTS** -- Desktop notifications (`notify-send`) and spoken alerts (`espeak-ng`) fire at configurable thresholds (default: 30, 15, 5, 1 minutes remaining).

**New Telegram commands:**
- `/hours bob 8-20` -- view or set allowed login hours
- `/say bob message` -- speak a message to the child via TTS

**New CLI commands:**
- `screentimectl status` -- for the child to check their remaining time
- `screentimectl ask` -- for the child to request more time
- `screentimectl hours bob 8-20` -- view or set allowed hours from SSH
- `screentimectl say bob message` -- speak a message via TTS
- `screentimectl check-login` -- PAM login check

**Configuration:**
- Added `daily_limit_minutes` and `allowed_hours` per user
- Added `notifications.thresholds` for alert timing
- Usage data stored in `/var/lib/screentimectl/usage.json`

### Removed

- timekpr-next dependency (no longer needed)

### Upgrade notes

1. Install `espeak-ng` and `libnotify-bin` if not already present
2. Run `sudo screentimectl setup` to install new sudoers rules, PAM rule, and data directory
3. Update `/etc/screentimectl/config.yaml` to add `daily_limit_minutes` and `allowed_hours` per user (defaults to 300 minutes and 8am-6pm if omitted)
4. Restart the service: `sudo systemctl restart screentimectl`
5. timekpr-next can be uninstalled if no longer needed
