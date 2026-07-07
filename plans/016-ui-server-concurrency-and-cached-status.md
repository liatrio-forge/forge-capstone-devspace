# Plan 016: Stop slow operations from blocking the ui-server request pipe, and stop `status` from doing a network git pull per call

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md` — unless a reviewer dispatched you and told you they
> maintain the index.
>
> **Drift check (run first)**: `git diff --stat 2ff060e..HEAD -- internal/devspace/ui_server.go internal/devspace/ui_actions.go internal/devspace/ui_model.go internal/devspace/ui_server_test.go internal/devspace/ui_test.go`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition.

## Status

- **Priority**: P1
- **Effort**: M
- **Risk**: MED
- **Depends on**: none
- **Category**: bug / perf
- **Planned at**: commit `2ff060e`, 2026-07-07

## Why this matters

`devspace ui` launches the `devspace-tui` companion, which talks to the hidden
`devspace ui-server` subcommand over stdio NDJSON JSON-RPC. Today the server
handles every request **sequentially inside its stdin read loop**, and the
`status` method does a **network `git pull`** on every call. The TUI client
fires `status` after every action and after every filesystem watch event. Net
effect: a `hydrate` (real `git clone`, can take minutes) or a single slow/offline
`status` blocks *every* queued request; innocent requests stall until the
client's 300s timeout; the dashboard feels frozen and the sync panel goes stale.
After this plan, read-only requests are answered concurrently, mutating actions
are single-flight with an immediate "busy" rejection (matching the built-in
dashboard's behavior), and sync status is served from a short-lived cache so
watch chatter no longer triggers repeated remote pulls.

## Current state

Relevant files:

- `internal/devspace/ui_server.go` — the stdio JSON-RPC server. `runUIServer`
  (line 122) reads lines and calls `srv.handle(req)` synchronously; `handle`
  (line 179) dispatches to the `dashboard*Cmd` closures.
- `internal/devspace/ui_actions.go` — shared domain closures used by both the
  Bubble Tea dashboard and the ui-server. `dashboardSyncStatusCmd` (line 128)
  computes sync status; the expensive part is `DiffWorkspaceManifest()` at
  line 167.
- `internal/devspace/ui_model.go` — the built-in Bubble Tea dashboard
  (`dashboardModel`). It re-requests sync status after every scan/action
  (`Update`, lines 145, 157, 204).
- `internal/devspace/workspace_sync.go` — `DiffWorkspaceManifest` (line 232)
  calls `fetchLocalizedWorkspaceRemoteManifest` (line 256), which runs
  `ensureManifestRepo` + `pullManifestRepo` — **a network git pull on every
  invocation**.
- `internal/devspace/ui_server_test.go` — existing server tests; use these as
  the structural pattern for new tests (they drive `runUIServer` with an
  `io.Pipe` and a JSON decoder).
- `tui/src/app.tsx` — the client; `refreshStatus()` is called after every
  completed action (line 60) and every watch-refresh event (line 75). No client
  change is required by this plan, but this is why `status` volume is high.

The read loop as it exists today (`ui_server.go:136-167`):

```go
reader := bufio.NewReader(r)
// stdin here is a trusted parent-child pipe to the devspace-tui client,
// not a network socket, so an unbounded per-line read is fine — nothing
// caps request size the way bufio.Scanner's fixed buffer would (and a
// scanner hitting that cap would kill the whole session).
// Requests are handled sequentially by design; that's the same
// single-flight guard the dashboard implements with startAction.
for {
    line, readErr := reader.ReadString('\n')
    if trimmed := strings.TrimSpace(line); trimmed != "" {
        var req uiServerRequest
        if err := json.Unmarshal([]byte(trimmed), &req); err != nil {
            srv.write(uiServerResponse{Error: &uiServerError{Message: "malformed request: " + err.Error()}})
        } else {
            result, err := srv.handle(req)
            ...
```

Note the comment claims sequential handling equals the dashboard's
single-flight guard. That's only true for mutating actions — the dashboard's
`startAction` guard (`ui_model.go:289-297`) never blocks *reads* (Bubble Tea
runs its `tea.Cmd`s on goroutines). This plan makes the server match that
model for real.

The dispatch (`ui_server.go:179-229`) routes: `hello`, `projects`, `status`,
`lastPlan` (read-only) and `scan`, `refresh`, `plan`, `apply`, `hydrate`
(mutating; all take the cross-process app lock via `runLocked` →
`withAppLock` inside their `dashboard*Cmd` closures).

The status closure (`ui_actions.go:128-179`): a `runLocked` section reads
config/state/reconcile plan, then — **outside the lock** —
`DiffWorkspaceManifest()` is called when a git manifest remote is configured.
There is already an existing test seam pattern on `uiServerOptions`:

```go
type uiServerOptions struct {
    Version  string
    NoWatch  bool
    SyncMode string

    watchCmdFactory func(string) tea.Cmd // test seam, same as dashboardModel
    watchRetryBase  time.Duration        // base backoff between watch retries; ...
}
```

The `uiServer` struct already serializes writes with a mutex
(`ui_server.go:115-120`), so concurrent handlers writing responses is safe:

```go
type uiServer struct {
    opts uiServerOptions

    mu  sync.Mutex // guards enc: responses and watch events interleave
    enc *json.Encoder
}
```

Concurrency safety fact you may rely on: the watch goroutine (`watchLoop`)
already runs `dashboardWatchRefresh` (which calls `runLocked` and file reads)
concurrently with request handling today. Mutating domain entry points
serialize on `withAppLock`; read paths (`LoadConfig`, `LoadManifest`,
`LoadState`, `LoadLastPlan`) are plain JSON file reads of atomically-written
files. Running read-only handlers concurrently does not introduce a new class
of race.

Repo conventions: single Go package `internal/devspace`; table-driven tests
with `t.Setenv("DEVSPACE_HOME", t.TempDir())` isolation (see
`ui_server_test.go:87` `TestUIServerRequestResponseFlow` for the exact
harness pattern). Comment style: explain *why*, tersely.

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| Test (package) | `go test ./internal/devspace -run TestUIServer -v` | all pass |
| Full gate | `make verify` | exit 0 (test+vet+lint+build) |
| One test | `go test ./internal/devspace -run <Name> -v` | pass |
| Race check | `go test ./internal/devspace -run TestUIServer -race` | pass, no races |

## Scope

**In scope** (the only files you should modify):
- `internal/devspace/ui_server.go`
- `internal/devspace/ui_actions.go`
- `internal/devspace/ui_model.go` (only the two lines wiring the status cache)
- `internal/devspace/ui_server_test.go`
- `internal/devspace/ui_test.go` (only if the status-cache wiring breaks an existing test's expectations)

**Out of scope** (do NOT touch, even though they look related):
- `internal/devspace/workspace_sync.go` — `DiffWorkspaceManifest` is shared by
  CLI commands (`devspace workspace diff`); caching belongs at the dashboard
  layer, not the domain layer.
- `tui/` — no client/protocol change is needed; the wire format is unchanged.
- `internal/devspace/watch.go`, `hosted_sync.go`.

## Git workflow

- Branch: `advisor/016-ui-server-concurrency` (repo uses conventional commits,
  e.g. `fix(ui-server): harden request loop and watch retry per PR #39 review`).
- Suggested message: `fix(ui-server): concurrent request handling and cached sync status`
- Do NOT push or open a PR unless the operator instructed it.

## Steps

### Step 1: Add a TTL cache for sync status in `ui_actions.go`

Add below `dashboardSyncStatusCmd`:

```go
// syncStatusCache memoizes dashboardSyncStatusCmd results so the TUI's
// status-after-every-event pattern doesn't trigger a remote git pull per
// filesystem change. Mutating actions call invalidate().
type syncStatusCache struct {
    mu       sync.Mutex
    ttl      time.Duration
    fetched  time.Time
    status   dashboardSyncStatus
    hasValue bool
}

const syncStatusCacheTTL = 30 * time.Second

func newSyncStatusCache() *syncStatusCache {
    return &syncStatusCache{ttl: syncStatusCacheTTL}
}

func (c *syncStatusCache) cmd() tea.Cmd {
    return func() tea.Msg {
        c.mu.Lock()
        if c.hasValue && time.Since(c.fetched) < c.ttl {
            status := c.status
            c.mu.Unlock()
            return syncStatusLoadedMsg{status: status}
        }
        c.mu.Unlock()
        msg := dashboardSyncStatusCmd()()
        loaded, ok := msg.(syncStatusLoadedMsg)
        if !ok {
            return msg
        }
        c.mu.Lock()
        c.status = loaded.status
        c.fetched = time.Now()
        c.hasValue = true
        c.mu.Unlock()
        return loaded
    }
}

func (c *syncStatusCache) invalidate() {
    c.mu.Lock()
    c.hasValue = false
    c.mu.Unlock()
}
```

Design notes (do not deviate): the cache holds the whole
`dashboardSyncStatus`, not the remote manifest — keep the shared domain path
untouched. Concurrent `cmd()` calls may both miss and both fetch; that is
acceptable (same as today's behavior) — do NOT add request coalescing.
`sync` and `time` are already imported in this file? `sync` is not — add it.

**Verify**: `go test ./internal/devspace -run TestDashboard -v` → existing tests still pass.

### Step 2: Wire the cache into both frontends

- `ui_model.go`: give `dashboardModel` a `statusCache *syncStatusCache` field,
  initialize it in `newDashboardModel` (`model.statusCache = newSyncStatusCache()`),
  and replace every `dashboardSyncStatusCmd()` call inside `Init`/`Update`
  (lines 124, 126, 145, 157, 204) with `m.statusCache.cmd()`. In
  `startAction` (line 289), after setting `m.busy = true`, call
  `m.statusCache.invalidate()` — mutating actions change the local manifest,
  so the next status must be fresh.
- `ui_server.go`: give `uiServer` a `statusCache *syncStatusCache` field,
  initialize it in `runUIServer`, use it in the `"status"` case, and call
  `s.statusCache.invalidate()` at the start of the `scan`, `refresh`, `plan`,
  `apply`, and `hydrate` cases.

**Verify**: `go test ./internal/devspace -run 'TestUIServer|TestDashboard' -v` → all pass.

### Step 3: Add a test seam for the action closures in `uiServerOptions`

Following the existing `watchCmdFactory` seam pattern, add to
`uiServerOptions`:

```go
// test seams; production fills these with the dashboard*Cmd defaults
scanCmd    func() tea.Cmd
refreshCmd func(string) tea.Cmd
planCmd    func() tea.Cmd
applyCmd   func() tea.Cmd
hydrateCmd func(string) tea.Cmd
statusCmd  func() tea.Cmd
```

In `runUIServer`, default each nil field to the real closure
(`dashboardScanCmd`, `dashboardRefreshCmd`, `dashboardPlanCmd`,
`dashboardApplyCmd`, `dashboardHydrateCmd`, and `srv.statusCache.cmd` for
status). Update `handle` to call through the options fields instead of the
package functions directly.

**Verify**: `go test ./internal/devspace -run TestUIServer -v` → all pass (behavior unchanged, seams defaulted).

### Step 4: Handle requests concurrently with single-flight actions

Rework the read loop and `handle`:

1. Classify methods: `hello`, `projects`, `status`, `lastPlan` are **reads**;
   `scan`, `refresh`, `plan`, `apply`, `hydrate` are **actions**.
2. Add to `uiServer`: `actionMu sync.Mutex` and `actionBusy string` (label of
   the in-flight action, guarded by `actionMu`).
3. In the read loop, after unmarshalling, dispatch **every** request on its
   own goroutine: `go srv.serve(req)`. `serve` calls `handle` and writes the
   response exactly as the loop does today (the `enc` mutex already makes
   concurrent writes safe).
4. At the top of each action case in `handle`, take the single-flight slot:

```go
func (s *uiServer) beginAction(label string) error {
    s.actionMu.Lock()
    defer s.actionMu.Unlock()
    if s.actionBusy != "" {
        return fmt.Errorf("busy: %s in progress", s.actionBusy)
    }
    s.actionBusy = label
    return nil
}

func (s *uiServer) endAction() {
    s.actionMu.Lock()
    s.actionBusy = ""
    s.actionMu.Unlock()
}
```

   An action request that arrives while another action runs gets an immediate
   error response `busy: <label> in progress` — do NOT queue it. This mirrors
   the dashboard's `startAction` guard message ("busy; wait for the current
   operation to finish") and protects against the client's known
   double-keypress race.
5. Update the now-stale comment above the read loop: requests are handled
   concurrently; actions are single-flight via `beginAction`; the unbounded
   `ReadString` rationale still applies — keep that part.
6. `runUIServer` must still return on stdin EOF exactly as today. In-flight
   goroutines writing after return is harmless (the process is about to exit;
   `write` ignores encode errors), but do not add synchronization for it —
   note this in a one-line comment.

**Verify**: `go test ./internal/devspace -run TestUIServer -race -v` → all pass, no data races.

### Step 5: New tests in `ui_server_test.go`

Model the harness after `TestUIServerRequestResponseFlow` (io.Pipe in/out,
`json.Decoder` on the output, `t.Setenv("DEVSPACE_HOME", ...)`). Add:

1. `TestUIServerReadsNotBlockedBySlowAction` — override `hydrateCmd` with a
   closure that blocks on a channel, then returns a valid `actionResultMsg`.
   Send `hydrate`, then `hello`. Assert the `hello` response (id 2) arrives
   while the hydrate is still blocked; then close the channel and assert the
   hydrate response (id 1) arrives.
2. `TestUIServerRejectsConcurrentActions` — with the same blocked `hydrateCmd`,
   send `hydrate` then `scan`. Assert the `scan` response is an error
   containing `busy: hydrate in progress`; unblock; assert hydrate completes
   successfully.
3. `TestUIServerStatusCachedWithinTTL` — override `statusCmd`? No — the cache
   wraps the real fetch, so instead: keep a counter closure as the underlying
   fetch by overriding `statusCmd` with `cache.cmd()` where the cache is
   constructed in the test around a counting fake. Simpler equivalent: test
   `syncStatusCache` directly in `ui_test.go` — two `cmd()()` calls within the
   TTL invoke the underlying fetch once (count via a stubbed
   `dashboardSyncStatusCmd`? it is a package function — instead refactor the
   cache to take the fetch function: `newSyncStatusCache(fetch func() tea.Msg)`
   with production passing `dashboardSyncStatusCmd()`). **Adopt this**: the
   cache takes its fetch function at construction; tests pass a counter.
   Assert: second call within TTL → 1 fetch; call after `invalidate()` → 2.

Note on test 3: apply the constructor-injection shape from the start in
Step 1 (i.e., write `newSyncStatusCache(fetch func() tea.Msg)` in Step 1 so
you don't rework it here).

**Verify**: `go test ./internal/devspace -run TestUIServer -race -v` and
`go test ./internal/devspace -run TestSyncStatusCache -v` → all pass.

## Test plan

Covered by Step 5. Existing tests that must keep passing unchanged:
`TestUIServerRequestResponseFlow`, `TestUIServerErrorPaths`,
`TestUIServerWatchEventPush`, `TestUIServerSyncModeWiring`,
`TestUIServerHandlesVeryLongRequestLine`, and all `TestDashboard*` tests.

## Done criteria

- [ ] `make verify` exits 0
- [ ] `go test ./internal/devspace -race -run 'TestUIServer|TestDashboard|TestSyncStatusCache'` exits 0
- [ ] New tests exist: slow-action-does-not-block-reads, concurrent-action-rejected, status-cache TTL/invalidate
- [ ] `grep -n "Requests are handled sequentially" internal/devspace/ui_server.go` returns no matches (comment updated)
- [ ] No files outside the in-scope list modified (`git status`)
- [ ] `plans/README.md` status row updated

## STOP conditions

Stop and report back (do not improvise) if:

- The read-loop code no longer matches the "Current state" excerpt (drift).
- `-race` reports a race in domain code (`workspace.go`, `state.go`, etc.) —
  that means a read path is not as concurrency-safe as assumed; do not paper
  over it with more locking in ui_server.go.
- You find yourself needing to modify `workspace_sync.go` or `tui/` to make a
  test pass.
- Making reads concurrent breaks `TestUIServerWatchEventPush` ordering
  assumptions in a way that requires changing the wire protocol.

## Maintenance notes

- Any new RPC method added to `handle` must be classified read vs action; an
  unclassified action would bypass single-flight.
- The 30s status TTL is a judgment call; if users report stale sync panels,
  lower it or invalidate on `watch-refresh` with `SyncChanged == true`.
- Plan 018 (watch robustness) builds on the goroutine dispatch introduced here;
  land this first.
- Reviewer scrutiny: the `beginAction`/`endAction` pairing in every action
  case (a missed `endAction` on an error path bricks all future actions —
  prefer `defer s.endAction()` immediately after a successful `beginAction`).
