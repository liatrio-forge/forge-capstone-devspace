# Plan 018: Make the TUI client stream-decode correctly and make the watcher's failure modes visible and recoverable in both frontends

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md` — unless a reviewer dispatched you and told you they
> maintain the index.
>
> **Drift check (run first)**: `git diff --stat 2ff060e..HEAD -- tui/src internal/devspace/ui_server.go internal/devspace/ui_model.go internal/devspace/ui_test.go internal/devspace/ui_server_test.go tui/test`
> Plan 016 intentionally modifies `ui_server.go` before this plan runs — for
> that file, compare against the post-016 state described below. For all other
> files, a mismatch with the "Current state" excerpts is a STOP condition.

## Status

- **Priority**: P2
- **Effort**: M
- **Risk**: LOW
- **Depends on**: plans/016-ui-server-concurrency-and-cached-status.md
  (Step 4 below assumes ui-server requests already run on goroutines and the
  server struct exists in its post-016 shape)
- **Category**: bug / dx
- **Planned at**: commit `2ff060e`, 2026-07-07

## Why this matters

Five small defects make the TUI flow fragile in ways users see as "it just
stopped updating":

1. The client's stdout decoder drops `{stream: true}`, so a multi-byte UTF-8
   character split across pipe chunks corrupts an NDJSON frame; the response
   is silently discarded and the request hangs until its 300s timeout.
2. The built-in Bubble Tea dashboard re-arms a failed watcher instantly with
   no backoff or give-up — a persistently failing watcher (e.g. fd
   exhaustion) becomes a hot loop. The ui-server got exactly this hardening in
   PR #39; the legacy dashboard did not.
3. When the ui-server's watch loop exits silently (closed watcher channels),
   the client keeps showing "● watching" forever; and once the watcher gives
   up after 5 consecutive errors there is no way to restart it short of
   restarting the TUI.
4. Watch events emitted before the React app attaches its listener are
   dropped; the reducer also treats any unknown event type as a
   watch-refresh, which would blank the project table.
5. Cosmetics with real confusion potential: rapid keypresses can double-fire
   actions (stale `busy` read), and column padding is UTF-16-length-based so
   CJK/emoji project names shear the table.

## Current state

Relevant files:

- `tui/src/client.ts` — NDJSON client + `connect()` spawn helper.
- `tui/src/state.ts` — pure reducer (`reduce`, line 68). The `server-event`
  case (lines 85-100) handles `watch-error` then **assumes everything else is
  a watch-refresh**.
- `tui/src/app.tsx` — `runAction` busy guard (lines 44-49), event subscription
  in `useEffect` (lines 69-82), `cell()` padding helper (lines 419-422),
  status bar watch indicator (line 401).
- `internal/devspace/ui_server.go` — `watchLoop` (line 255 at planned-at
  commit; renumbered after plan 016). Silent exit paths at the top:

```go
msg := s.opts.watchCmdFactory(s.opts.SyncMode)()
if msg == nil {
    return
}
refresh, ok := msg.(watchRefreshMsg)
if !ok {
    return
}
```

- `internal/devspace/ui_model.go` — legacy dashboard watch retry
  (`Update`, lines 158-166):

```go
case watchRefreshMsg:
    m.busy = false
    if msg.err != nil {
        m.errText = msg.err.Error()
        if m.noWatch {
            return m, nil
        }
        return m, m.nextWatchCmd()   // <-- immediate re-arm, no backoff, no give-up
    }
```

  The retry constants already exist in `ui_server.go:109-113`:

```go
const (
    watchRetryDefaultBase = 1 * time.Second
    watchRetryMaxBackoff  = 30 * time.Second
    watchRetryMaxAttempts = 5
)
```

- The client decode bug, `tui/src/client.ts:173-180` — note the stderr loop
  just above it (line 163) uses `{ stream: true }` correctly, the stdout loop
  does not:

```ts
void (async () => {
  const decoder = new TextDecoder();
  for await (const chunk of proc.stdout) {
    client.feed(decoder.decode(chunk));   // <-- missing { stream: true }
  }
  const tail = stderrLines.join("\n").trim();
  client.closed(tail ? new Error(`devspace ui-server exited: ${tail}`) : undefined);
})();
```

- The busy-guard race, `tui/src/app.tsx:44-49`: `stateRef.current` only
  updates on render, so two keypresses in one tick both pass the check:

```ts
function runAction(method: ActionMethod, ref?: string) {
  if (stateRef.current.busy) {
    addToast("error", "busy; wait for the current operation");
    return;
  }
```

- The padding helper, `tui/src/app.tsx:419-422`:

```ts
export function cell(value: string, width: number): string {
  const truncated = value.length > width - 1 ? value.slice(0, Math.max(1, width - 2)) + "…" : value;
  return truncated.padEnd(width);
}
```

  The Go side solved the same problem rune-safely (`truncateCell`,
  `ui_model.go:425-434`, with test `TestDashboardTruncateCellRuneSafe`). Bun
  provides `Bun.stringWidth(str)` (terminal display width, wide-char aware) —
  use it; do not add a dependency.

Conventions: Bun tests in `tui/test/*.test.ts` using `bun:test`; Go tests
per `ui_server_test.go` harness (io.Pipe + json.Decoder + `t.Setenv`).

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| TUI tests | `cd tui && bun test` | all pass |
| TUI typecheck | `cd tui && bun run typecheck` | exit 0 |
| Go tests | `go test ./internal/devspace -run 'TestUIServer|TestDashboard' -race -v` | all pass |
| Full gates | `make verify && make tui-verify` | exit 0 |

## Scope

**In scope** (the only files you should modify):
- `tui/src/client.ts`, `tui/src/state.ts`, `tui/src/app.tsx`
- `tui/test/client.test.ts`, `tui/test/state.test.ts`
- `internal/devspace/ui_server.go`, `internal/devspace/ui_server_test.go`
- `internal/devspace/ui_model.go`, `internal/devspace/ui_test.go`

**Out of scope** (do NOT touch):
- `tui/src/protocol.ts` — no new event types; reuse `watch-error` (see Step 4).
  If you conclude a new event type is required, STOP.
- `internal/devspace/watch.go`, `ui_actions.go` (except: no exceptions).
- The request/response protocol and timeouts.

## Git workflow

- Branch: `advisor/018-tui-watch-robustness`
- Conventional commits; one commit per step is fine, e.g.
  `fix(tui): stream-decode stdout so split UTF-8 frames survive`.
- Do NOT push or open a PR unless the operator instructed it.

## Steps

### Step 1: Fix the stdout stream decode (client.ts)

Extract a tiny exported pump so it's testable, and use it for **both** stdout
and stderr:

```ts
/** Feed an async byte stream through a stateful UTF-8 decode into onText. */
export async function pumpText(stream: AsyncIterable<Uint8Array>, onText: (text: string) => void): Promise<void> {
  const decoder = new TextDecoder();
  for await (const chunk of stream) onText(decoder.decode(chunk, { stream: true }));
  const tail = decoder.decode();
  if (tail) onText(tail);
}
```

Rewire in `connect()`: stderr pump feeds the existing line-splitting logic
(keep `pushStderrLine`; you'll need to keep the line-buffering that currently
lives inline — move it into the callback), stdout pump feeds `client.feed`,
and the `client.closed(...)` call moves to after `await pumpText(proc.stdout, ...)`
resolves (same position in control flow as today).

Test (in `tui/test/client.test.ts`): build an async iterable of two
`Uint8Array` chunks that split the UTF-8 encoding of `"🚀"` (bytes
`f0 9f 92` / `80` — or encode `{"id":1,"result":"🚀"}\n` and slice mid-emoji),
pump it, and assert the reassembled text contains `"🚀"` with no `�`.

**Verify**: `cd tui && bun test client` → all pass including the new test.

### Step 2: Buffer events until the first listener attaches (client.ts)

In `DevspaceClient`, when an event arrives and `eventListeners.size === 0`,
push it onto a private `earlyEvents: ServerEvent[]` (cap 20, drop oldest).
In `onEvent`, after adding the listener, flush and clear `earlyEvents` to it.

Test: dispatch an event line via `feed()` before any listener, then attach a
listener and assert it receives the buffered event; assert a second listener
attached later does NOT get a replay.

**Verify**: `cd tui && bun test client` → all pass.

### Step 3: Fix the busy double-fire race and make the reducer defensive (app.tsx, state.ts)

- `app.tsx`: add `const busyRef = useRef(false)`. In `runAction`: check
  `busyRef.current || stateRef.current.busy` for the guard, set
  `busyRef.current = true` synchronously before dispatching `action-start`,
  and clear it in both `.then` handlers (success and error).
- `state.ts` `server-event` case: replace the implicit else with an explicit
  check — `if (event.type === "watch-refresh") { ...existing... }` and
  `return state;` for anything else (a newer server sending an unknown event
  must be a no-op, not a table wipe). Also set `watchAlive: true` in the
  watch-refresh branch — today a recovered watcher never clears the
  "watch stopped" indicator.

Tests (`tui/test/state.test.ts`): (a) an event with an unknown `type` leaves
state unchanged; (b) `watch-error` then `watch-refresh` flips `watchAlive`
back to `true`.

**Verify**: `cd tui && bun test && bun run typecheck` → all pass, exit 0.

### Step 4: Never let the ui-server watcher die silently; restart it on the next action (ui_server.go)

This step assumes the post-016 server (requests on goroutines, `beginAction`
single-flight). Changes:

1. In `watchLoop`, replace the two silent `return`s with an event first:

```go
if msg == nil {
    s.watchEnded("watcher closed")
    return
}
refresh, ok := msg.(watchRefreshMsg)
if !ok {
    s.watchEnded("watcher returned an unexpected result")
    return
}
```

   where `watchEnded(reason string)` emits the existing wire shape
   `{type: "watch-error", message: reason}` (reusing `watch-error` keeps
   protocol v1 unchanged) and records that the loop is gone. Also route the
   existing give-up path ("watcher stopped after %d consecutive errors")
   through `watchEnded`.
2. Track liveness: add `watchDown atomic.Bool` to `uiServer` (import
   `sync/atomic`). `watchEnded` sets it; the start of `watchLoop` clears it.
3. Restart on demand: after a **successful** `scan` or `refresh` request, if
   `!s.opts.NoWatch && s.watchDown.Load()`, start a fresh `go s.watchLoop()`
   and emit `{type: "watch-error", message: "watcher restarting"}`… no —
   emit nothing extra; the next `watch-refresh` event flips the client
   indicator back (Step 3 made the reducer do that). Guard against double
   restarts with `watchDown.CompareAndSwap(true, false)`.

Tests (`ui_server_test.go`, following `TestUIServerWatchErrorEndsLoop` /
`TestUIServerWatchErrorRecovers` patterns):
- `TestUIServerWatchClosedEmitsEvent`: `watchCmdFactory` returns a cmd whose
  msg is nil → expect a `watch-error` event containing "watcher closed".
- `TestUIServerScanRestartsDeadWatcher`: factory that fails
  `watchRetryMaxAttempts` times (loop gives up), then a successful `scan`
  request → assert the factory is invoked again (count invocations through
  the factory closure) and a subsequent `watch-refresh` event arrives.

**Verify**: `go test ./internal/devspace -run TestUIServerWatch -race -v` → all pass.

### Step 5: Port the watch backoff/give-up to the legacy dashboard (ui_model.go)

Give `dashboardModel` the same policy the server got in PR #39, reusing the
same constants:

- Add fields: `watchErrCount int`, `watchBackoff time.Duration`.
- In `Update`'s `watchRefreshMsg` error branch: increment `watchErrCount`;
  if `>= watchRetryMaxAttempts`, set
  `m.errText = fmt.Sprintf("watcher stopped after %d consecutive errors: %s", m.watchErrCount, msg.err)`
  and return `m, nil` (no re-arm). Otherwise compute the backoff (start at
  `watchRetryDefaultBase`, double, cap at `watchRetryMaxBackoff`) and re-arm
  with a delayed command:

```go
next := m.nextWatchCmd()
delay := m.watchBackoff
return m, func() tea.Msg { time.Sleep(delay); return next() }
```

- On a successful `watchRefreshMsg`, reset `watchErrCount` and `watchBackoff`.
- To keep tests fast, add a `watchRetryBase time.Duration` field on
  `dashboardModel` defaulted to `watchRetryDefaultBase` (mirror of the
  server's `opts.watchRetryBase` seam) and use it as the backoff base.

Tests (`ui_test.go`, following `TestDashboardWatchRefreshUpdatesModel`):
- error msg → returned cmd non-nil (re-armed) and count incremented;
- after `watchRetryMaxAttempts` consecutive error msgs → returned cmd is nil
  and `errText` contains "watcher stopped after";
- success resets the counter (send errors, then success, then error again →
  still re-arms).

**Verify**: `go test ./internal/devspace -run TestDashboard -race -v` → all pass.

### Step 6: Width-aware cells (app.tsx)

Replace `cell()` with a `Bun.stringWidth`-based version: measure with
`Bun.stringWidth(value)`; if it exceeds `width - 1`, truncate by code points
(iterate `for (const ch of value)`) accumulating width until `width - 2`,
append `"…"`; pad with spaces to `width` based on measured width (manual
spaces, not `padEnd`, since padEnd counts UTF-16 units). Keep the function
exported; add `tui/test/app-cell.test.ts`? No — the existing test files don't
import from app.tsx (it pulls opentui). Move `cell` into a new tiny module
`tui/src/text.ts` exporting `cell`, import it from `app.tsx`, and test it in
`tui/test/text.test.ts`: ASCII pad/truncate round-trips, an emoji name, and a
CJK name (`"日本語プロジェクト"`, width 10 → truncated string with
`Bun.stringWidth(result) === 10`).

**Verify**: `cd tui && bun test && bun run typecheck` → all pass; `make tui-verify` → exit 0.

## Test plan

Summarized from steps: split-UTF-8 pump test, early-event buffering test,
unknown-event no-op + watchAlive-recovery reducer tests, watcher-closed event
test, scan-restarts-watcher test, legacy backoff/give-up/reset tests,
width-aware cell tests. Existing suites must pass unchanged apart from tests
explicitly extended here.

## Done criteria

- [ ] `make verify` exits 0
- [ ] `make tui-verify` exits 0
- [ ] `go test ./internal/devspace -race -run 'TestUIServer|TestDashboard'` exits 0
- [ ] `grep -n "stream: true" tui/src/client.ts` shows the stdout path (via `pumpText`)
- [ ] `grep -n "return m, m.nextWatchCmd()" internal/devspace/ui_model.go` returns no matches (immediate re-arm gone)
- [ ] Reducer returns state unchanged for unknown event types (test exists)
- [ ] No files outside the in-scope list modified (`git status`)
- [ ] `plans/README.md` status row updated

## STOP conditions

Stop and report back (do not improvise) if:

- Plan 016 has not landed (no `beginAction` in `ui_server.go`) — Step 4's
  restart hook has nowhere to live; report instead of inventing a different
  concurrency model.
- `Bun.stringWidth` is unavailable in the pinned Bun version (CI pins 1.3.14;
  it should exist) — do not add a width dependency; report.
- You conclude a new server event type is needed (protocol change) — that
  belongs with plan 017's versioning procedure, not here.
- Any `-race` failure originating outside the files in scope.

## Maintenance notes

- The "restart watcher on next scan/refresh" behavior is deliberately
  implicit (no new RPC). If users want an explicit restart key, that's a new
  protocol method — follow plan 017's change procedure.
- `watch-error` now doubles as "watcher stopped" notification; if the client
  ever needs to distinguish transient vs terminal, that's the moment to add a
  `fatal: boolean` field (additive, non-breaking).
- Reviewer scrutiny: Step 4's CompareAndSwap guard (a scan racing a restart
  must not spawn two watch loops), and Step 5's delayed-command closure
  (capture `delay`/`next` by value, not via `m`).
