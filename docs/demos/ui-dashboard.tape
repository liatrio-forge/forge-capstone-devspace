Output ui-dashboard.gif
Set Width 1000
Set Height 700
Set FontSize 16
Set Shell bash
Set TypingSpeed 40ms

# The built-in full-screen dashboard (`devspace ui --legacy`): tracked
# projects, scan counts, and safe actions only. --legacy forces the
# built-in Bubble Tea view even when the devspace-tui companion is
# installed; --no-watch keeps the recording deterministic.
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
Type "devspace init --workspace $DEMO_WS >/dev/null"
Enter
Type "devspace scan >/dev/null"
Enter
Sleep 500ms
Type "clear"
Enter
Show

Type "devspace ui --legacy --no-watch"
Enter
Sleep 4s

Down
Sleep 1.5s
Down
Sleep 1.5s
Up
Sleep 1.5s

Type "q"
Sleep 1.5s
