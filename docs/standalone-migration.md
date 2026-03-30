# Standalone Migration: Replace timekpr-next with loginctl

## Context

timekpr-next doesn't properly pause its timer when the screen is locked, so the child loses screen time while away from the computer. We're replacing it with direct session tracking via `loginctl`, giving us full control over what counts as active time.

## Overview

- Replace timekpr CLI calls with a polling loop that checks `loginctl` session state
- Track usage ourselves in `/var/lib/screentimectl/usage.json`
- Enforce login via PAM (`pam_exec` calling `screentimectl check-login`)
- Lock accounts via `passwd -l` / `passwd -u` when time runs out
- Send desktop notifications (`notify-send`) and TTS alerts (`espeak-ng`) at configurable thresholds
- Add `screentimectl status`, `screentimectl ask`, and `screentimectl check-login` commands
- Add Telegram `/hours` command to adjust allowed login hours

The Telegram bot interface, HTTP API, and config structure remain the same (with additions).

## Config Changes

```yaml
users:
  - name: "bob"
    daily_limit_minutes: 300    # NEW (default: 300)
    allowed_hours:              # NEW - time window for login
      start: 8                  # 8am
      end: 18                   # 6pm

notifications:                  # NEW section
  thresholds: [30, 15, 5, 1]   # minutes remaining (default: [30, 15, 5, 1])
```

## Login Enforcement

### PAM Integration

`screentimectl setup` installs a PAM rule in `/etc/pam.d/gdm-password` (and other relevant PAM services):

```
auth required pam_exec.so /usr/local/bin/screentimectl check-login
```

### `screentimectl check-login` command

Called by PAM at login time. Reads `$PAM_USER`, checks:

1. Is the user in the config? If not, exit 0 (allow — not a managed user).
2. Is there an active override in the usage store? If yes, exit 0.
3. Is the current time within `allowed_hours`? If not, exit 1 (deny).
4. Does the user have remaining time? If not, exit 1 (deny).
5. Otherwise, exit 0 (allow).

### Account Locking

When time runs out, the session manager:
1. Sends a final notification + TTS warning
2. `loginctl terminate-session` to end all sessions
3. `passwd -l bob` to prevent re-login

Account is unlocked (`passwd -u bob`) when:
- A new day starts and the time window opens
- Parents use `/give` to add time (sets an override flag + unlocks)

### Overrides

When parents use `/give bob 30m` outside the allowed hours:
- `bonus_seconds` is added to the usage store
- An `override_until` timestamp is set (now + bonus duration)
- Account is unlocked via `passwd -u`
- `check-login` sees the override and allows login regardless of `allowed_hours`

### Telegram `/hours` command

```
/hours bob 8-20    → Set allowed hours to 8am-8pm
/hours bob         → Show current allowed hours
```

Updates the config file on disk and reloads in memory. This way parents can adjust the window without SSH access.

## New Files

### `usage.go` — Persistent daily usage store

Stores usage at `/var/lib/screentimectl/usage.json`:

```json
{
  "date": "2026-03-29",
  "users": {
    "bob": {
      "used_seconds": 3600,
      "bonus_seconds": 900,
      "override_until": "2026-03-29T20:30:00Z",
      "notified_thresholds": [30]
    }
  }
}
```

- `UsageStore` struct with mutex for concurrent access
- Resets automatically when the date changes (clears used_seconds, bonus_seconds, override)
- Remaining = (daily_limit + bonus) - used
- Atomic writes (write to temp, rename)

### `session.go` — Session manager (replaces timekpr)

Core polling loop (every 10s):
1. Reset usage if new day; unlock accounts if within allowed hours
2. For each configured user, find sessions via `loginctl list-sessions`
3. Check if any session is active: `loginctl show-session ID -p Active -p IdleHint -p LockedHint`
4. If active, increment `used_seconds` by poll interval
5. Check notification thresholds, fire `notify-send` + `espeak-ng` if crossed
6. If remaining <= 0: terminate sessions, lock account with `passwd -l`
7. If outside `allowed_hours` and no override: terminate sessions, lock account
8. Save to disk

Notifications target the user's session:
```
sudo -u bob XDG_RUNTIME_DIR=/run/user/UID notify-send "Screen Time" "15 minutes remaining"
sudo -u bob XDG_RUNTIME_DIR=/run/user/UID espeak-ng "You have 15 minutes of screen time remaining"
```

Exposes methods used by Telegram/HTTP handlers:
- `GetUserTime(user) (UserTime, error)`
- `AddTime(user, minutes) (UserTime, error)` — adds bonus_seconds, sets override, unlocks account
- `SetTime(user, minutes) (UserTime, error)` — sets remaining to exact value
- `LockUser(user) error` — terminates sessions + `passwd -l`
- `UnlockUser(user) error` — `passwd -u`

## Modified Files

### `timekpr.go`

- Remove: `GetUserTime`, `AddTime`, `SetTime`, `parseUserInfo`, `lookupFirst`, `reTimekprField`
- Keep: `UserTime` struct, `RemainingStr()`, `UsedStr()`, `formatDuration()`

### `telegram.go`

- Add `mgr *SessionManager` to `Bot` struct
- Update `newBot(cfg, mgr)` signature
- Handlers call `b.mgr.GetUserTime()`, `b.mgr.AddTime()`, `b.mgr.SetTime()`
- `/lock` with 0 minutes also calls `b.mgr.LockUser()`
- Add `/hours` command handler: parse `user start-end`, update config, reply with confirmation

### `http.go`

- Pass `*SessionManager` to `startHTTPServer` and handlers
- Add `GET /status?user=X` endpoint returning JSON `{"remaining_seconds": N, "used_seconds": N}`

### `main.go`

- Wire up: create `UsageStore` -> `SessionManager` -> pass to bot and HTTP
- Start `mgr.Run()` goroutine in `runDaemon()`
- Add `status` command: resolve current user, `GET http://localhost:3847/status?user=USER`, print remaining/used
- Add `ask` command: resolve current user, `POST http://localhost:3847/request-more-time?user=USER`, print confirmation
- Add `hours` command: `screentimectl hours {user} [start-end]` — view or update allowed hours in config
- Add `check-login` command: read `$PAM_USER`, check usage store + config, exit 0 or 1

### `setup.go`

- Add step: create `/var/lib/screentimectl/` owned by screentimectl
- Add step: install PAM rule in `/etc/pam.d/gdm-password`
- Update sudoers:
  ```
  screentimectl ALL=(ALL) NOPASSWD: /usr/bin/loginctl
  screentimectl ALL=(ALL) NOPASSWD: /usr/bin/passwd -l *, /usr/bin/passwd -u *
  screentimectl ALL=(ALL:ALL) NOPASSWD: /usr/bin/notify-send, /usr/bin/espeak-ng
  ```
- Update example config with `daily_limit_minutes`, `allowed_hours`, and `notifications`

### `doctor.go`

- Remove: timekpra binary check
- Add: loginctl, notify-send, espeak-ng binary checks
- Add: `/var/lib/screentimectl/` exists and owned by screentimectl
- Add: PAM rule installed check
- Add: config has `daily_limit_minutes` and `allowed_hours` for each user

## New CLI Commands

### `screentimectl status`

For the child to check their own time. Determines current user via `os.Getenv("USER")`, hits local HTTP API, prints:

```
You have 1h 15m remaining (used 3h 45m today)
Allowed hours: 8am - 6pm
```

### `screentimectl ask`

For the child to request more time. Determines current user, hits `/request-more-time`, prints:

```
Request sent! Your parents have been notified.
```

### `screentimectl hours {user} [start-end]`

View or set allowed hours for a user. Can be run from SSH or locally.

```
screentimectl hours bob          → Allowed hours for bob: 8am - 6pm
screentimectl hours bob 8-20     → Updated allowed hours for bob: 8am - 8pm
```

Updates the config file on disk. The daemon reloads config periodically or on SIGHUP.

### `screentimectl check-login`

Called by PAM. Reads `$PAM_USER`, checks config + usage store, exits 0 (allow) or 1 (deny). No output on success; on denial, prints reason to stderr (shown to user at login screen).

## Implementation Order

1. `config.go` — add new config fields (backward compatible)
2. `usage.go` + `usage_test.go` — usage store, testable in isolation
3. `session.go` + `session_test.go` — session manager, notifications, TTS, account lock/unlock
4. `timekpr.go` — remove timekpr functions, keep UserTime (atomic with steps 5-7)
5. `telegram.go` — update Bot to use SessionManager, add `/hours`
6. `http.go` — update handlers, add /status endpoint
7. `main.go` — wire everything, add ask/status/check-login commands
8. `setup.go` — sudoers, data dir, PAM rule, example config
9. `doctor.go` — replace checks
10. `test/docker/` — update Dockerfile, stubs, assertions

Steps 4-7 must be done together (they break the old API).

## Docker Test Updates

- Remove `fake-timekpra`, add `fake-loginctl` stub
- Add `fake-notify-send`, `fake-espeak-ng`, `fake-passwd` stubs
- Test `setup` creates `/var/lib/screentimectl/`
- Test `check-login` allows/denies correctly
- Test `status` reads from usage store
- Test `doctor` checks new binaries and PAM rule

## Verification

1. `go test ./...` — unit tests for usage store, check-login logic, duration parsing
2. `docker build && docker run` — integration tests for setup/doctor/check-login
3. Deploy to server, run `screentimectl doctor` to validate
4. Test manually: lock screen, verify timer pauses; unlock, verify it resumes
5. Test time expiry: terminates session and locks account
6. Test `check-login` blocks login when locked, allows when unlocked
7. Test `/give` outside allowed hours: sets override, unlocks account, allows login
8. Test `/hours` updates the time window
9. Test `screentimectl status` and `screentimectl ask` as the child user
10. Test notifications and TTS fire at thresholds
