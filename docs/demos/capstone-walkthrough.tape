# DevSpace capstone walkthrough — two-machine "Dropbox for workspaces" payoff.
# Render from docs/demos/:  vhs capstone-walkthrough.tape
# The Hide block seeds Machine A + a manifest remote (via capstone-rehearsal.sh)
# and leaves Machine B ready at the pull step, then records the hero sequence.

Output capstone-walkthrough.gif
Set Width 1000
Set Height 700
Set FontSize 16
Set Shell bash
Set TypingSpeed 40ms

# --- silent setup: seed everything, put devspace on PATH, target Machine B ---
Hide
Type "export SANDBOX=/tmp/devspace-capstone-demo" Enter
Type "./capstone-rehearsal.sh seed >/tmp/devspace-capstone-seed.log 2>&1" Enter
Wait@2m
Type 'export PATH="$SANDBOX/bin:$PATH"; export DEVSPACE_HOME="$SANDBOX/home-b"; cd "$SANDBOX/workspace-b"' Enter
Type "clear" Enter
Show

# --- Machine B — the payoff ---
Type "# New machine. Empty workspace. Let's pull my workspace shape." Enter
Sleep 1.5s
Type "devspace workspace pull" Enter
Sleep 2.5s

Type "# plan: diff desired vs reality — every action is Safety-tagged" Enter
Sleep 1s
Type "devspace plan" Enter
Sleep 3s

Type "# apply: creates EMPTY placeholders only. Never clones. Never deletes." Enter
Sleep 1s
Type "devspace apply" Enter
Sleep 2s
Type "ls" Enter
Sleep 2s

Type "# hydrate: turn one placeholder into a real checkout (atomic git clone)" Enter
Sleep 1s
Type "devspace project hydrate web-store" Enter
Sleep 3s

Type "# whole-workspace truth, one command" Enter
Sleep 1s
Type "devspace status" Enter
Sleep 3.5s
