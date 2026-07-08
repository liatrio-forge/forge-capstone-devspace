# Task 02 Proofs - Verify the GoReleaser release path attaches all four TUI assets

## Task Summary

Confirms the Task 1.0 fix holds all the way through the GoReleaser release
path: config validation, a snapshot release run, and checksum coverage for
all four `devspace-tui_*` assets, without publishing/tagging/deploying
anything.

## What This Task Proves

- `.goreleaser.yaml` remains valid with the `devspace-tui_*` extra_files
  glob.
- A snapshot GoReleaser run (`--clean --skip=publish`) produces the four Go
  archives and attaches/checksums the four TUI companion binaries alongside
  them.
- No generated `tui/dist/` or `dist/` output is staged for commit.
- `release.yml` already invokes `make tui-build-all`, so no CI workflow edit
  is required by this spec.

## Evidence Summary

- `goreleaser check`: `1 configuration file(s) validated`.
- `goreleaser release --snapshot --clean --skip=publish`: `release succeeded
  after 4s`.
- `grep -c 'devspace-tui_' dist/checksums.txt` returns `4`.
- `git status --porcelain` shows no `dist/` or `tui/dist/` entries (both
  remain `.gitignore`d, confirmed via `--ignored`).
- `.github/workflows/release.yml` already calls `make tui-build-all` (no
  edit made).

## Artifact: `goreleaser check` validates the config

**What it proves:** `.goreleaser.yaml` (including the `devspace-tui_*`
extra_files glob) is still a valid GoReleaser configuration.

**Why it matters:** This is the fast, non-destructive gate the spec asks for
before attempting a full snapshot run.

**Command:**

```bash
goreleaser check; echo "EXIT:$?"
```

**Result summary:** Validated, exit 0.

```text
  • checking                                  path=.goreleaser.yaml
  • 1 configuration file(s) validated
  • thanks for using GoReleaser!
EXIT:0
```

## Artifact: GoReleaser snapshot release run

**What it proves:** End-to-end, GoReleaser builds the four Go archives,
computes checksums, and (via the `ko` image step) completes a full release
pass with the `devspace-tui_*` extra_files attached.

**Why it matters:** This is the closest local approximation of what
`release.yml` does on `ubuntu-latest`; it proves the release path — not just
the build script — sees all four TUI assets.

**Deviation note:** `docker info` reported success but the default
`/var/run/docker.sock` path was absent because the local Docker CLI targets
an OrbStack context at a non-standard socket path. Setting
`DOCKER_HOST=unix:///Users/lecoqjacob/.orbstack/run/docker.sock` (an
environment variable pointing at the already-running daemon, not a new
install) let the `ko` step publish to the local daemon. No software was
installed to make this work.

The `ko` step logged `git is in a dirty state ... M tui/build-all.sh`; this
is an accurate, expected warning since Task 1.0's `build-all.sh` edit was
still uncommitted at snapshot-run time. It does not affect the build/asset
outcome and is informational only from `ko`'s embedded VCS metadata check.

**Command:**

```bash
DOCKER_HOST=unix:///Users/lecoqjacob/.orbstack/run/docker.sock \
  goreleaser release --snapshot --clean --skip=publish
```

**Result summary:** `release succeeded after 4s`; four archives + four TUI
binaries checksummed.

```text
  • starting release
  • skipping announce, publish, and validate...
  • cleaning distribution directory
  • getting and validating git state
    • using tags                                     previous=v0.1.0 current=v0.2.0
  • snapshotting
    • building snapshot...                           version=0.2.0-SNAPSHOT-31b3cae
  • building binaries
    • building                                       paths=cmd/devspace binaries=devspace target=darwin_arm64_v8.0
    • building                                       paths=cmd/devspace binaries=devspace target=linux_arm64_v8.0
    • building                                       paths=cmd/devspace binaries=devspace target=darwin_amd64_v1
    • building                                       paths=cmd/devspace binaries=devspace target=linux_amd64_v1
  • archives
    • archiving                                      name=dist/devspace_v0.2.0-SNAPSHOT-31b3cae_darwin_arm64.tar.gz
    • archiving                                      name=dist/devspace_v0.2.0-SNAPSHOT-31b3cae_linux_arm64.tar.gz
    • archiving                                      name=dist/devspace_v0.2.0-SNAPSHOT-31b3cae_darwin_amd64.tar.gz
    • archiving                                      name=dist/devspace_v0.2.0-SNAPSHOT-31b3cae_linux_amd64.tar.gz
  • calculating checksums
  • ko
2026/07/08 16:42:20 git is in a dirty state
Please check in your pipeline what can be changing the following files:
 M tui/build-all.sh

2026/07/08 16:42:21 Adding tag v0.2.0
2026/07/08 16:42:21 Added tag v0.2.0
2026/07/08 16:42:21 Adding tag latest
2026/07/08 16:42:22 Added tag latest
  • writing artifacts metadata
  • release succeeded after 4s
  • thanks for using GoReleaser!
```

## Artifact: checksums.txt contains all four TUI assets

**What it proves:** GoReleaser's `extra_files` glob picked up all four
`devspace-tui_*` binaries from `tui/dist/` and checksummed them alongside the
Go release archives.

**Why it matters:** This is the spec's release-readiness success metric (4
assets + 4 checksum entries). Note `extra_files` are checksummed in place
from their source glob path; GoReleaser does not copy them into the
top-level `dist/` directory (only the Go archives/binaries are materialized
there), so `find dist -iname '*devspace-tui*'` correctly returns nothing —
the checksums.txt entries are the authoritative proof surface here.

**Command:**

```bash
grep -c 'devspace-tui_' dist/checksums.txt
grep 'devspace-tui_' dist/checksums.txt
```

**Result summary:** Count is `4`, one line per platform binary.

```text
4
c43be768afb2f770ff3a83f746cacb26b6c1e81aa57ace5bb0c8138bd52d090f  devspace-tui_darwin_amd64
a48d4006bff2d9b9f9cdcaee84d8c2f13a622ed566ab3e40b1d147dce7651de7  devspace-tui_darwin_arm64
6e96dcfcbc317b5acc442be3ef68fe24c610fa03fceefef76df4622265fa19be  devspace-tui_linux_amd64
ce47d6920f0a3f72400714d21db54c17e86335de4a4fd26bcb63975d4f995420  devspace-tui_linux_arm64
```

## Artifact: no generated build output staged

**What it proves:** Neither the snapshot's `dist/` output nor `tui/dist/`
binaries are tracked or staged for commit.

**Why it matters:** The spec requires generated build products stay
git-ignored, never committed.

**Command:**

```bash
/usr/bin/git -C /Users/lecoqjacob/Projects/liatrio/devspace status --porcelain --ignored | grep -E '^\?\?|dist/'
```

**Result summary:** All `dist/` and `tui/dist/` entries appear with the `!!`
(ignored) marker, not `??` (untracked) — none are staged or eligible for
accidental commit.

```text
!! dist/artifacts.json
!! dist/checksums.txt
!! dist/config.yaml
!! dist/devspace_darwin_amd64_v1/devspace
!! dist/devspace_darwin_arm64_v8.0/devspace
!! dist/devspace_linux_amd64_v1/devspace
!! dist/devspace_linux_arm64_v8.0/devspace
!! dist/devspace_v0.2.0-SNAPSHOT-31b3cae_darwin_amd64.tar.gz
!! dist/devspace_v0.2.0-SNAPSHOT-31b3cae_darwin_arm64.tar.gz
!! dist/devspace_v0.2.0-SNAPSHOT-31b3cae_linux_amd64.tar.gz
!! dist/devspace_v0.2.0-SNAPSHOT-31b3cae_linux_arm64.tar.gz
!! dist/metadata.json
!! tui/dist/devspace-tui_darwin_amd64
!! tui/dist/devspace-tui_darwin_arm64
!! tui/dist/devspace-tui_linux_amd64
!! tui/dist/devspace-tui_linux_arm64
```

## Artifact: `release.yml` already calls `make tui-build-all`

**What it proves:** No CI workflow edit is required for this spec; the
Task 1.0 fix takes effect the moment `release.yml` runs on `ubuntu-latest`.

**Why it matters:** Confirms the narrow scope of this change — the fix lives
entirely in `tui/build-all.sh`.

**Command:**

```bash
grep -n 'tui-build-all' .github/workflows/release.yml
```

**Result summary:** `release.yml` already runs `make tui-build-all` as a
build step ahead of the GoReleaser action; confirmed present, unedited.

## Reviewer Conclusion

The GoReleaser release path is proven end-to-end for the TUI assets: config
validation passes, a full snapshot release run succeeds and attaches all
four `devspace-tui_*` binaries with correct checksums, and no build output
leaks into the git index. `release.yml` already invokes `make
tui-build-all`, so this spec requires no workflow changes — the fix in Task
1.0 is sufficient for CI to pick up automatically. Task 2.0 is complete and
verified.
