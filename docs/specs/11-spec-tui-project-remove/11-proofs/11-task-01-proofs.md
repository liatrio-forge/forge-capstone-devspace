# Task 01 Proofs - ui-server Project Remove Request

## Task Summary

This task adds the backend JSON-RPC `remove` action for the external TUI. The action reuses `RemoveProject`, returns the refreshed dashboard snapshot, includes the removed project in the response, and exposes access advisory warnings without ANSI styling.

## What This Task Proves

- `devspace ui-server` accepts `remove` with a project ref and returns an updated snapshot.
- The remove path untracks manifest/access/state metadata but leaves project files on disk.
- Missing, blank, and unknown refs return user-facing errors.
- Existing `RemoveProject` domain behavior still passes its regression tests.

## Evidence Summary

- The new ui-server remove tests pass.
- Existing project removal domain tests pass.
- The red run failed with `unknown method "remove"` before implementation, proving the regression tests covered missing behavior.

## Artifact: RED test run before implementation

**What it proves:** The new tests failed for the expected reason before the backend remove method existed.

**Why it matters:** This satisfies the test-first requirement and proves the tests were not just confirming existing behavior.

**Command:**

```bash
GOCACHE=$(mktemp -d) go test ./internal/devspace -run 'TestUIServer.*Remove|TestUIServerErrorPaths' -v
```

**Result summary:** The remove tests failed with `unknown method "remove"`, which is the expected missing-feature failure.

```text
=== RUN   TestUIServerRemoveUntracksProjectAndLeavesFiles
    ui_server_test.go:196: unexpected error response: map[error:map[message:unknown method "remove"] id:1]
--- FAIL: TestUIServerRemoveUntracksProjectAndLeavesFiles (0.15s)
=== RUN   TestUIServerRemoveReturnsAccessAdvisoryWarnings
    ui_server_test.go:233: unexpected error response: map[error:map[message:unknown method "remove"] id:1]
--- FAIL: TestUIServerRemoveReturnsAccessAdvisoryWarnings (0.07s)
=== RUN   TestUIServerErrorPaths
    ui_server_test.go:266: remove no-ref error = "unknown method \"remove\""
--- FAIL: TestUIServerErrorPaths (0.13s)
FAIL
```

## Artifact: GREEN targeted backend tests

**What it proves:** The ui-server remove method and existing domain removal behavior pass together.

**Why it matters:** This proves the new TUI backend route uses the same safe manifest/access/state semantics as `devspace project remove`.

**Command:**

```bash
GOCACHE=$(mktemp -d) go test ./internal/devspace -run 'TestUIServer.*Remove|TestUIServerErrorPaths|TestRemoveProject' -v
```

**Result summary:** All targeted ui-server and domain removal tests passed.

```text
=== RUN   TestRemoveProjectUntracksAndCascades
--- PASS: TestRemoveProjectUntracksAndCascades (0.18s)
=== RUN   TestRemoveProjectByPathAndID
--- PASS: TestRemoveProjectByPathAndID (0.19s)
=== RUN   TestRemoveProjectNotFoundLeavesFilesUnchanged
--- PASS: TestRemoveProjectNotFoundLeavesFilesUnchanged (0.07s)
=== RUN   TestRemoveProjectRescanBehavior
--- PASS: TestRemoveProjectRescanBehavior (0.25s)
=== RUN   TestUIServerRemoveUntracksProjectAndLeavesFiles
--- PASS: TestUIServerRemoveUntracksProjectAndLeavesFiles (0.16s)
=== RUN   TestUIServerRemoveReturnsAccessAdvisoryWarnings
--- PASS: TestUIServerRemoveReturnsAccessAdvisoryWarnings (0.10s)
=== RUN   TestUIServerErrorPaths
--- PASS: TestUIServerErrorPaths (0.14s)
PASS
ok  	github.com/liatrio-forge/devdrop-capstone/internal/devspace	1.101s
```

## Reviewer Conclusion

The backend remove request is implemented and covered by failing-then-passing tests for safe untracking, advisory warning output, refreshed rows, and error handling.
