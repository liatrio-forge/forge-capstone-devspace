#!/usr/bin/env bash
# Hidden setup (part 2) for the spec 06 reconcile demo tape.
# Creates a genuine conflict: A sets apps/shared defaultBranch=develop and
# pushes; B sets defaultBranch=trunk locally without pushing. Mechanics
# lifted verbatim from the spec 06 task-04 demo script
# (.claude/jobs/ecc582af/tmp/demo-reconcile.sh), conflict-setup portion only.
# Requires part 1 to have run first (sources its env file). Never touches
# the real ~/.devspace.
set -euo pipefail
source /tmp/06-reconcile-demo-env.sh

DEVSPACE_HOME="$HOME_A" "$BIN" workspace pull >/dev/null

python3 - "$WS_A/.devspace/manifest.json" develop <<'PY'
import json,sys
p,branch=sys.argv[1],sys.argv[2]
m=json.load(open(p))
for pr in m["projects"]:
    if pr["path"]=="apps/shared": pr["defaultBranch"]=branch
json.dump(m,open(p,"w"),indent=2)
PY
DEVSPACE_HOME="$HOME_A" "$BIN" workspace push >/dev/null

python3 - "$WS_B/.devspace/manifest.json" trunk <<'PY'
import json,sys
p,branch=sys.argv[1],sys.argv[2]
m=json.load(open(p))
for pr in m["projects"]:
    if pr["path"]=="apps/shared": pr["defaultBranch"]=branch
json.dump(m,open(p,"w"),indent=2)
PY
