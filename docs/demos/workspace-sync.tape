Output workspace-sync.gif
Set Width 1000
Set Height 700
Set FontSize 16
Set Shell bash
Set TypingSpeed 40ms

# Two machines sharing a workspace manifest through a local bare Git remote.
# Everything below runs against temp DEVSPACE_HOME dirs and a temp workspace
# tree — never the real ~/.devspace. Run `vhs workspace-sync.tape` from this
# directory (docs/demos/).

Hide
Type "export CLICOLOR_FORCE=1"
Enter
Type "WORK=$(mktemp -d)"
Enter
Type `export HOME_A="$WORK/home-a" HOME_B="$WORK/home-b" CODE_A="$WORK/machine-a/code" CODE_B="$WORK/machine-b/code" REMOTE="$WORK/manifest-remote.git"`
Enter
Type `mkdir -p "$HOME_A" "$HOME_B" "$CODE_A" "$CODE_B"`
Enter
Type `export PATH="$(pwd)/../../bin:$PATH"`
Enter
Type "clear"
Enter
Sleep 500ms
Show

Type `export DEVSPACE_HOME="$HOME_A"`
Enter
Sleep 1.5s

Type "echo '--- MACHINE A: MY WORKSPACE ---'"
Enter
Sleep 1.5s

Type `devspace init --workspace "$CODE_A"`
Enter
Sleep 2.5s

Hide
Type `./mkgitproject.sh "$CODE_A/api-service"`
Enter
Sleep 1s
Show

Type `devspace scan`
Enter
Sleep 2.5s

Type `devspace project`
Enter
Sleep 2.5s

Type `devspace workspace remote create local "$REMOTE"`
Enter
Sleep 2.5s

Type `devspace workspace push`
Enter
Sleep 3s

# --- Machine B joins the same workspace manifest ---

Type `export DEVSPACE_HOME="$HOME_B"`
Enter
Sleep 1.5s

Type "echo '--- MACHINE B: A NEW COMPUTER ---'"
Enter
Sleep 1.5s

Type `devspace init --workspace "$CODE_B"`
Enter
Sleep 2.5s

Type `devspace workspace remote set "$REMOTE"`
Enter
Sleep 2.5s

Type `devspace workspace pull`
Enter
Sleep 3s

Type `devspace project`
Enter
Sleep 3s

# --- Back on A: a second project appears and gets pushed ---

Hide
Type `export DEVSPACE_HOME="$HOME_A"`
Enter
Type `./mkgitproject.sh "$CODE_A/worker-service"`
Enter
Sleep 1s
Show

Type "echo '--- MACHINE A: ADD A SECOND PROJECT ---'"
Enter
Sleep 1.5s

Type `devspace scan`
Enter
Sleep 2.5s

Type `devspace project`
Enter
Sleep 2.5s

Type `devspace workspace push`
Enter
Sleep 3s

# --- B catches up: pull the new project, then diff confirms it's clean ---

Type `export DEVSPACE_HOME="$HOME_B"`
Enter
Sleep 1.5s

Type "echo '--- MACHINE B: CATCH UP ---'"
Enter
Sleep 1.5s

Type `devspace workspace pull`
Enter
Sleep 3s

Type `devspace project`
Enter
Sleep 3s

Type `devspace workspace diff`
Enter
Sleep 3s
