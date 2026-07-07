# Task 01 Proofs - ui-server answers reads during slow actions, rejects concurrent actions, and caches sync status

## Task Summary

This task removed the two root causes of the "frozen TUI": `devspace ui-server`
handled every request sequentially in its stdin read loop, and the `status`
RPC ran a network `git pull` on every call. Requests now run concurrently with
single-flight mutating actions (immediate `busy: <label> in progress`
rejection), and sync status is served from a 30-second TTL cache invalidated
whenever a mutating action starts — in both the ui-server and the built-in
Bubble Tea dashboard. Implemented per `plans/016-ui-server-concurrency-and-cached-status.md`
(executed by Codex under orchestrator review; gates run by the orchestrator).

## What This Task Proves

- Read-only requests (`hello`, `projects`, `status`, `lastPlan`) are answered
  while a long action (e.g. `hydrate`) is still running.
- A second mutating action is rejected immediately with
  `busy: <label> in progress` instead of queueing.
- Sync status is fetched at most once per 30s window and re-fetched after
  `invalidate()` (called at the start of every mutating action).
- The wire protocol is unchanged and the whole package is race-clean.

## Evidence Summary

- The three new behavior tests pass under `-race`.
- The full UI selection (30 tests) passes under `-race`.
- `make verify` (test + vet + lint + vulncheck + build) exits 0.
- The stale "requests are handled sequentially" design comment is gone.

## Artifact: New behavior tests pass under the race detector

**What it proves:** The three FR-mapped behaviors from spec Unit 1 —
non-blocking reads, busy rejection, cache TTL/invalidation — are implemented
and race-free.

**Why it matters:** These are the audit's traceability targets for Unit 1; the
race detector also guards the new goroutine-per-request dispatch.

**Command:**

~~~bash
go test ./internal/devspace -race -run 'TestUIServerReadsNotBlockedBySlowAction|TestUIServerRejectsConcurrentActions|TestSyncStatusCache' -v
~~~

**Result summary:** All three tests pass (0.13s, 0.13s, 0.00s); package result `ok`.

~~~text
=== RUN   TestUIServerReadsNotBlockedBySlowAction
--- PASS: TestUIServerReadsNotBlockedBySlowAction (0.13s)
=== RUN   TestUIServerRejectsConcurrentActions
--- PASS: TestUIServerRejectsConcurrentActions (0.13s)
=== RUN   TestSyncStatusCacheCachesWithinTTLAndInvalidates
--- PASS: TestSyncStatusCacheCachesWithinTTLAndInvalidates (0.00s)
PASS
ok  	github.com/liatrio-forge/devdrop-capstone/internal/devspace	1.295s
~~~

## Artifact: Full UI test selection race-clean

**What it proves:** The concurrency rework didn't regress any existing
ui-server or dashboard behavior.

**Command:**

~~~bash
go test ./internal/devspace -race -run 'TestUIServer|TestDashboard|TestSyncStatusCache'
~~~

**Result summary:** 30 tests passed, no data races reported.

~~~text
Go test: 30 passed in 1 packages
~~~

## Artifact: Full repository gate

**What it proves:** The change holds against the repo's local CI gate,
including lint and vulncheck.

**Command:**

~~~bash
make verify
~~~

**Result summary:** Exit 0. govulncheck: "0 vulnerabilities in packages you
import" (13 in required-but-uncalled modules, pre-existing); build produced
`bin/devspace`.

## Artifact: Stale sequential-design comment removed

**What it proves:** The in-code documentation now describes the concurrent
model (Unit 1 proof requirement).

**Command:**

~~~bash
grep -n "Requests are handled sequentially" internal/devspace/ui_server.go
~~~

**Result summary:** No matches (exit 1). The comment now reads "Requests run
concurrently; mutating actions are single-flight in handle."

## Reviewer Conclusion

Unit 1's functional requirements are implemented and proven: concurrent reads
during actions, immediate busy rejection, TTL-cached status with
invalidate-on-action in both frontends, wire protocol untouched, all gates
green under the race detector. Manual `devspace ui` smoke is deferred to the
end-of-spec check recorded in the audit FLAG disposition.
