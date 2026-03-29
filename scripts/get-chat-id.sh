#!/bin/bash
set -euo pipefail

read -rp "Bot token: " TOKEN

if [[ -z "$TOKEN" ]]; then
    echo "Error: token cannot be empty" >&2
    exit 1
fi

echo ""
echo "Send a message to your bot first, then press Enter to fetch updates."
read -r

curl -s "https://api.telegram.org/bot${TOKEN}/getUpdates" | python3 -m json.tool
