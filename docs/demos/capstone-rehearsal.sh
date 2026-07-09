#!/usr/bin/env bash
# DevSpace capstone demo rehearsal — two-machine "Dropbox for dev workspaces" arc.
#
# Everything runs in a throwaway sandbox (default /tmp/devspace-capstone-demo).
# Your real ~/.devspace is never touched (we set DEVSPACE_HOME per "machine").
#
# Usage:
#   ./capstone-rehearsal.sh          full run: seed Machine A, sync, then Machine B end-to-end
#   ./capstone-rehearsal.sh seed     seed only, leave Machine B ready at "pull" (used by the .tape)
#   ./capstone-rehearsal.sh clean    remove the sandbox
#
# Overrides: SANDBOX=<dir>  DEVSPACE_REPO=<repo>  DEVSPACE_BIN=<prebuilt binary>
set -euo pipefail

# ---- config ---------------------------------------------------------------
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# Repo root = two levels up from docs/demos/ (portable; no hardcoded paths).
DEVSPACE_REPO="${DEVSPACE_REPO:-$(cd "$SCRIPT_DIR/../.." && pwd)}"
SANDBOX="${SANDBOX:-/tmp/devspace-capstone-demo}"
DS="${DEVSPACE_BIN:-$SANDBOX/bin/devspace}"

case "$SANDBOX" in
  ""|"/"|"$HOME"|"$HOME"/*)
    echo "SANDBOX='$SANDBOX' is unsafe; refusing to proceed" >&2
    exit 1
    ;;
esac

# Deterministic git identity for seed repos + manifest commits.
export GIT_AUTHOR_NAME="Demo Dev" GIT_AUTHOR_EMAIL="demo@example.invalid"
export GIT_COMMITTER_NAME="Demo Dev" GIT_COMMITTER_EMAIL="demo@example.invalid"

# "Machines" = separate DevSpace homes over separate workspaces.
HOME_A="$SANDBOX/home-a"      # laptop
HOME_B="$SANDBOX/home-b"      # new machine
WS_A="$SANDBOX/workspace-a"
WS_B="$SANDBOX/workspace-b"
ORIGINS="$SANDBOX/origins"               # upstream repos the projects clone from
MANIFEST_REMOTE="$SANDBOX/manifest.git"  # bare repo the manifest syncs through

banner() { printf '\n\033[1;36m━━ %s\033[0m\n' "$*"; }
say()    { printf '\033[2m$ %s\033[0m\n' "$*"; }

build_binary() {
  if [[ -n "${DEVSPACE_BIN:-}" && -x "$DEVSPACE_BIN" ]]; then DS="$DEVSPACE_BIN"; return; fi
  banner "Building devspace binary"
  mkdir -p "$(dirname "$DS")"
  ( cd "$DEVSPACE_REPO" && go build -trimpath -o "$DS" ./cmd/devspace )
  echo "built: $DS"
}

make_upstream() { # $1 = name
  local name="$1" src="$SANDBOX/src/$1"
  mkdir -p "$src"
  ( cd "$src"
    git init -q
    printf '# %s\n\nDemo project.\n' "$name" > README.md
    printf 'node_modules\n' > .gitignore
    [[ "$name" == web-* ]] && printf '{ "name": "%s", "scripts": { "dev": "vite" } }\n' "$name" > package.json
    git add -A && git commit -qm "init $name"
  )
  git clone -q --bare "$src" "$ORIGINS/$name.git"
}

seed_machine_a() {
  banner "Seed upstream repos (stand-ins for GitHub remotes)"
  mkdir -p "$ORIGINS" "$SANDBOX/src"
  make_upstream web-store
  make_upstream web-admin
  make_upstream api-gateway

  banner "Machine A (laptop): populate the workspace by cloning those repos"
  mkdir -p "$WS_A"
  for r in web-store web-admin api-gateway; do
    git clone -q "$ORIGINS/$r.git" "$WS_A/$r"
  done
  ls -1 "$WS_A"

  export DEVSPACE_HOME="$HOME_A"
  banner "devspace init + scan  (idempotent: safe to re-run, never rotates keys)"
  say "devspace init --workspace $WS_A"; "$DS" init --workspace "$WS_A"
  say "devspace scan";                   "$DS" scan
  say "devspace project";                "$DS" project

  banner "Create a manifest remote and push  (your own Git repo = the sync channel)"
  say "devspace workspace remote create local $MANIFEST_REMOTE"
  "$DS" workspace remote create local "$MANIFEST_REMOTE"
  say "devspace workspace push"; "$DS" workspace push

  banner "Encrypt a per-project secret (age)  — never leaves as plaintext"
  say "echo 'postgres://demo:secret@localhost/store' | devspace env set web-store DATABASE_URL"
  echo 'postgres://demo:secret@localhost/store' | "$DS" env set web-store DATABASE_URL
  say "devspace env list web-store"; "$DS" env list web-store
}

prepare_machine_b() {
  banner "Machine B (new laptop): empty workspace, fresh DevSpace home"
  export DEVSPACE_HOME="$HOME_B"
  mkdir -p "$WS_B"
  say "devspace init --workspace $WS_B"; "$DS" init --workspace "$WS_B"
  say "devspace workspace remote set $MANIFEST_REMOTE"
  "$DS" workspace remote set "$MANIFEST_REMOTE"
  echo "workspace-b contents (empty):"; ls -A "$WS_B" || true
}

run_machine_b() {
  export DEVSPACE_HOME="$HOME_B"
  banner "Pull the manifest — the shape of the workspace arrives"
  say "devspace workspace pull"; "$DS" workspace pull

  banner "PLAN — diff desired vs reality. Every action is Safety-tagged."
  say "devspace plan"; "$DS" plan

  banner "APPLY — creates EMPTY placeholder folders only. Never clones, never deletes."
  say "devspace apply"; "$DS" apply
  echo "workspace-b now has placeholders:"; ls -1 "$WS_B"

  banner "HYDRATE — turn one placeholder into a real checkout (git clone, atomic)"
  say "devspace project hydrate web-store"; "$DS" project hydrate web-store
  echo "web-store is now a real repo:"; ls -A "$WS_B/web-store"

  banner "Doctor + status — readiness at a glance"
  say "devspace doctor"; "$DS" doctor || true
  say "devspace status"; "$DS" status
}

case "${1:-full}" in
  clean) rm -rf "$SANDBOX"; echo "removed $SANDBOX"; exit 0 ;;
  seed)
    rm -rf "$SANDBOX"; build_binary; seed_machine_a; prepare_machine_b
    banner "Seeded. Machine B is ready at the PULL step."
    echo "Next (for live/VHS):  DEVSPACE_HOME=$HOME_B $DS workspace pull"
    ;;
  full)
    rm -rf "$SANDBOX"; build_binary; seed_machine_a; prepare_machine_b; run_machine_b
    banner "Demo complete."
    ;;
  *) echo "usage: $0 [full|seed|clean]"; exit 2 ;;
esac
