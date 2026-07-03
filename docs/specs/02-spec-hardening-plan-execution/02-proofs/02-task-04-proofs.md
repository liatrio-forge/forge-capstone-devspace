# Task 04 Proofs - Concurrency, Scan, Watch, and Project Lifecycle Hardening

## Task Summary

Task 4 implemented Plans 009-012: app-home locking for mutating commands, monorepo scan descent, scoped watch refreshes, and `devspace project remove`.

## What This Task Proves

- Mutating CLI flows acquire an app-home lock while domain functions remain lock-free.
- Scans track a monorepo root once while preserving nested git repositories and standalone siblings.
- Watch refreshes can update changed projects without full rescans, with full-scan fallback metadata.
- Project removal cascades manifest access and state while preserving files and encrypted env profiles.

## Evidence Summary

- Targeted lifecycle tests passed.
- Watch tests passed under the race detector.
- Full `go test ./internal/devspace -race` passed.
- Temp-home smoke flows for `init`, `scan`, `watch --once`, `project add`, and `project remove` passed.
- Final `make verify` passed after lint fixes for lock unlock handling.

## Artifact: Targeted Lifecycle Tests

**What it proves:** Locking, scan, watch, project remove, and hardening tests pass together.

**Command:**

```bash
go test ./internal/devspace -run 'Lock|Scan|Watch|Project|Remove|Hardening' -v
```

**Result summary:** Passed. The run included `TestWithAppLockSerializesWriters`, `TestWithAppLockTimesOut`, `TestScanTreatsMonorepoAsOneProject`, `TestRefreshProjectsForWatchOnlyTouchesChangedProject`, and the project removal cascade tests.

## Artifact: Watch Race Tests

**What it proves:** Watch scoped refresh behavior remains race-clean.

**Command:**

```bash
go test ./internal/devspace -run 'Watch' -race -v
```

**Result summary:** Passed.

## Artifact: Full Internal Race Suite

**What it proves:** The new app lock and watch refresh paths do not introduce data races in the internal package.

**Command:**

```bash
go test ./internal/devspace -race
```

**Result summary:** Passed:

```text
ok  	github.com/HexSleeves/devspace/internal/devspace	25.583s
```

## Artifact: CLI Smoke Flow

**What it proves:** The built CLI can initialize, scan, run a one-shot watch refresh, add a project, and remove that project using isolated temp state.

**Command:**

```bash
go build -trimpath -o bin/devspace ./cmd/devspace
TMP_ROOT=$(mktemp -d)
DEVSPACE_HOME="$TMP_ROOT/home" ./bin/devspace init --workspace "$TMP_ROOT/code"
mkdir -p "$TMP_ROOT/code/apps/api" "$TMP_ROOT/code/apps/worker"
printf '{"scripts":{"dev":"vite"}}\n' > "$TMP_ROOT/code/apps/api/package.json"
DEVSPACE_HOME="$TMP_ROOT/home" ./bin/devspace scan
DEVSPACE_HOME="$TMP_ROOT/home" ./bin/devspace watch --once
DEVSPACE_HOME="$TMP_ROOT/home" ./bin/devspace project add tools/manual
DEVSPACE_HOME="$TMP_ROOT/home" ./bin/devspace project remove tools/manual
rm -rf "$TMP_ROOT"
```

**Result summary:** Passed. Sanitized excerpt:

```text
Initialized DevSpace workspace: [TMP]/code
Found 1 projects
0 Git repos
2 untracked folders
1 local-only projects
0 projects with env files
Watching [TMP]/code (4 directories)
Refreshed at [UTC_TIMESTAMP] (full): found 1 projects, 0 Git repos, 2 untracked folders, 1 local-only projects, 0 projects with env files.
Added project manual at tools/manual
Removed project manual (tools/manual) from the manifest. Files on disk were not touched.
```

## Artifact: Lock Placement Checks

**What it proves:** Locking is at CLI/watch entry points, not inside domain functions.

**Commands:**

```bash
grep -c "withAppLock" internal/devspace/commands.go
grep -n "withAppLock" internal/devspace/watch.go
grep -rn "withAppLock" internal/devspace/workspace.go internal/devspace/secrets.go internal/devspace/config.go || true
grep -n "os.Remove\|os.RemoveAll" internal/devspace/workspace.go
```

**Result summary:** `commands.go` contains 21 lock wrappers, `watch.go` wraps refresh execution, domain files contain no lock calls, and no new deletion calls were added for project removal. The remaining `os.Remove` calls are the pre-existing hydrate temp-dir cleanup and empty-placeholder replacement paths.

## Artifact: Full Verification Gate

**What it proves:** Repository tests, vet, lint, vulnerability scan, and build pass with Task 4 changes.

**Command:**

```bash
make verify
```

**Result summary:** Passed after fixing lint-reported unchecked `Unlock` returns.

## Artifact: Diff Summary

**What it proves:** Changes stayed focused on Plans 009-012 implementation, tests, dependency metadata, and SDD status/proof artifacts.

```text
go.mod
go.sum
internal/devspace/commands.go
internal/devspace/devspace_test.go
internal/devspace/lock.go
internal/devspace/lock_test.go
internal/devspace/watch.go
internal/devspace/watch_test.go
internal/devspace/workspace.go
plans/README.md
docs/specs/02-spec-hardening-plan-execution/02-tasks-hardening-plan-execution.md
docs/specs/02-spec-hardening-plan-execution/02-proofs/02-task-04-proofs.md
```

## Reviewer Conclusion

Task 4 is implemented and verified. Plans 009-012 are marked DONE, and the remaining SDD implementation work is Task 5.
