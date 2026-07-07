# 09-tasks-tui-flow-hardening.md

Tasks for `09-spec-tui-flow-hardening.md`. Each parent task executes one
advisor plan from `plans/` (Wave 2, planned at commit `2ff060e`); the plan
documents are the step-level implementation blueprints and carry the in-scope
file lists, STOP conditions, and drift checks that implementation must honor.
Execution order: 1.0 → 2.0 → 3.0 → 4.0 (3.0 hard-depends on 1.0; 2.0 and 4.0
are independent but sequenced to keep `ui_server.go` merges trivial).

Every sub-task inherits its plan's rules: run the plan's drift check first,
touch only the plan's in-scope files, and treat the plan's STOP conditions as
hard stops (report back instead of improvising).

## Relevant Files

| File | Why It Is Relevant |
| --- | --- |
| `internal/devspace/ui_server.go` | ui-server request loop, DTOs, watch loop — concurrency (1.0) and watch lifecycle (3.0) changes land here. |
| `internal/devspace/ui_actions.go` | Shared `dashboard*Cmd` domain closures — `syncStatusCache` (1.0) is added here. |
| `internal/devspace/ui_model.go` | Legacy Bubble Tea dashboard — status-cache wiring (1.0) and watch backoff parity (3.0). |
| `internal/devspace/ui_server_test.go` | Server test harness (io.Pipe + json.Decoder) — new concurrency (1.0) and watch-lifecycle (3.0) tests. |
| `internal/devspace/ui_test.go` | Dashboard tests — cache tests (1.0) and legacy backoff tests (3.0). |
| `internal/devspace/ui_protocol_fixtures_test.go` | New (2.0): golden-fixture generator/verifier with `-update-ui-fixtures` flag. |
| `internal/devspace/ui.go` | `devspace ui` command — fallback hint updated to mention `devspace tui install` (4.0). |
| `internal/devspace/commands.go` | Command wiring — one `AddCommand(newTUICommand(version))` line (4.0). |
| `internal/devspace/tui_install.go` | New (4.0): `devspace tui install` command and download/verify/install logic. |
| `internal/devspace/tui_install_test.go` | New (4.0): 7 httptest-backed install tests. |
| `.goreleaser.yaml` | Checksum block gains `extra_files` glob so tui binaries appear in `checksums.txt` (4.0). |
| `tui/src/client.ts` | NDJSON client — streaming decode via `pumpText` and early-event buffering (3.0). |
| `tui/src/state.ts` | Pure reducer — unknown-event no-op and `watchAlive` recovery (3.0). |
| `tui/src/app.tsx` | React app — hello handshake (2.0), `busyRef` double-fire guard, `cell` import swap (3.0). |
| `tui/src/main.tsx` | Renderer/quit owner — `quit(code, message?)` fatal-error plumbing (2.0). |
| `tui/src/protocol.ts` | TS protocol mirror — runtime type guards added, no shape changes (2.0). |
| `tui/src/text.ts` | New (3.0): width-aware `cell()` helper (moved out of app.tsx for testability). |
| `tui/test/client.test.ts` | Client tests — pump/split-UTF-8 and early-event tests (3.0). |
| `tui/test/state.test.ts` | Reducer tests — unknown-event and watchAlive-recovery cases (3.0). |
| `tui/test/protocol.test.ts` | New (2.0): fixture validation, negative guard case, version-lockstep assertion. |
| `tui/test/text.test.ts` | New (3.0): width-aware cell tests (ASCII, emoji, CJK). |
| `tui/test/fixtures/*.json` | New (2.0): six checked-in golden contract fixtures. |
| `plans/README.md` | Wave 2 status table — each parent task flips its plan's row to DONE. |
| `docs/specs/09-spec-tui-flow-hardening/09-proofs/` | Proof files capturing command outputs per parent task. |

### Notes

- Go tests live beside code in `internal/devspace/` (`*_test.go`), isolate
  state with `t.Setenv("DEVSPACE_HOME", t.TempDir())`, and are named after
  behavior (AGENTS.md). Run one test with
  `go test ./internal/devspace -run TestName -v`.
- TUI tests use `bun:test` under `tui/test/`. Gates: `make verify` (Go) and
  `make tui-verify` (Bun typecheck + tests). Pre-commit runs fmt/lint/test/build.
- Conventional Commits; branch names per plan (`advisor/NNN-…`). Do not push
  or open PRs unless the operator asks.
- Never commit real tokens; tests and proofs use placeholder values only
  (AGENTS.md security guidance).

## Tasks

### [x] 1.0 Make ui-server responsive: concurrent reads, single-flight actions, TTL-cached sync status (plan 016)

#### 1.0 Proof Artifact(s)

- Test: `go test ./internal/devspace -race -run 'TestUIServer|TestDashboard|TestSyncStatusCache'` passing, including new `TestUIServerReadsNotBlockedBySlowAction`, `TestUIServerRejectsConcurrentActions`, and sync-status cache TTL/invalidation tests, demonstrates FR: reads answered during a slow action, `busy: <label> in progress` rejection, and 30s cache with invalidate-on-action
- CLI: `make verify` exits 0 demonstrates the full Go gate (test+vet+lint+build) still holds
- CLI: `grep -n "Requests are handled sequentially" internal/devspace/ui_server.go` returns no matches demonstrates the stale sequential-design comment was replaced with the concurrent model's rationale
- Proof file: `docs/specs/09-spec-tui-flow-hardening/09-proofs/09-task-01-proofs.md` captures the command outputs above

#### 1.0 Tasks

- [x] 1.1 Run plan 016's drift check (`git diff --stat 2ff060e..HEAD -- <plan in-scope files>`); create branch `advisor/016-ui-server-concurrency`
- [x] 1.2 Add `syncStatusCache` to `ui_actions.go` with constructor-injected fetch (`newSyncStatusCache(fetch func() tea.Msg)`), 30s TTL, `cmd()` and `invalidate()` (plan 016 Steps 1+5 note)
- [x] 1.3 Wire the cache into `dashboardModel` (field, init, replace `dashboardSyncStatusCmd()` call sites, invalidate in `startAction`) and into `uiServer` (field, init, `status` case, invalidate at the start of scan/refresh/plan/apply/hydrate) (Step 2)
- [x] 1.4 Add action-closure test seams to `uiServerOptions` (`scanCmd`, `refreshCmd`, `planCmd`, `applyCmd`, `hydrateCmd`, `statusCmd`), defaulted in `runUIServer`, used by `handle` (Step 3)
- [x] 1.5 Dispatch every request on its own goroutine; add `beginAction`/`endAction` single-flight (immediate `busy: <label> in progress` rejection, `defer endAction` after successful begin); update the stale sequential-handling comment, keep the unbounded-ReadString rationale and EOF behavior (Step 4)
- [x] 1.6 Write `TestUIServerReadsNotBlockedBySlowAction`, `TestUIServerRejectsConcurrentActions`, and `TestSyncStatusCache` TTL/invalidate tests using the existing io.Pipe harness pattern (Step 5)
- [x] 1.7 Run gates (`go test -race` selection, `make verify`), write proof file `09-proofs/09-task-01-proofs.md`, flip plan 016's row to DONE in `plans/README.md`, commit

### [x] 2.0 Enforce the protocol contract: hello version handshake + Go/TS golden fixtures (plan 017)

#### 2.0 Proof Artifact(s)

- Test: `go test ./internal/devspace -run TestUIProtocolFixtures` passes without `-update-ui-fixtures` demonstrates the Go DTOs match the checked-in contract fixtures
- Test: `cd tui && bun test` passes including new `tui/test/protocol.test.ts` (six fixtures validated by runtime type guards, one negative guard case, `hello.protocol === PROTOCOL_VERSION` lockstep assertion, and `helloProblem` mismatch/success cases) demonstrates the TS side enforces the same contract and the handshake decision logic is behavior-tested
- CLI: `ls tui/test/fixtures/` lists the six fixture JSON files demonstrates the contract artifacts exist and are versioned
- CLI: `grep -n "PROTOCOL_VERSION" tui/src/app.tsx` shows the handshake check demonstrates mismatch produces a fatal, terminal-restored error
- Proof file: `docs/specs/09-spec-tui-flow-hardening/09-proofs/09-task-02-proofs.md` captures the command outputs above

#### 2.0 Tasks

- [x] 2.1 Run plan 017's drift check; create branch `advisor/017-protocol-contract`
- [x] 2.2 Create `ui_protocol_fixtures_test.go`: fully-populated DTO instances for hello, snapshot (plan+project), sync status, both event shapes, and error response; `-update-ui-fixtures` flag writes `tui/test/fixtures/`, default mode byte-compares and names the regeneration command on failure (Step 1); generate and check in the six fixtures
- [x] 2.3 Add runtime type guards `isHello`/`isSnapshot`/`isSyncStatus`/`isServerEvent` to `tui/src/protocol.ts` — explicit `typeof` checks, `null`-tolerant for `actions`/`warnings`, no new dependency — plus the pure handshake decision `helloProblem(hello: Hello): string | null` (returns an error message naming both protocol versions and the server version on mismatch; `null` on match) (Step 2, remediated)
- [x] 2.4 Create `tui/test/protocol.test.ts`: validate all six fixtures via the guards, assert `hello.protocol === PROTOCOL_VERSION` (lockstep), one negative case (deleted field → guard returns false), and `helloProblem` cases — mismatch returns a message naming both versions, match returns `null` (Step 3, remediated)
- [x] 2.5 Enforce the handshake: `app.tsx` routes any non-null `helloProblem(hello)` result — and any hello request failure — to the fatal-quit path; `main.tsx` `quit(code, message?)` restores the terminal before writing the message to stderr and exiting non-zero (Step 4, remediated)
- [x] 2.6 Run gates (`make verify`, `make tui-verify`), spot-check that deleting a fixture field fails `bun test` (restore it), write proof file `09-proofs/09-task-02-proofs.md`, flip plan 017's row in `plans/README.md`, commit

### [x] 3.0 Harden the client and watcher: stream-safe decode, visible/recoverable watch death, legacy backoff parity (plan 018 — requires 1.0)

#### 3.0 Proof Artifact(s)

- Test: `cd tui && bun test` passes including the split-UTF-8 pump test, early-event buffering test, unknown-event no-op + watchAlive-recovery reducer tests, and width-aware cell tests, demonstrates the client-side FRs
- Test: `go test ./internal/devspace -race -run 'TestUIServerWatch|TestDashboard'` passes including `TestUIServerWatchClosedEmitsEvent`, `TestUIServerScanRestartsDeadWatcher`, and legacy backoff/give-up/reset tests, demonstrates no silent watcher death, restart-on-next-action, and dashboard retry parity
- CLI: `grep -n "return m, m.nextWatchCmd()" internal/devspace/ui_model.go` returns no matches demonstrates the instant re-arm loop is gone
- CLI: `make verify && make tui-verify` exits 0 demonstrates both gates hold
- Proof file: `docs/specs/09-spec-tui-flow-hardening/09-proofs/09-task-03-proofs.md` captures the command outputs above

#### 3.0 Tasks

- [x] 3.1 Verify task 1.0 landed (`beginAction` exists in `ui_server.go` — plan 018's STOP condition otherwise); run plan 018's drift check; create branch `advisor/018-tui-watch-robustness`
- [x] 3.2 Extract exported `pumpText(stream, onText)` in `client.ts` with `TextDecoder` `{stream: true}` + final flush; use it for both stdout and stderr; add the split-emoji chunk test (Step 1)
- [x] 3.3 Buffer up to 20 pre-listener events in `DevspaceClient`, replay to the first listener only; add buffering/no-replay tests (Step 2)
- [x] 3.4 Add `busyRef` synchronous double-fire guard in `app.tsx`; make the reducer ignore unknown event types and set `watchAlive: true` on `watch-refresh`; add both reducer tests (Step 3)
- [x] 3.5 ui-server: `watchEnded(reason)` emits a `watch-error` event on every watcher exit path (closed channels, unexpected result, give-up); `watchDown atomic.Bool` + CompareAndSwap restart after successful `scan`/`refresh`; add `TestUIServerWatchClosedEmitsEvent` and `TestUIServerScanRestartsDeadWatcher` (Step 4)
- [x] 3.6 Port backoff/give-up to `dashboardModel` using the shared `watchRetry*` constants, delayed re-arm command, `watchRetryBase` test seam, reset on success; add backoff/give-up/reset tests (Step 5)
- [x] 3.7 Move `cell()` to new `tui/src/text.ts` using `Bun.stringWidth` (measure, truncate by code points, manual pad); update `app.tsx` import; add `tui/test/text.test.ts` (ASCII, emoji, CJK width-10 case) (Step 6)
- [x] 3.8 Run gates (`make verify`, `make tui-verify`, race-enabled Go selection), write proof file `09-proofs/09-task-03-proofs.md`, flip plan 018's row in `plans/README.md`, commit

### [x] 4.0 Ship `devspace tui install`: version-matched, token-aware, checksum-verified companion install (plan 019)

#### 4.0 Proof Artifact(s)

- Test: `go test ./internal/devspace -run TestTUIInstall -v` passes all 7 httptest-backed cases (happy path with checksum, checksum mismatch, no checksum coverage, missing asset, auth header with placeholder token, unsupported platform, dev-version guard) demonstrates the install FRs without network access
- CLI: `go run ./cmd/devspace tui install --help` output shows `--version` and `--repo` flags with defaults demonstrates the command exists and is documented
- Diff: `.goreleaser.yaml` checksum block containing the `tui/dist/devspace-tui_*` glob demonstrates future releases publish verifiable tui checksums
- CLI: `devspace ui` fallback hint text mentioning `devspace tui install` demonstrates the discovery path for users without the companion
- Proof file: `docs/specs/09-spec-tui-flow-hardening/09-proofs/09-task-04-proofs.md` captures the command outputs above (placeholder tokens only)

#### 4.0 Tasks

- [x] 4.1 Run plan 019's drift check; create branch `advisor/019-tui-install`
- [x] 4.2 Extend `.goreleaser.yaml` `checksum:` with `extra_files: [glob: tui/dist/devspace-tui_*]` (Step 1)
- [x] 4.3 Create `tui_install.go`: `newTUICommand(version)` / `newTUIInstallCommand(version)` with `--version` (default `v<version>`, required for `dev` builds) and `--repo` (default `liatrio-forge/devdrop-capstone`); install logic with injected API base URL — platform validation (linux/darwin × amd64/arm64 only), token from `GITHUB_TOKEN`/`GH_TOKEN`/`gh auth token`, release lookup by tag, asset download via API `url` with `Accept: application/octet-stream` (30s metadata / 5m download timeouts), sha256 verification against `checksums.txt` when covered (stated skip otherwise), same-dir temp + chmod 0755 + rename into `$DEVSPACE_HOME/bin/devspace-tui`, temp cleanup on all error paths, shadowing warning when `findTUIBinary()` resolves elsewhere; no token value in any output (Step 2)
- [x] 4.4 Wire `cmd.AddCommand(newTUICommand(version))` in `commands.go`; update the `ui.go` fallback hint to mention `devspace tui install` (Step 3)
- [x] 4.5 Write the 7 httptest-backed tests in `tui_install_test.go` (per plan 019 Step 4; placeholder token `test-token` only), including asserting error output never contains the token value in the failure cases (Step 4)
- [x] 4.6 Run gates (`make verify`, `go run ./cmd/devspace tui install --help`), write proof file `09-proofs/09-task-04-proofs.md`, flip plan 019's row in `plans/README.md`, commit; note in the proof file that the one-time manual smoke against a real release (plan 019 test plan) remains for a human with repo access
