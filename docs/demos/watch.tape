Output watch.gif
Set Width 1000
Set Height 700
Set FontSize 16
Set Shell bash
Set TypingSpeed 40ms

# `devspace watch` keeps metadata fresh from filesystem events. The demo
# shows a one-shot refresh (--once), then live mode: a helper drops a new
# project into the workspace a few seconds in (spawn-project-later.sh) and
# watch picks it up without being asked.
Hide
Type "export DEVSPACE_HOME=$(mktemp -d)"
Enter
Type "export CLICOLOR_FORCE=1"
Enter
Type "export DEMO_WS=$(mktemp -d)/code"
Enter
Type "mkdir -p $DEMO_WS/api-service"
Enter
Type "git -C $DEMO_WS/api-service init -q"
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

Type "devspace watch --once"
Enter
Sleep 2.5s

Hide
Type './spawn-project-later.sh "$DEMO_WS/web-app" 4'
Enter
Sleep 200ms
Show

Type "devspace watch --debounce 1s"
Enter
Sleep 9s

Ctrl+C
Sleep 1s

Type "devspace status"
Enter
Sleep 2.5s
