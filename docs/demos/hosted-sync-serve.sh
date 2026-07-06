#!/bin/bash
# ponytail: backgrounding + a pidfile lives here instead of inline in the tape
# because VHS Type lines handle `&` and command substitution poorly.
set -e
STORE_DIR="$(mktemp -d)"
LOG_FILE="$STORE_DIR/hosted-sync.log"
PID_FILE="$STORE_DIR/hosted-sync.pid"
devspace hosted serve --addr 127.0.0.1:8791 --store "$STORE_DIR" --token demo-token \
  >"$LOG_FILE" 2>&1 &
echo $! >"$PID_FILE"
if [ -n "${DEMO_WS:-}" ]; then
  cat > "$DEMO_WS/hosted-sync-demo.env" <<EOF
export HOSTED_SYNC_DEMO_LOG_FILE="$LOG_FILE"
export HOSTED_SYNC_DEMO_PID_FILE="$PID_FILE"
export HOSTED_SYNC_DEMO_STORE_DIR="$STORE_DIR"
EOF
fi
sleep 2
