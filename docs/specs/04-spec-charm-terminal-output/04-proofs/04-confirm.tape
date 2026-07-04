Output confirm.gif
Set Width 1000
Set Height 400
Set FontSize 16
Set Shell bash
Set TypingSpeed 40ms

# Prerequisite (run before this tape, not captured here): a devspace binary
# on PATH as ./devspace, and DEVSPACE_HOME already initialized with a
# workspace containing an apps/web project with a package.json (npm scripts),
# e.g. via `devspace init --workspace ...` + `devspace scan`. This tape only
# replays the demo portion for a portable path; substitute your own
# initialized DEVSPACE_HOME below.
Type "export DEVSPACE_HOME=$(mktemp -d)"
Enter
Type "clear"
Enter
Sleep 500ms

Type "./devspace setup run web"
Enter
Sleep 800ms
Screenshot confirm-prompt.png
Sleep 500ms
Type "n"
Sleep 1s
Screenshot confirm-rejected.png
Sleep 500ms
