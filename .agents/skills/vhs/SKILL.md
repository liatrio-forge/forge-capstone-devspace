---
name: vhs
description: >-
  Use when you need to record or animate a terminal session — create a demo
  GIF, screencast a CLI, produce a README demo, showcase command output, or
  document a tool. Covers VHS (Charm.sh) tape-file syntax, settings, recording
  patterns, the Windows/WSL shell gotcha, and recording TUI apps. Keywords:
  terminal recording, tape file, demo GIF/MP4/WebM, animate CLI, screencast.
---

# VHS — Terminal Recorder

[VHS](https://github.com/charmbracelet/vhs) records terminal sessions from declarative `.tape` files.
Produces GIF, MP4, or WebM. No screen recording needed — deterministic and reproducible, so recordings can live in git and regenerate on demand.

## Install

```bash
brew install vhs     # macOS / Linux
scoop install vhs    # Windows
```

Requires `ffmpeg` for video encoding (`brew install ffmpeg` / `scoop install ffmpeg` if missing).

## Quick Start

```bash
vhs new demo.tape        # scaffold an example tape
vhs demo.tape            # render it (produces the Output files)
vhs validate demo.tape   # parse-check without rendering
vhs record > demo.tape   # capture live keystrokes into a tape, then edit
vhs publish demo.gif     # host on vhs.charm.sh, returns a shareable URL
```

## Tape File Syntax

A tape is read top-to-bottom. **`Output` and all `Set` lines must come before any `Type`/`Enter`/`Sleep` command** — settings apply to the whole recording, not retroactively.

### Output

```tape
Output demo.gif                    # GIF — best for READMEs (autoplays inline)
Output demo.mp4                    # MP4 video
Output demo.webm                   # WebM video
```

Multiple `Output` lines = multiple formats from one tape.

### Settings

```tape
Set Shell "bash"                   # shell to run (see Windows gotcha below)
Set FontSize 14                    # large by default; 12–16 reads well in READMEs
Set FontFamily "JetBrains Mono"
Set Width 1200                     # terminal width in pixels
Set Height 600                     # terminal height in pixels
Set Padding 15                     # padding inside the terminal
Set Margin 20                      # margin outside the terminal (needs MarginFill)
Set MarginFill "#1a1a2e"           # color of that margin
Set TypingSpeed 50ms               # delay between keystrokes
Set Theme "Dracula"                # color theme (`vhs themes` lists them all)
Set Framerate 30                   # frames per second
Set PlaybackSpeed 1.0              # speed multiplier
Set LoopOffset 80%                 # frame the GIF loop restarts on (% or frame #)
Set CursorBlink false              # steady cursor — fewer frames, cleaner GIF
Set WindowBar "Colorful"           # Colorful, ColorfulRight, Rings, RingsRight
Set WindowBarSize 40
Set BorderRadius 8
```

### Commands

```tape
Type "echo hello"                  # type characters
Type@100ms "slow typing"           # type at a custom per-key speed
Enter                              # press Enter
Enter 3                            # press Enter 3 times
Sleep 2s                           # wait (s or ms) — REQUIRED after commands, see below
Sleep 500ms

# Special keys (all accept an optional repeat count, e.g. `Backspace 5`)
Backspace 5
Tab
Ctrl+C
Ctrl+L
Up / Down / Left / Right
Escape
Space
PageUp / PageDown

# Visibility — run commands without showing them
Hide
Show
```

### Require

```tape
Require git                        # abort the recording if the tool isn't in PATH
Require node
```

## Windows: use `Set Shell "cmd"`

VHS's default `bash` resolves to **WSL bash** on Windows — a separate environment with no access to Windows-installed tools (node, scoop packages, etc.). On Windows, always:

```tape
Set Shell "cmd"
```

`cmd` gives VHS the full Windows PATH so installed CLIs resolve. On macOS/Linux, leave `Shell` unset (or `bash`/`zsh`) — this section does not apply.

## Recommended Defaults

Solid starting point for a README demo (macOS/Linux; add `Set Shell "cmd"` on Windows):

```tape
Output demo.gif
Set FontSize 14
Set Width 1100
Set Height 600
Set Theme "Dracula"
Set TypingSpeed 30ms
Set Padding 15
Set CursorBlink false
Set WindowBar "Colorful"
Set BorderRadius 8
```

## Patterns

### Simple command showcase

```tape
Output demo.gif
Set FontSize 14
Set Width 1100
Set Height 600
Set Theme "Dracula"
Set TypingSpeed 30ms

Type "my-tool --help"
Enter
Sleep 3s

Type "my-tool run --input data.json"
Enter
Sleep 5s
```

### Complex commands → use a wrapper script

Tape files handle shell quoting poorly. For commands with quotes, pipes, or
multi-line args, put them in a script and call the script:

```bash
# demo-run.sh
#!/bin/bash
echo "Running analysis..."
my-tool analyze --format json | jq '.results[] | .name'
```

```tape
Type "bash demo-run.sh"
Enter
Sleep 10s
```

### Hide setup, show the interesting part

```tape
Hide
Type "cd /tmp/demo-project"
Enter
Type "export DEMO_MODE=1"
Enter
Sleep 1s
Show

# visible demo starts here
Type "my-tool init"
Enter
Sleep 3s
```

### Before/after comparison

```tape
Type "cat config.yaml"
Enter
Sleep 3s

Type "my-tool fix config.yaml"
Enter
Sleep 3s

Type "cat config.yaml"
Enter
Sleep 3s
```

### Record live, then clean up

```bash
vhs record > my-session.tape   # captures your keystrokes as you work
vhs my-session.tape            # then edit timing/sleeps and re-render
```

### Recording TUI apps (holdpty)

Plain VHS can't drive apps that need a real PTY (colors/layout break) or that
take time to start. [holdpty](https://github.com/marcfargas/holdpty) holds the
PTY; VHS attaches to it:

```bash
holdpty launch --bg --name demo -- my-tui-app   # or attach to an already-running one
sleep 5                                          # let it start
vhs record-demo.tape                             # record
holdpty stop demo                                # or leave it running
```

```tape
Output demo.gif
Set FontSize 14
Set Width 1200
Set Height 700
Set Theme "Dracula"
Set Padding 15
Set WindowBar "Colorful"

Type "holdpty attach demo"                       # `holdpty view demo` for read-only
Enter
Sleep 2s
Type "your input here"
Enter
Sleep 15s
```

Good for TUIs (htop, k9s, lazygit), slow-starting servers/agents, live bug
repros, and capturing an AI agent working with its full colored TUI.

> Windows: launch node CLIs as `node.exe <path/to/cli.js>` — see the holdpty repo for the `.cmd` wrapper gotcha.

## Common Mistakes

| Mistake | Fix |
| ------- | --- |
| Output cut off / command "did nothing" | Add `Sleep` after every `Enter` — VHS types instantly and stops recording before slow output renders. |
| `Set`/`Output` placed after `Type` | Move all settings to the top; they don't apply retroactively. |
| Quotes/pipes in `Type` misbehave | Put the command in a wrapper script and `Type "bash script.sh"`. |
| Windows CLIs "not found" | `Set Shell "cmd"` — default `bash` is WSL with a different PATH. |
| Huge GIF file | Lower `Width`/`Height`/`Framerate`, raise `PlaybackSpeed`, trim `Sleep`s, or output MP4/WebM. |
| Blinking cursor adds noise/frames | `Set CursorBlink false`. |
