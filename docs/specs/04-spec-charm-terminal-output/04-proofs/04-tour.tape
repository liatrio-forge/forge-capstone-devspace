Output tour.gif
Set Width 1000
Set Height 700
Set FontSize 16
Set Shell bash
Set TypingSpeed 40ms

# Prerequisite (run before this tape, not captured here): a devspace binary
# on PATH as ./devspace, and DEVSPACE_HOME already initialized with a
# workspace containing one tracked project, e.g. via
# `devspace init --workspace ...` + `devspace project add ...`. This tape
# only replays the demo portion for a portable path; substitute your own
# initialized DEVSPACE_HOME below.
Type "export DEVSPACE_HOME=$(mktemp -d)"
Enter
Type "export CLICOLOR_FORCE=1"
Enter
Type "clear"
Enter
Sleep 500ms

Type "./devspace status"
Enter
Sleep 2s

Type "./devspace doctor"
Enter
Sleep 2s

Type "./devspace plan"
Enter
Sleep 2s
