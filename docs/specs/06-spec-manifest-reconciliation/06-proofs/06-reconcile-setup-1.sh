#!/usr/bin/env bash
# Hidden setup (part 1) for the spec 06 reconcile demo tape.
# Two machines share a local bare manifest remote and diverge
# non-conflictingly: A adds apps/api and pushes, B adds apps/web locally.
# Mechanics lifted verbatim from the spec 06 task-04 demo script
# (.claude/jobs/ecc582af/tmp/demo-reconcile.sh), setup portion only.
# Run from the repo root. Writes /tmp/06-reconcile-demo-env.sh for the tape
# to source; never touches the real ~/.devspace.
set -euo pipefail

BIN="$(pwd)/bin/devspace"
DEMO="$(mktemp -d /tmp/devspace-reconcile-demo.XXXXXX)"
HOME_A="$DEMO/home-a"; HOME_B="$DEMO/home-b"
WS_A="$DEMO/machine-a/code"; WS_B="$DEMO/machine-b/code"
REMOTE="$DEMO/manifest-remote.git"

mkdir -p "$WS_A/apps/shared" "$WS_B"
git -C "$WS_A/apps/shared" init -q -b main

DEVSPACE_HOME="$HOME_A" "$BIN" init --workspace "$WS_A" >/dev/null
DEVSPACE_HOME="$HOME_A" "$BIN" workspace remote create local "$REMOTE" >/dev/null
DEVSPACE_HOME="$HOME_A" "$BIN" scan >/dev/null
DEVSPACE_HOME="$HOME_A" "$BIN" workspace push >/dev/null

DEVSPACE_HOME="$HOME_B" "$BIN" init --workspace "$WS_B" >/dev/null
DEVSPACE_HOME="$HOME_B" "$BIN" workspace remote set "$REMOTE" >/dev/null
DEVSPACE_HOME="$HOME_B" "$BIN" workspace pull >/dev/null

mkdir -p "$WS_A/apps/api"
git -C "$WS_A/apps/api" init -q -b main
DEVSPACE_HOME="$HOME_A" "$BIN" scan >/dev/null
DEVSPACE_HOME="$HOME_A" "$BIN" workspace push >/dev/null

mkdir -p "$WS_B/apps/web"
git -C "$WS_B/apps/web" init -q -b main
DEVSPACE_HOME="$HOME_B" "$BIN" scan >/dev/null

ENV_FILE=/tmp/06-reconcile-demo-env.sh
if [ -e "$ENV_FILE" ] || [ -L "$ENV_FILE" ]; then
  rm -f "$ENV_FILE"
fi
(umask 077
cat > "$ENV_FILE" <<EOF
export BIN="$BIN"
export DEMO="$DEMO"
export HOME_A="$HOME_A"
export HOME_B="$HOME_B"
export WS_A="$WS_A"
export WS_B="$WS_B"
export REMOTE="$REMOTE"
export DEVSPACE_HOME="$HOME_B"
export PATH="$(dirname "$BIN"):\$PATH"
EOF
)
