#!/usr/bin/env bash
# Helper for watch.tape: after a short delay, drop a new git-init'd project
# into the workspace so `devspace watch` has a live filesystem event to react
# to. Self-backgrounds because VHS Type lines handle `&` poorly.
set -euo pipefail
dir="$1" delay="${2:-4}"
(
  sleep "$delay"
  mkdir -p "$dir"
  git -C "$dir" init -q
) >/dev/null 2>&1 &
