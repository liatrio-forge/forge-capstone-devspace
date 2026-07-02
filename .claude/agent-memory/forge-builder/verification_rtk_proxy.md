---
name: verification-rtk-proxy
description: Use `rtk proxy go test ...` to get raw/verbose go test output in this repo, since the user's global rtk hook intercepts and condenses `go test` results.
metadata:
  type: feedback
---

In this repo (and likely others under this user's global Claude Code config), a shell hook driven by `rtk` (Rust Token Killer, see the user's global CLAUDE.md/RTK.md) transparently rewrites commands like `go test ./...` and condenses the output to a one-line summary (e.g. `Go test: 76 passed in 2 packages`), even when `-v` is passed.

**Why:** This is intentional token-saving behavior the user has configured globally, not a bug. But it hides per-test names, `--- PASS`/`--- FAIL` lines, and race-detector detail needed to verify new tests actually ran and passed individually.

**How to apply:** When verifying specific new tests (checking exact test names ran/passed, counting subtests, or reading race detector output), bypass the condensed hook with `rtk proxy <command>`, e.g. `rtk proxy go test ./... -run 'TestFoo' -v` or `rtk proxy go test ./... -race`. Use plain `go build ./...`, `go vet ./...`, `gofmt -l <dir>` as normal (not intercepted/condensed the same way). Use the condensed `go test ./...` summary for a quick pass/fail check, and `rtk proxy` when you need the detail.
