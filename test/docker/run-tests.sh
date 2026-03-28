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

output=$(screentimectl setup 2>&1)
rc=$?

if [[ $rc -eq 0 ]]; then
    pass "setup exits 0"
else
    fail "setup exits 0 (got $rc)"
    echo "$output"
fi

assert_file /etc/screentimectl/config.yaml "config file created"
assert_file /etc/sudoers.d/screentimectl   "sudoers rule created"
assert_file /etc/systemd/system/screentimectl.service "systemd service created"

if id screentimectl &>/dev/null; then
    pass "system user created"
else
    fail "system user created"
fi

# --- Test: doctor ---

echo ""
echo "=== screentimectl doctor ==="

# Write a config that doctor can parse (bot token will fail validation against Telegram API, which is expected)
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
EOF

# Create the test user so the "system user exists" check passes
useradd --system --no-create-home --shell /usr/sbin/nologin testuser 2>/dev/null || true

doctor_output=$(screentimectl doctor 2>&1)
echo "$doctor_output"

# Checks that should pass
for check in "timekpra binary exists" "config file exists and parses" "systemd service installed" "sudoers rule installed" 'system user "testuser" exists'; do
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

# --- Summary ---

echo ""
echo "=== Results: $PASS passed, $FAIL failed ==="
if [[ $FAIL -gt 0 ]]; then
    exit 1
fi
