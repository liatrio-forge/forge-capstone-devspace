Output env-secrets.gif
Set Width 1000
Set Height 700
Set FontSize 16
Set Shell bash
Set TypingSpeed 40ms

# Setup (not part of the visible story): isolated DEVSPACE_HOME, a fresh
# workspace with one git-init'd project, initialized + scanned so the
# manifest already tracks "demo-project" before the demo starts.
Hide
Type "export DEVSPACE_HOME=$(mktemp -d)"
Enter
Type "export CLICOLOR_FORCE=1"
Enter
Type "export DEMO_WS=$(mktemp -d)"
Enter
Type "mkdir -p $DEMO_WS/demo-project"
Enter
Type "git -C $DEMO_WS/demo-project init -q"
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
Type "cd $DEMO_WS"
Enter
Sleep 500ms
Type 'cp "$REPO_ROOT/docs/demos/set-api-key.sh" ./set-api-key.sh'
Enter
Type "chmod +x set-api-key.sh"
Enter
Sleep 500ms
Type "clear"
Enter
Show

Type "cat set-api-key.sh"
Enter
Sleep 2s

Type "./set-api-key.sh"
Enter
Sleep 2s

Type "devspace env list demo-project"
Enter
Sleep 2s

Type "devspace env pull demo-project"
Enter
Sleep 2s

Type "cat demo-project/.env"
Enter
Sleep 2s

Type "ls -l demo-project/.env"
Enter
Sleep 2s
