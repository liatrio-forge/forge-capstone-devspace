Output spinner.gif
Set Width 1000
Set Height 300
Set FontSize 16
Set Shell bash
Set TypingSpeed 40ms

# Prerequisite (run before this tape, not captured here): a devspace binary
# on PATH as ./devspace, and DEVSPACE_HOME already initialized with a
# workspace tracking an on-demand Git placeholder project named "lazy"
# pointing at a real (e.g. local bare) remote. This tape only replays the
# demo portion for a portable path; substitute your own initialized
# DEVSPACE_HOME below.
Type "export DEVSPACE_HOME=$(mktemp -d)"
Enter
Type "clear"
Enter
Sleep 300ms
Type "./devspace project hydrate lazy"
Enter
Screenshot s1.png
Sleep 50ms
Screenshot s2.png
Sleep 50ms
Screenshot s3.png
Sleep 50ms
Screenshot s4.png
Sleep 1s
Screenshot s5.png
