#!/usr/bin/env bash
# Helper for workspace-sync.tape: create a throwaway git-init'd project dir
# so `devspace scan` has something real to find. Kept as a script (rather
# than inline in the tape) because the git plumbing needs quoting that's
# awkward to type-and-Enter one line at a time in VHS.
set -euo pipefail
dir="$1"
mkdir -p "$dir"
git -C "$dir" init -q
git -C "$dir" config user.email demo@example.invalid
git -C "$dir" config user.name "DevSpace Demo"
touch "$dir/README.md"
git -C "$dir" add -A
git -C "$dir" commit -q -m "init" >/dev/null
