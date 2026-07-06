#!/bin/bash
# ponytail: backgrounding + a pidfile lives here instead of inline in the tape
# because VHS Type lines handle `&` and command substitution poorly.
set -e
STORE_DIR="$(mktemp -d)"
devspace hosted serve --addr 127.0.0.1:8791 --store "$STORE_DIR" --token demo-token \
  >/tmp/devspace-hosted-demo.log 2>&1 &
echo $! >/tmp/devspace-hosted-demo.pid
sleep 2
