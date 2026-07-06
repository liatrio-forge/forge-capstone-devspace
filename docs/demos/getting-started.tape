Output getting-started.gif
Set Width 1000
Set Height 700
Set FontSize 16
Set Shell bash
Set TypingSpeed 40ms

# Setup (not part of the visible story): isolated DEVSPACE_HOME, a fresh
# workspace root, and two placeholder project dirs each with their own git
# repo so `devspace scan` has something to discover.
Hide
Type "export DEVSPACE_HOME=$(mktemp -d)"
Enter
Type "export CLICOLOR_FORCE=1"
Enter
Type "export DEMO_WS=$(mktemp -d)/code"
Enter
Type "mkdir -p $DEMO_WS/api-service $DEMO_WS/web-app"
Enter
Type "git -C $DEMO_WS/api-service init -q"
Enter
Type "git -C $DEMO_WS/web-app init -q"
Enter
Type "export REPO_ROOT=$(git rev-parse --show-toplevel)"
Enter
Type 'export PATH="$REPO_ROOT/bin:$PATH"'
Enter
Sleep 500ms
Type "clear"
Enter
Show

Type "devspace init --workspace $DEMO_WS"
Enter
Sleep 2s

Type "devspace scan"
Enter
Sleep 2s

Type "devspace status"
Enter
Sleep 2s

Type "devspace plan"
Enter
Sleep 2s

Type "devspace apply"
Enter
Sleep 2s

Type "devspace status"
Enter
Sleep 2s
