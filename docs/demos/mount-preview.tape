Output mount-preview.gif
Set Width 1000
Set Height 700
Set FontSize 16
Set Shell bash
Set TypingSpeed 40ms

# `devspace mount --preview` projects the manifest-backed lazy-mount view
# without requiring FUSE: tracked projects appear as entries whether or not
# they are hydrated on disk yet.
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
Type "devspace init --workspace $DEMO_WS"
Enter
Sleep 1s
Type "devspace scan"
Enter
Sleep 1s
Type "clear"
Enter
Show

Type "devspace mount /tmp/devspace-mnt --preview"
Enter
Sleep 3s

Type "devspace mount /tmp/devspace-mnt --preview --json"
Enter
Sleep 3s
