Output setup-commands.gif
Set Width 1000
Set Height 700
Set FontSize 16
Set Shell bash
Set TypingSpeed 40ms

# `devspace setup` detects per-project install/dev commands and only ever
# runs them after review. The demo stays on plan/--dry-run so nothing is
# actually installed — that's the point of the command.
Hide
Type "export DEVSPACE_HOME=$(mktemp -d)"
Enter
Type "export CLICOLOR_FORCE=1"
Enter
Type "export DEMO_WS=$(mktemp -d)/code"
Enter
Type "mkdir -p $DEMO_WS/web-app $DEMO_WS/api-service"
Enter
Type "git -C $DEMO_WS/web-app init -q"
Enter
Type "git -C $DEMO_WS/api-service init -q"
Enter
Type `printf '{"name":"web-app","scripts":{"dev":"vite"}}' > $DEMO_WS/web-app/package.json`
Enter
Type "printf 'module demo\n\ngo 1.22\n' > $DEMO_WS/api-service/go.mod"
Enter
Type "export REPO_ROOT=$(git rev-parse --show-toplevel)"
Enter
Type 'export PATH="$REPO_ROOT/bin:$PATH"'
Enter
Type "devspace init --workspace $DEMO_WS"
Enter
Sleep 1s
Type "devspace scan"
Enter
Sleep 1s
Type "clear"
Enter
Show

Type "devspace setup plan"
Enter
Sleep 3s

Type "devspace setup run web-app --dry-run"
Enter
Sleep 2.5s

Type "devspace setup run web-app --command dev --dry-run"
Enter
Sleep 2.5s

Type "devspace setup apply --dry-run"
Enter
Sleep 3s
