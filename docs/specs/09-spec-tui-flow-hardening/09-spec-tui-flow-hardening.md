# 09-spec-tui-flow-hardening.md

## Introduction/Overview

The `devspace ui` flow launches the `devspace-tui` companion (OpenTUI/Bun/React
app in `tui/`), which spawns the hidden `devspace ui-server` subcommand and
talks stdio NDJSON JSON-RPC. A deep audit of this flow (2026-07-07, commit
`2ff060e`) found that slow operations block the entire request pipe, the
`status` RPC triggers a network `git pull` per call, the Go↔TypeScript
protocol has no enforced contract, several client/watch failure modes are
silent or unrecoverable, and there is no install path for the companion
binary. This feature executes advisor plans **016–019** (`plans/README.md`,
Wave 2) to make the TUI flow responsive, contract-safe, self-healing, and
installable with one command.

The four plan documents are the implementation blueprints and the detailed
source of truth for code-level decisions:

- `plans/016-ui-server-concurrency-and-cached-status.md`
- `plans/017-protocol-handshake-and-contract-tests.md`
- `plans/018-tui-client-and-watch-robustness.md`
- `plans/019-devspace-tui-install-command.md`

This spec defines the requirements, boundaries, and acceptance evidence; where
this spec and a plan conflict, stop and resolve with the user rather than
improvising.

## Goals

- Eliminate head-of-line blocking in `devspace ui-server`: read-only requests
  are answered while a long action (e.g. `hydrate`) runs; concurrent actions
  are rejected immediately with a clear "busy" error.
- Reduce sync-status cost: watch chatter and routine actions no longer trigger
  a remote `git pull` per event (TTL cache, invalidated by mutating actions).
- Make protocol drift between `internal/devspace/ui_server.go` and
  `tui/src/protocol.ts` impossible to ship: version handshake at connect time
  plus golden contract fixtures verified by both `go test` and `bun test` in CI.
- Make watcher failure visible and recoverable in both frontends, and make the
  NDJSON client byte-stream-safe (UTF-8 split across chunks, early events,
  double-fire guard, width-aware cells).
- Ship `devspace tui install`, which downloads the release asset matching the
  running devspace version into `$DEVSPACE_HOME/bin` with checksum
  verification when the release provides one.

## User Stories

- **As a devspace user on an active workspace**, I want the dashboard to stay
  responsive while a hydrate (git clone) runs so that I can keep navigating
  and see live status instead of a frozen UI and 300-second timeouts.
- **As a devspace user with a git manifest remote**, I want filesystem watch
  events to stop triggering repeated network pulls so that the TUI stays fast
  and usable offline.
- **As a user with mismatched `devspace`/`devspace-tui` versions**, I want one
  clear error telling me the versions don't speak the same protocol so that I
  don't debug blank panels caused by silent DTO drift.
- **As a contributor changing the ui-server protocol**, I want CI to fail on
  whichever side of the Go/TypeScript boundary drifted so that the lockstep
  rule in CLAUDE.md is enforced by tests, not memory.
- **As a devspace user whose watcher died**, I want the TUI to tell me and to
  recover on the next scan/refresh so that live updates come back without
  restarting the app.
- **As a new devspace user**, I want `devspace tui install` to fetch the right
  companion binary for my platform and version so that I get the full
  dashboard without hunting release assets by hand.

## Demoable Units of Work

### Unit 1: Responsive ui-server (plan 016)

**Purpose:** Requests no longer queue behind slow operations, and sync status
is served from a short-lived cache — the "TUI feels frozen" root cause is gone.

**Functional Requirements:**

- The system shall answer read-only requests (`hello`, `projects`, `status`,
  `lastPlan`) while a mutating action (`scan`, `refresh`, `plan`, `apply`,
  `hydrate`) is in flight.
- The system shall reject a mutating action that arrives while another is in
  flight with an immediate error of the form `busy: <label> in progress`
  (single-flight, no queueing).
- The system shall serve `status` from a 30-second TTL cache and shall
  invalidate that cache when any mutating action starts, in both the ui-server
  and the built-in Bubble Tea dashboard.
- The system shall preserve the existing wire protocol unchanged (no new
  methods, fields, or event types in this unit).

**Proof Artifacts:**

- Test output: `go test ./internal/devspace -race -run 'TestUIServer|TestDashboard|TestSyncStatusCache'` passing demonstrates concurrent reads, single-flight rejection, and cache TTL/invalidation behavior (new tests named in plan 016 Step 5).
- CLI: `make verify` exit 0 demonstrates the full Go gate (test+vet+lint+build) still holds.
- Grep: `grep -n "Requests are handled sequentially" internal/devspace/ui_server.go` returning no matches demonstrates the stale design comment was updated with the new model.

### Unit 2: Enforced protocol contract (plan 017)

**Purpose:** Version skew between independently-installed `devspace` and
`devspace-tui` produces one clear error, and DTO drift fails CI on either side.

**Functional Requirements:**

- The client shall compare `hello.protocol` against its `PROTOCOL_VERSION` and,
  on mismatch, restore the terminal and exit non-zero with a message naming
  both versions and advising matching releases.
- The client shall treat a failed `hello` request as fatal (no silent-swallow).
- The system shall provide golden JSON fixtures (checked in under
  `tui/test/fixtures/`) covering hello, snapshot (fully populated, including
  plan and project), sync status, both watch event shapes, and the error
  response shape.
- The Go test suite shall fail when a DTO's marshaled form no longer matches
  the fixtures (with a `-update-ui-fixtures` regeneration flag), and the Bun
  test suite shall fail when the fixtures no longer satisfy the TypeScript
  types (via runtime type guards), so both existing CI jobs enforce the
  contract without workflow changes.

**Proof Artifacts:**

- Test output: `go test ./internal/devspace -run TestUIProtocolFixtures` passing without the update flag demonstrates the Go side matches the checked-in contract.
- Test output: `cd tui && bun test` passing (including `protocol.test.ts` with its negative guard case and version-lockstep assertion) demonstrates the TS side matches the same contract.
- File listing: `ls tui/test/fixtures/` showing the six fixture files demonstrates the contract artifacts exist and are versioned.

### Unit 3: Client and watch robustness (plan 018)

**Purpose:** The flow's silent failure modes become visible and recoverable;
the client survives real-world byte streams and input timing.

**Functional Requirements:**

- The client shall decode stdout with streaming UTF-8 semantics so a
  multi-byte character split across pipe chunks cannot corrupt a frame or hang
  a request.
- The client shall buffer server events that arrive before the first listener
  attaches and replay them to that listener.
- The reducer shall ignore unknown server event types (no table wipe) and
  shall mark the watcher alive again when a `watch-refresh` arrives after a
  failure.
- The ui-server shall emit a `watch-error` event on every watcher exit path
  (no silent death) and shall restart a dead watcher after the next successful
  `scan` or `refresh` request.
- The built-in dashboard shall retry a failed watcher with the same
  exponential backoff and give-up policy the ui-server uses (shared
  constants), instead of re-arming instantly.
- The UI shall guard against double-fired actions from rapid keypresses and
  shall pad/truncate table cells by terminal display width
  (`Bun.stringWidth`), not UTF-16 length.

**Proof Artifacts:**

- Test output: `cd tui && bun test` passing, including the split-emoji pump test, early-event buffering test, unknown-event no-op test, and width-aware cell tests, demonstrates the client-side requirements.
- Test output: `go test ./internal/devspace -race -run 'TestUIServerWatch|TestDashboard'` passing, including watcher-closed-emits-event, scan-restarts-watcher, and legacy backoff/give-up/reset tests, demonstrates the server and legacy-dashboard requirements.
- Grep: `grep -n "return m, m.nextWatchCmd()" internal/devspace/ui_model.go` returning no matches demonstrates the instant re-arm is gone.

### Unit 4: Companion install command (plan 019)

**Purpose:** One command installs the version-matched companion binary, making
the full dashboard the default experience.

**Functional Requirements:**

- The system shall provide `devspace tui install` with `--version` (default:
  the running binary's release tag; required when running a `dev` build) and
  `--repo` (default `liatrio-forge/forge-capstone-devspace`) flags.
- The system shall download the `devspace-tui_<os>_<arch>` asset via the
  GitHub releases API, sending a Bearer token when available from
  `GITHUB_TOKEN`, `GH_TOKEN`, or `gh auth token` (the canonical repo is
  private), and shall install it atomically to
  `$DEVSPACE_HOME/bin/devspace-tui` with mode 0755.
- The system shall verify the asset's sha256 against the release's
  `checksums.txt` when it covers the asset, and shall state that verification
  was skipped when it does not; the GoReleaser config shall be extended so
  future releases include the tui binaries in `checksums.txt`.
- The system shall error clearly on unsupported platforms (only
  linux/darwin × amd64/arm64 are shipped) and shall never print a token value.
- The `devspace ui` fallback hint shall mention `devspace tui install`.

**Proof Artifacts:**

- Test output: `go test ./internal/devspace -run TestTUIInstall -v` passing (7 httptest-backed cases: happy path with checksum, checksum mismatch, no checksum coverage, missing asset, auth header, unsupported platform, dev-version guard) demonstrates the install logic without network access.
- CLI: `go run ./cmd/devspace tui install --help` output demonstrates the command, flags, and defaults exist.
- Diff: `.goreleaser.yaml` checksum block showing the `tui/dist/devspace-tui_*` glob demonstrates future releases publish verifiable checksums.

## Non-Goals (Out of Scope)

1. **Protocol v2 / new wire capabilities**: no new methods, fields, or event
   types; unit 3 deliberately reuses the existing `watch-error` event. A
   protocol change follows the change procedure unit 2 establishes.
2. **Removing or freezing the legacy Bubble Tea dashboard**: the
   freeze-vs-delete decision is an open direction item (see
   `plans/README.md`, Wave 2 rejected/deferred list); this work only ports the
   watch-retry hardening to it.
3. **Auto-update, background version checks, signature/attestation
   verification, and Windows support** for `devspace tui install` — explicitly
   deferred (plan 019 maintenance notes).
4. **Hydrate progress streaming**: deferred until large-repo clones are a
   reported pain.
5. **Changes to shared domain paths** (`workspace_sync.go`,
   `DiffWorkspaceManifest`, `watch.go` internals): caching lives at the
   dashboard layer only.

## Design Considerations

Terminal UI only; no visual redesign. User-visible surface changes are limited
to: the "busy" rejection toast, the protocol-mismatch fatal message, watcher
stopped/recovered status-bar states, correctly aligned columns for wide
(CJK/emoji) project names, and the updated `devspace ui` fallback hint. The
companion keeps the existing theme system (`tui/src/theme.ts`) untouched.

## Repository Standards

- Single Go package `internal/devspace`; commands wired in `commands.go`;
  thin entrypoint. Conventional commits (e.g.
  `fix(ui-server): …`, `feat(ui): …`).
- Go tests isolate state with `t.Setenv("DEVSPACE_HOME", t.TempDir())` and
  follow the io.Pipe + `json.Decoder` harness in `ui_server_test.go`;
  HTTP-facing tests use `httptest` (see hosted-sync/hardening tests).
- TUI tests use `bun:test` in `tui/test/`; gates are `make verify` (Go) and
  `make tui-verify` (Bun typecheck + tests) — both already run in CI
  (`.github/workflows/ci.yml`).
- Protocol DTO changes must update `tui/src/protocol.ts` in lockstep
  (CLAUDE.md rule — unit 2 turns this from convention into a test gate).
- Reuse existing helpers: `atomicWriteFile` (`jsonio.go:44`), `appHome()`
  (`paths.go`), watch-retry constants (`ui_server.go:109-113`).
- Each plan document specifies branch names (`advisor/NNN-…`), in-scope file
  lists, and STOP conditions — implementation must honor them.

## Technical Considerations

- **Concurrency model (unit 1)**: every request runs on its own goroutine;
  response writes are already mutex-serialized; mutating actions take a
  single-flight slot (`beginAction`/`endAction`). Read paths are plain JSON
  file reads of atomically-written files and already run concurrently with the
  watch goroutine today. Verified with `-race`.
- **Ordering (unit 1 → unit 3)**: plan 018's watcher-restart hook assumes plan
  016's server shape; execute 016 before 018. 017 and 019 are independent.
- **Contract testing (unit 2)**: golden fixtures generated by Go
  (`-update-ui-fixtures` flag), validated by hand-written TS type guards — no
  new runtime dependency on either side.
- **GitHub API (unit 4)**: release lookup by tag, asset download via the asset
  API `url` with `Accept: application/octet-stream` (the `browser_download_url`
  does not accept token auth on private repos); 30s metadata / 5m download
  timeouts mirroring `hosted_sync.go`'s client pattern; API base URL injected
  for httptest.
- **Bun**: CI pins 1.3.14; `Bun.stringWidth` and WHATWG `TextDecoder`
  streaming are available — no added dependencies.

## Security Considerations

- **Token handling (unit 4)**: GitHub tokens are read from environment or
  `gh auth token` at runtime only, sent solely as an Authorization header to
  the GitHub API/asset endpoints, and must never appear in output, errors,
  logs, tests, or committed files. Go's http.Client forwards Authorization
  only same-host on redirect, which is the desired behavior — do not override
  `CheckRedirect`.
- **Binary provenance (unit 4)**: sha256 verification against the release's
  `checksums.txt` when present; the GoReleaser change makes coverage the norm
  going forward. Attestation verification is a stated non-goal this round.
- **Filesystem safety**: installs write via same-dir temp file + rename
  (atomic, no partial binaries on the resolved path); temp files are removed
  on every error path. Existing `safeWorkspacePath` and hydrate ref validation
  are untouched.
- **No new network listeners**: ui-server remains stdio-only; the trusted
  parent-child pipe trust model (documented in `ui_server.go`) is preserved.

## Success Metrics

1. **Responsiveness**: with a hydrate in flight, a `hello`/`status` request is
   answered before the action completes (proven by
   `TestUIServerReadsNotBlockedBySlowAction`); zero repeat network pulls
   within the 30s status TTL (proven by the cache tests).
2. **Contract enforcement**: deleting any field from a golden fixture or
   bumping one side's protocol version fails CI (spot-checked during
   validation; the negative-guard and lockstep tests encode it permanently).
3. **Recoverability**: a dead watcher produces a visible event 100% of the
   time and resumes after the next successful scan/refresh (proven by
   `TestUIServerWatchClosedEmitsEvent` / `TestUIServerScanRestartsDeadWatcher`).
4. **Install UX**: `devspace tui install` on a supported platform yields an
   executable `$DEVSPACE_HOME/bin/devspace-tui` that `devspace ui` then
   prefers, in one command (proven by the httptest suite plus one manual smoke
   against a real release before the command is announced in release notes).
5. **Gates**: `make verify` and `make tui-verify` remain green throughout; all
   plan done-criteria checklists are satisfied.

## Open Questions

1. Non-blocking: the 30s status-cache TTL is a judgment call; plan 016 notes
   lowering it or invalidating on `SyncChanged` watch refreshes if users
   report stale sync panels. Default stands unless feedback says otherwise.
2. Non-blocking: whether the protocol-mismatch error message should also
   suggest `devspace tui install` once both land — a one-line follow-up noted
   in plan 019's maintenance notes, deliberately kept out of scope to keep the
   units independent.
3. Non-blocking assumption: release tags follow the `v<semver>` release-please
   convention (`v0.2.0` today); `--version` overrides if a tag ever deviates.
