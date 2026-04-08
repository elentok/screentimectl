#!/bin/bash
set -euo pipefail

PASS=0
FAIL=0

pass() { echo "PASS: $1"; PASS=$((PASS + 1)); }
fail() { echo "FAIL: $1"; FAIL=$((FAIL + 1)); }

assert_file() {
    if [[ -e "$1" ]]; then pass "$2"; else fail "$2 ($1 not found)"; fi
}

# --- Test: setup ---

echo "=== screentimectl setup ==="

if output=$(screentimectl setup 2>&1); then
    pass "setup exits 0"
else
    rc=$?
    fail "setup exits 0 (got $rc)"
    echo "$output"
fi

assert_file /etc/screentimectl/config.yaml "config file created"
assert_file /etc/sudoers.d/screentimectl   "sudoers rule created"
assert_file /etc/systemd/system/screentimectl.service "systemd service created"
assert_file /var/lib/screentimectl         "data directory created"
assert_file /usr/local/bin/screentimectl-tray "tray indicator installed"

if id screentimectl &>/dev/null; then
    pass "system user created"
else
    fail "system user created"
fi

# Check PAM rule was installed
if grep -q "screentimectl" /etc/pam.d/gdm-password; then
    pass "PAM rule installed"
else
    fail "PAM rule installed"
fi

# Check sudoers contains loginctl (not timekpra)
if grep -q "loginctl" /etc/sudoers.d/screentimectl; then
    pass "sudoers has loginctl rule"
else
    fail "sudoers has loginctl rule"
fi

# --- Test: doctor ---

echo ""
echo "=== screentimectl doctor ==="

# Write a config that doctor can parse
cat > /etc/screentimectl/config.yaml <<'EOF'
machine_name: "test-machine"

telegram:
  bot_token: "fake-token"
  allowed_chat_ids:
    - 123456789

server:
  listen_addr: "127.0.0.1:3847"

notifications:
  thresholds: [30, 15, 5, 1]

users:
  - name: "testuser"
    daily_limit_minutes: 300
    allowed_hours:
      start: 8
      end: 18
EOF

# Fix ownership after writing config
chown screentimectl:screentimectl /etc/screentimectl/config.yaml

# Create the test user so the "system user exists" check passes
useradd --system --no-create-home --shell /usr/sbin/nologin testuser 2>/dev/null || true

doctor_output=$(screentimectl doctor 2>&1)
echo "$doctor_output"

# Checks that should pass
for check in "loginctl binary exists" "notify-send binary exists" "espeak-ng binary exists" "python3 binary exists" "tray AppIndicator Python bindings available" "config file exists and parses" "systemd service installed" "sudoers rule installed" "config file owned by screentimectl" "data directory exists" "tray indicator installed" "PAM rule installed" 'system user "testuser" exists'; do
    if echo "$doctor_output" | grep -qF "[OK]   $check"; then
        pass "doctor: $check"
    else
        fail "doctor: $check"
    fi
done

# Telegram check should fail (fake token)
if echo "$doctor_output" | grep -qF "[FAIL] telegram bot token valid"; then
    pass "doctor: telegram token correctly fails with fake token"
else
    fail "doctor: telegram token correctly fails with fake token"
fi

# --- Test: check-login ---

echo ""
echo "=== screentimectl check-login ==="

# Create usage store directory
mkdir -p /var/lib/screentimectl

# Should allow login for unmanaged user
if PAM_USER=nobody screentimectl check-login; then
    pass "check-login allows unmanaged user"
else
    fail "check-login allows unmanaged user"
fi

# Should allow login for managed user within hours with time remaining
cat > /etc/screentimectl/config.yaml <<'EOF'
machine_name: "test-machine"
telegram:
  bot_token: "fake-token"
  allowed_chat_ids:
    - 123456789
server:
  listen_addr: "127.0.0.1:3847"
users:
  - name: "testuser"
    daily_limit_minutes: 300
    allowed_hours:
      start: 0
      end: 23
EOF

if PAM_USER=testuser screentimectl check-login; then
    pass "check-login allows user with time remaining"
else
    fail "check-login allows user with time remaining"
fi

# Should deny login when no time remaining
# Write usage file with all time used up
mkdir -p /var/lib/screentimectl
cat > /etc/screentimectl/config.yaml <<'EOF'
machine_name: "test-machine"
telegram:
  bot_token: "fake-token"
  allowed_chat_ids:
    - 123456789
server:
  listen_addr: "127.0.0.1:3847"
users:
  - name: "testuser"
    daily_limit_minutes: 1
    allowed_hours:
      start: 0
      end: 23
EOF

today=$(date +%Y-%m-%d)
cat > /var/lib/screentimectl/usage.json <<EOF
{
  "date": "$today",
  "users": {
    "testuser": {
      "used_seconds": 99999,
      "bonus_seconds": 0
    }
  }
}
EOF

if PAM_USER=testuser screentimectl check-login 2>/dev/null; then
    fail "check-login denies user with no time remaining"
else
    pass "check-login denies user with no time remaining"
fi

# --- Summary ---

echo ""
echo "=== Results: $PASS passed, $FAIL failed ==="
if [[ $FAIL -gt 0 ]]; then
    exit 1
fi
