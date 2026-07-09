Output reconcile.gif
Set Width 1000
Set Height 700
Set FontSize 16
Set Shell bash
Set TypingSpeed 40ms

# Two machines diverge, then `workspace diff` previews and
# `workspace reconcile` three-way merges the manifests. Setup seeds both
# machines already sharing a manifest remote (same shape as
# workspace-sync.tape); the visible story starts at the divergence.
Hide
Type "export CLICOLOR_FORCE=1"
Enter
Type "WORK=$(mktemp -d)"
Enter
Type `export HOME_A="$WORK/home-a" HOME_B="$WORK/home-b" CODE_A="$WORK/machine-a/code" CODE_B="$WORK/machine-b/code" REMOTE="$WORK/manifest-remote.git"`
Enter
Type `mkdir -p "$HOME_A" "$HOME_B" "$CODE_A" "$CODE_B"`
Enter
Type `export PATH="$(pwd)/../../bin:$PATH"`
Enter
Type `export DEVSPACE_HOME="$HOME_A"`
Enter
Type `devspace init --workspace "$CODE_A" >/dev/null`
Enter
Type `devspace workspace remote create local "$REMOTE" >/dev/null`
Enter
Type `devspace workspace push >/dev/null`
Enter
Type `export DEVSPACE_HOME="$HOME_B"`
Enter
Type `devspace init --workspace "$CODE_B" >/dev/null`
Enter
Type `devspace workspace remote set "$REMOTE" >/dev/null`
Enter
Type `devspace workspace pull >/dev/null`
Enter
Sleep 500ms
Type "clear"
Enter
Show

# --- Machine A tracks a new project and pushes ---

Type `export DEVSPACE_HOME="$HOME_A"`
Enter
Sleep 1s

Hide
Type `./mkgitproject.sh "$CODE_A/payments-api"`
Enter
Sleep 500ms
Show

Type `devspace scan`
Enter
Sleep 2s

Type `devspace workspace push`
Enter
Sleep 2s

# --- Machine B diverged in the meantime: it tracked its own project ---

Type `export DEVSPACE_HOME="$HOME_B"`
Enter
Sleep 1s

Hide
Type `./mkgitproject.sh "$CODE_B/data-pipeline"`
Enter
Sleep 500ms
Show

Type `devspace scan`
Enter
Sleep 2s

Type `devspace workspace diff`
Enter
Sleep 3s

Type `devspace workspace reconcile`
Enter
Sleep 3s

Type `devspace workspace reconcile --apply`
Enter
Sleep 3s

Type `devspace workspace push`
Enter
Sleep 2s

Type `devspace status`
Enter
Sleep 2.5s
