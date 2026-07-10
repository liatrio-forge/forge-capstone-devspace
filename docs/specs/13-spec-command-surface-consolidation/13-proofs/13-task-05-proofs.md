# Task 05 Proofs - Release Command Contract

Status: READY FOR REVIEW

## Task Summary

This task proves that maintained documentation, executable demos, local quality
gates, and release archives all use the Spec 13 command contract. The evidence
was produced with isolated temporary application homes and workspaces; no
credentials, private remotes, or user project data were used.

## What This Task Proves

- Maintained documentation and demo sources contain the canonical command paths
  and reject removed paths, including wrapped Markdown examples.
- The two-machine demo restores workspace metadata, creates placeholders,
  explicitly updates a project, and writes a masked env profile with mode
  `0600`.
- The Go, TUI, vulnerability, command-surface, and GoReleaser configuration
  gates pass.
- The complete GoReleaser snapshot, including the ko image step, succeeds when
  pointed at the active Docker context.
- Every Linux/macOS amd64/arm64 archive contains executable, platform-matched
  `devspace` and `devspace-tui` files, and the local archive launches the
  adjacent companion.

## Evidence Summary

| Gate | Result |
| --- | --- |
| Maintained command migration scan | PASS |
| Wrapped removed-command regression | PASS |
| Isolated two-machine demo | PASS |
| Isolated canonical/removed-path CLI smoke | PASS |
| `make verify` | PASS |
| `make tui-verify` | PASS, 45 tests |
| `goreleaser check` | PASS |
| Full GoReleaser snapshot and ko image load | PASS |
| Four-platform archive validation | PASS |
| Adjacent companion launch | PASS |

## Artifact: Maintained Command-Surface Scan

**What it proves:** The allowlisted release documentation and active demo files
use the canonical vocabulary, while the negative fixture proves wrapped removed
paths are detected.

**Command**

```bash
scripts/check-command-surface.sh
scripts/check-command-surface-self-test.sh
```

**Result**

```text
command-surface: maintained documentation and demos use canonical commands
command-surface self-test: wrapped removed path rejected
```

## Artifact: Isolated Demo and CLI Smoke

**What it proves:** Canonical commands work together without network services or
existing user state, and removed commands do not resolve.

**Commands**

```bash
scripts/demo-check.sh

DEVSPACE_HOME=<tmp>/home bin/devspace init --workspace <tmp>/workspace
DEVSPACE_HOME=<tmp>/home bin/devspace scan
DEVSPACE_HOME=<tmp>/home bin/devspace project list --json
DEVSPACE_HOME=<tmp>/home bin/devspace status --verbose
DEVSPACE_HOME=<tmp>/home bin/devspace setup show
DEVSPACE_HOME=<tmp>/home bin/devspace setup run api --dry-run --yes
DEVSPACE_HOME=<tmp>/home bin/devspace experimental mount <tmp>/mount --preview
```

**Result summary:** The demo pushed and pulled only `manifest.json`, applied an
empty placeholder, reported `hydrate client-a-api: updated`, wrote a masked env
profile to a `0600` `.env`, and finished with one hydrated project. The focused
smoke printed an explicit project list, workspace overview, setup review,
`Would run npm install`, and a FUSE-free mount preview. It also rejected:

```text
devspace workspace
devspace project add
devspace env pull api
devspace setup plan
devspace hosted serve
devspace mount
```

The first demo run caught a stale assertion expecting the pre-formatter phrase
`Hydrated client-a-api`. The assertion was corrected to the stable formatter's
existing `hydrate client-a-api: updated` contract, and the full demo then passed.

## Artifact: Source and TUI Verification

**What it proves:** The final source passes the repository's local CI gates and
the bundled companion passes its typecheck and test suite.

**Commands**

```bash
make verify
make tui-verify
goreleaser check
```

**Result**

```text
command-surface: maintained documentation and demos use canonical commands
command-surface self-test: wrapped removed path rejected
ok github.com/liatrio-forge/devdrop-capstone/internal/devspace
go vet ./...
0 issues.
No vulnerabilities found.
go build -trimpath -o bin/devspace ./cmd/devspace

45 pass
0 fail
101 expect() calls

1 configuration file(s) validated
```

Toolchain evidence: Go `1.26.5`, Bun `1.3.14`, and GoReleaser v2 were used.

## Artifact: Full Snapshot and Archive Contract

**What it proves:** The actual release pipeline builds all supported CLI/TUI
pairs and completes the hosted ko image build without publishing anything.

**Commands**

```bash
make tui-build-all
DOCKER_HOST=unix://<active-docker-socket> \
  goreleaser release --snapshot --clean --skip=publish
scripts/verify-release-archives.sh dist
```

**Result**

```text
building dist/devspace-tui_linux_amd64
building dist/devspace-tui_darwin_amd64
building dist/devspace-tui_darwin_arm64
building dist/devspace-tui_linux_arm64

release succeeded

devspace_v0.3.0-SNAPSHOT-9e3b113_linux_amd64.tar.gz: devspace devspace-tui
devspace_v0.3.0-SNAPSHOT-9e3b113_linux_arm64.tar.gz: devspace devspace-tui
devspace_v0.3.0-SNAPSHOT-9e3b113_darwin_amd64.tar.gz: devspace devspace-tui
devspace_v0.3.0-SNAPSHOT-9e3b113_darwin_arm64.tar.gz: devspace devspace-tui
```

The checksum manifest contains four `devspace-tui_*` entries. Archive listings
show both binaries with executable mode in every archive.

The initial snapshot attempt used ko's default `/var/run/docker.sock` and
failed only at the image-load boundary because the active Docker daemon was on
the named local context socket. Re-running the identical snapshot with that
socket supplied completed all CLI builds, archives, checksums, and the ko image
load. This was an environment routing issue, not a source or packaging failure.

## Artifact: Adjacent Companion Smoke

**What it proves:** A release consumer can extract one archive and use `ui`
without an installer command.

**Method:** The `darwin_arm64` archive was extracted into an isolated directory,
its `devspace` initialized a temporary workspace, and `devspace ui --no-watch`
was launched under a controlled pseudo-terminal with the archive's companion
beside it.

**Result**

```text
adjacent companion rendered DevSpace
devspace version v0.3.0-SNAPSHOT-9e3b113
local archive smoke: pass
```

## Artifact: Overlapping Plan Reconciliation

**What it proves:** Plan statuses changed only after their acceptance criteria
were demonstrably satisfied.

- Plan 021 is `DONE (spec 13 task 4.0)`: release-check installs Bun, runs
  `make tui-build-all` before the snapshot, regression-tests the validator, and
  validates all produced archives.
- Plan 022 is `DONE (spec 13 task 5.0)`: README, architecture, and release
  readiness documentation use the shipped reconcile/FUSE/CI state; stale
  `future reconcile`, `global-only`, `macOS local proof pending`, and
  `manifest sync has no force flag` claims are absent.
- Plans 020 and 023 remain TODO because Spec 13 does not satisfy their separate
  runtime-validation and managed-hosting scopes.

## Reviewer Conclusion

The maintained release contract is internally consistent and reproducible:
canonical commands pass the local workflows and all required gates, removed
commands are rejected, and every supported archive contains a working adjacent
UI companion.
