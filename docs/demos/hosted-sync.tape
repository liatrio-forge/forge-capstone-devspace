Output hosted-sync.gif
Set Width 1000
Set Height 700
Set FontSize 16
Set Shell bash
Set TypingSpeed 40ms

# Setup (not part of the visible story): isolated DEVSPACE_HOME, a fresh
# workspace with one git-init'd project, and a background hosted-sync
# server (docs/demos/hosted-sync-serve.sh) on a scratch port. The token is
# an obvious placeholder and is only ever set as an env var, never typed.
Hide
Type "export DEVSPACE_HOME=$(mktemp -d)"
Enter
Type "export CLICOLOR_FORCE=1"
Enter
Type "export DEVSPACE_HOSTED_TOKEN=demo-token"
Enter
Type "export REPO_ROOT=$(git rev-parse --show-toplevel)"
Enter
Type 'export PATH="$REPO_ROOT/bin:$PATH"'
Enter
Type "export DEMO_WS=$(mktemp -d)/code"
Enter
Type "mkdir -p $DEMO_WS/api-service"
Enter
Type "git -C $DEMO_WS/api-service init -q"
Enter
Type "git -C $DEMO_WS/api-service -c user.email=demo@example.com -c user.name=Demo commit --allow-empty -q -m init"
Enter
Type "devspace init --workspace $DEMO_WS"
Enter
Sleep 500ms
Type "devspace project add api-service"
Enter
Sleep 500ms
Type "devspace scan"
Enter
Sleep 500ms
Type 'bash "$REPO_ROOT/docs/demos/hosted-sync-serve.sh"'
Enter
Sleep 2s
Type "clear"
Enter
Show

Type "devspace hosted config set http://127.0.0.1:8791 --workspace demo-team"
Enter
Sleep 2s

Type "devspace hosted push"
Enter
Sleep 2s

Type "devspace hosted config get"
Enter
Sleep 2s

Type "devspace hosted pull"
Enter
Sleep 2s

Type "devspace hosted reconcile"
Enter
Sleep 3s

Hide
Type 'source "$DEMO_WS/hosted-sync-demo.env"'
Enter
Type 'kill "$(cat "$HOSTED_SYNC_DEMO_PID_FILE")" 2>/dev/null'
Enter
Type 'rm -rf "$HOSTED_SYNC_DEMO_STORE_DIR" "$DEMO_WS/hosted-sync-demo.env"'
Enter
Sleep 500ms
Show
