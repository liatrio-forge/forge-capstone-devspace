Output project-lifecycle.gif
Set Width 1000
Set Height 700
Set FontSize 16
Set Shell bash
Set TypingSpeed 40ms

# The full `devspace project` lifecycle: add → status → update (fast-forward
# from origin) → remove. Setup seeds a project cloned from a local bare
# origin (project-remote.sh) so `project update` has a real fast-forward.
Hide
Type "export DEVSPACE_HOME=$(mktemp -d)"
Enter
Type "export CLICOLOR_FORCE=1"
Enter
Type "export WORK=$(mktemp -d)"
Enter
Type "export DEMO_WS=$WORK/code"
Enter
Type "mkdir -p $DEMO_WS"
Enter
Type './project-remote.sh seed "$DEMO_WS" "$WORK"'
Enter
Type "export REPO_ROOT=$(git rev-parse --show-toplevel)"
Enter
Type 'export PATH="$REPO_ROOT/bin:$PATH"'
Enter
Type "devspace init --workspace $DEMO_WS"
Enter
Sleep 1s
Type "clear"
Enter
Show

Type "devspace project add api-service"
Enter
Sleep 2s

Type "devspace project status"
Enter
Sleep 2s

# --- upstream moves ahead; update fast-forwards the local checkout ---

Hide
Type './project-remote.sh advance "$WORK"'
Enter
Sleep 500ms
Show

Type "devspace project update api-service"
Enter
Sleep 2s

Type "devspace project status api-service"
Enter
Sleep 2s

Type "devspace project remove api-service"
Enter
Sleep 2s

Type "devspace project status"
Enter
Sleep 2s

Type "ls $DEMO_WS"
Enter
Sleep 2s
