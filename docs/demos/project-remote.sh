#!/usr/bin/env bash
# Helper for project-lifecycle.tape: seed a project cloned from a local bare
# origin, then advance that origin so `devspace project update` has a real
# fast-forward to perform. Kept as a script because the git plumbing needs
# quoting that's awkward to Type one line at a time in VHS.
#
#   ./project-remote.sh seed <workspace-dir> <scratch-dir>
#   ./project-remote.sh advance <scratch-dir>
set -euo pipefail
verb="$1"
case "$verb" in
  seed)
    ws="$2" work="$3"
    src="$work/src" origin="$work/origin.git"
    mkdir -p "$src"
    git -C "$src" init -q -b main
    git -C "$src" config user.email demo@example.invalid
    git -C "$src" config user.name "DevSpace Demo"
    echo "# api-service" >"$src/README.md"
    git -C "$src" add -A
    git -C "$src" commit -q -m "init"
    git clone -q --bare "$src" "$origin"
    git clone -q "$origin" "$ws/api-service"
    ;;
  advance)
    work="$2"
    src="$work/src" origin="$work/origin.git"
    echo "upstream change" >>"$src/README.md"
    git -C "$src" commit -q -am "feat: upstream change"
    git -C "$src" push -q "$origin" main
    ;;
  *)
    echo "usage: $0 seed <workspace-dir> <scratch-dir> | advance <scratch-dir>" >&2
    exit 2
    ;;
esac
