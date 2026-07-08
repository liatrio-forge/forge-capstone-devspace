Output docs/specs/11-spec-tui-project-remove/11-tui-project-remove.gif
Set Width 1000
Set Height 700
Set FontSize 16
Set Shell bash
Set TypingSpeed 40ms

Hide
Type "export DEVSPACE_HOME=$(mktemp -d)"
Enter
Type "export DEMO_WS=$(mktemp -d)/code"
Enter
Type `mkdir -p "$DEMO_WS/apps/api"`
Enter
Type "export REPO_ROOT=$(git rev-parse --show-toplevel)"
Enter
Type `export PATH="$REPO_ROOT/bin:$PATH"`
Enter
Type `export DEVSPACE_BIN="$REPO_ROOT/bin/devspace"`
Enter
Type `devspace init --workspace "$DEMO_WS" >/dev/null`
Enter
Type "devspace project add apps/api >/dev/null"
Enter
Type "clear"
Enter
Sleep 500ms
Show

Type "$REPO_ROOT/tui/dist/devspace-tui --no-watch"
Enter
Sleep 3s

Type "x"
Sleep 2s

Type "y"
Sleep 3s

Type "q"
Sleep 1s

Type "devspace project"
Enter
Sleep 2s

Type `test -d "$DEMO_WS/apps/api" && echo "project files still on disk"`
Enter
Sleep 2s
