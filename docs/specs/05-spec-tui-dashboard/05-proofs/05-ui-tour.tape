# vhs tape for the devspace ui dashboard tour (spec 05, task 4.1).
# Render from the repo root:  vhs docs/specs/05-spec-tui-dashboard/05-proofs/05-ui-tour.tape
# Uses a throwaway DEVSPACE_HOME + temp workspace — no real remotes/tokens/secrets.

Output docs/specs/05-spec-tui-dashboard/05-proofs/05-ui-tour.gif

Set Shell "bash"
Set FontSize 16
Set Width 1200
Set Height 720
Set Padding 12

# --- setup (hidden): sandboxed home + seeded workspace ---
Hide
Type "export DEVSPACE_HOME=$(mktemp -d) NO_COLOR=" Enter
Type "WS=$(mktemp -d)" Enter
Type "bin/devspace init --workspace $WS >/dev/null" Enter
Sleep 400ms
Type "mkdir -p $WS/apps/api && git -C $WS/apps/api init -q && git -C $WS/apps/api commit -q --allow-empty -m init" Enter
Type "printf 'TOKEN=x\n' > $WS/apps/api/.env && printf 'dirty\n' > $WS/apps/api/work.txt" Enter
Type "mkdir -p $WS/services/worker && printf '{}\n' > $WS/services/worker/package.json" Enter
Type "bin/devspace scan >/dev/null" Enter
Sleep 400ms
Type "clear" Enter
Show

# --- the tour ---
Type "bin/devspace ui" Enter
Sleep 2s
Type "j" Sleep 700ms
Type "j" Sleep 700ms
Type "k" Sleep 700ms
Type "r" Sleep 1500ms
Type "s" Sleep 1500ms
Type "q"
Sleep 1s
