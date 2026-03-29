#!/bin/bash
set -euo pipefail

if [[ $# -ne 1 ]]; then
    echo "Usage: $0 <hostname>" >&2
    exit 1
fi

HOST="$1"
BINARY="screentimectl"
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

echo "Building for linux/amd64..."
GOOS=linux GOARCH=amd64 go build -o "$TMPDIR/$BINARY" .

echo "Uploading to $HOST..."
scp "$TMPDIR/$BINARY" "$HOST:~/$BINARY"

echo "Installing on $HOST..."
ssh -t "$HOST" "sudo cp -f ~/$BINARY /usr/local/bin/$BINARY && sudo chmod +x /usr/local/bin/$BINARY && rm ~/$BINARY && sudo systemctl restart screentimectl"

echo "Done."
echo

echo "Showing Logs..."
ssh -t "$HOST" "screentimectl logs"

