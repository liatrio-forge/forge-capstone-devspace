# Implementation Plans

Generated and reconciled by the improve skill. Execute TODO plans in the order below unless dependencies say otherwise. Each executor: read the plan fully before starting, honor its STOP conditions, and update your row when done.

## Deep audit 2026-07-10 — selected plans

- Planned at: `7b521c3` on `main`.
- Audit scope: whole source repository across correctness, security,
  performance, tests, architecture, dependencies, DX, docs, and direction.
  Generated HTML/GIF/binaries, ignored dependencies, live GitHub/Railway state,
  release publication, and FUSE runtime integration were excluded.
- Verification at planning time:
  - `make verify` → pass.
  - `go test ./internal/devspace -race -count=1` → pass.
  - `cd tui && bun test && bun run typecheck` → 45 tests pass; typecheck pass.
  - Go statement coverage → 75.4%.
  - `govulncheck` → no called vulnerabilities.
- User selected findings 1-6. Existing plan 020 was refreshed rather than
  duplicated; plans 024-028 are new and keep numbering monotonic.

### Execution order & status

| Plan | Title | Priority | Effort | Depends on | Status |
|---|---|---|---|---|---|
| 024 | Publish a pending manifest commit when `sync push` is retried | P1 | S | — | DONE |
| 025 | Keep credentials out of project remotes and sync artifacts | P1 | M | — | TODO |
| 026 | Verify tagged source before publishing release artifacts | P1 | S | — | TODO |
| 027 | Serialize diff-cache and mount hydration mutations | P1 | M | — | TODO |
| 020 | Runtime-validate every devspace-tui RPC result and server event | P1 | M | — | TODO |
| 028 | Exit devspace-tui with an error when ui-server dies | P2 | S | 020 | TODO |

### Dependency notes

- 024-027 and 020 are independent; the table orders them by impact and
  shortest safe path.
- 028 follows 020 because both edit `tui/src/client.ts` and its tests; landing
  validation first avoids conflict and keeps transport-boundary behavior clear.

### Vetted findings table

| # | Finding | Category | Impact | Effort | Risk | Evidence | Plan |
|---|---|---|---|---|---|---|---|
| 1 | A retry after a failed Git manifest push can report unchanged without publishing the cached ahead commit. | correctness | HIGH | S | LOW | `internal/devspace/workspace_sync.go:128-174,418-434` | 024 |
| 2 | Credential-bearing HTTPS project remotes can enter manifests, sync storage, plan warnings, and clone errors. | security | HIGH | M | MED | `internal/devspace/git.go:55-58,134-151`; `internal/devspace/workspace.go:140-142,399-400` | 025 |
| 3 | The tag workflow publishes without verifying the exact tagged source. | release / DX | HIGH | S | LOW | `.github/workflows/release.yml:18-53`; `Makefile:115` | 026 |
| 4 | Diff/status cache mutation and FUSE hydration bypass the application lock. | correctness / concurrency | HIGH | M | MED | `internal/devspace/commands.go:477-486`; `internal/devspace/ui_actions.go:155-203`; `internal/devspace/mount.go:322-334` | 027 |
| 5 | Runtime ui-server failure exits devspace-tui successfully and discards diagnostics. | correctness | MED | S | LOW | `tui/src/client.ts:198-201`; `tui/src/app.tsx:94`; `tui/src/main.tsx:40-54` | 028 |
| 6 | TUI results and unsolicited events bypass existing runtime validators. | correctness / tests | MED | M | LOW | `tui/src/client.ts:102-130`; `tui/src/protocol.ts:312-393` | 020 |

### Deferred, not rejected

- Avoid Git subprocesses for every scanned directory.
- Add the race detector to CI.
- Test Railway deployment scripting before production mutation.
- Remove redundant whole-workspace refreshes from `project update --all`.
- Recognize `bun.lock` during setup detection.
- Pin privileged release actions to immutable commits.
- Qualify private-release provenance docs.
- Replace fixed watcher-test startup sleeps with readiness signaling.

These were audited but not selected for plans in this wave.

## Reconcile 2026-07-08 — status and audit

- Planned at: `cedcbc7` on branch `feat/11-tui-project-remove`.
- Existing plans `001`-`019`: kept as DONE; no duplicate plan created for completed hardening/TUI work.
- Verification sampled during reconciliation:
  - `go test ./... -count=1` → pass.
  - `go test ./internal/devspace -coverprofile=/tmp/devspace-cover.out -covermode=atomic` → pass, 73.5% statement coverage.
  - `cd tui && bun test` → pass, 45 tests.
  - `cd tui && bun run typecheck` → pass.
  - `goreleaser check` → pass.
  - `goreleaser release --snapshot --clean --skip=publish` → local dry-run reached ko image loading and stopped because Docker daemon was unavailable; release-check should validate that path on GitHub Actions.
- Audit scope: standard, hotspot-weighted. Read root docs/config, CI/release config, existing plans, architecture/spec docs, Go CLI hot paths, hosted sync, reconcile, setup, TUI protocol/client/state tests, and release workflow. Not a whole-repo deep line-by-line audit of generated HTML, demo artifacts, or vendored `tui/node_modules`.

## Wave 3 — reconciliation audit findings (2026-07-08)

### Execution order & status

| Plan | Title | Priority | Effort | Depends on | Status |
|------|-------|----------|--------|------------|--------|
| 020 | Runtime-validate every devspace-tui RPC result and server event | P1 | M | — | TODO (refreshed 2026-07-10) |
| 021 | Make release-check build devspace-tui assets before the GoReleaser dry-run | P1 | S | — | DONE (spec 13 task 4.0) |
| 022 | Reconcile README, architecture, and follow-up docs with shipped DevSpace state | P2 | S | — | DONE (spec 13 task 5.0) |
| 023 | Define the managed hosted sync production contract | P2 | M | 022 | TODO |

Status values: TODO | IN PROGRESS | DONE | BLOCKED (with one-line reason) | REJECTED (with one-line rationale — finding fixed independently or approach abandoned)

### Dependency notes

- 020 and 021 are independent.
- 022 can run anytime, but doing it after 020/021 lets the docs mention any extra verification if those land first.
- 023 should run after 022 so the production contract is not built on stale README/architecture claims.

### Vetted findings table

| # | Finding | Category | Impact | Effort | Risk | Evidence |
|---|---------|----------|--------|--------|------|----------|
| 1 | TUI RPC responses are not runtime-validated at the client boundary even though validators exist. | correctness / tests | Protocol drift can enter React state as typed data and fail later with unclear UI errors. | M | LOW | `tui/src/client.ts:124-130`; `tui/src/protocol.ts:312-349`; `tui/src/protocol.ts:381-393` |
| 2 | `release-check` does not build `devspace-tui` assets before the GoReleaser dry-run. | dx / release | Release-config PRs do not exercise the TUI extra-file path used by real releases. | S | LOW | `.github/workflows/release-check.yml:26-36`; `.github/workflows/release.yml:40-51`; `.goreleaser.yaml:49-52,71-76` |
| 3 | Root docs still describe shipped reconcile/FUSE work as pending. | docs | Future agents and maintainers can plan duplicate work or trust stale limitations. | S | LOW | `README.md:87-93,416-423`; `ARCHITECTURE.md:173-175,221-229,248-251`; `FOLLOWUP.md:7`; `docs/operations/release-readiness.md:57-67` |

### Direction findings

- Managed hosted sync remains the largest product direction item, but it is intentionally still a prototype (`README.md:398`, `ARCHITECTURE.md:257`). Keep it as roadmap until token-to-user auth, deployment ownership, and service operations are chosen.
- Hydration progress streaming was explicitly deferred in spec 09 until large-repo clones become reported pain. Do not plan it before user feedback; `hydrate` already has generous request timeouts and safe clone behavior.
- Production app shape: keep the CLI local-first and make managed hosted sync the production boundary. Plan 023 is a docs/design spike only; it avoids a broad SaaS rewrite and turns the hosted prototype into small future slices.

### Findings considered and rejected

- Reopen plans `001`-`019`: rejected; `plans/README.md` marks them DONE and sampled tests/docs support current completion.
- `findTUIBinary` PATH lookup: rejected; `internal/devspace/ui.go` documents adjacent binary and `$DEVSPACE_HOME/bin` precedence, then PATH as the usual CLI trust model.
- ui-server unbounded `ReadString`: rejected; `internal/devspace/ui_server.go` documents trusted parent-child pipe rationale, and plan 018 already covered the risk.
- `goreleaser release --snapshot --clean --skip=publish` local failure: not a source finding; the observed failure was missing local Docker daemon for ko image loading.

This document tracks the execution and status of Liatrio Spec-Driven Development (SDD) plans for DevSpace.

## Wave 2 — TUI flow & setup audit (2026-07-07, planned at `2ff060e`)

Deep focused audit of the `devspace ui` flow: `tui/` (OpenTUI companion),
`internal/devspace/ui*.go` (ui-server + legacy dashboard), and their
CI/release wiring. Execute in the order below. Each executor: read the plan
fully before starting, honor its STOP conditions, and update your row when
done.

### Execution order & status

| Plan | Title | Priority | Effort | Depends on | Status |
|------|-------|----------|--------|------------|--------|
| 016 | ui-server concurrent requests + cached sync status (no more network pull per watch event) | P1 | M | — | DONE (spec 09 task 1.0) |
| 017 | Protocol version handshake + Go/TS golden contract tests | P1 | M | — | DONE (spec 09 task 2.0) |
| 018 | TUI client stream-decode + watch failure visibility/recovery in both frontends | P2 | M | 016 | DONE (spec 09 task 3.0) |
| 019 | `devspace tui install` (download matching release asset to $DEVSPACE_HOME/bin) | P2 | M | — | DONE (spec 09 task 4.0; superseded by spec 13 task 4.0 bundled UI companion) |

Status values: TODO | IN PROGRESS | DONE | BLOCKED (with one-line reason) | REJECTED (with one-line rationale)

### Dependency notes

- 018 requires 016: its watcher-restart hook assumes the goroutine-dispatch +
  single-flight (`beginAction`) server shape introduced in 016.
- 017 and 019 are independent; if 016/017 land in either order expect only a
  trivial merge in `ui_server.go` test imports.

### Findings considered and rejected (do not re-audit)

- Unbounded `ReadString` in the ui-server read loop — documented, deliberate
  (trusted parent-child pipe; a capped Scanner would kill sessions).
- PATH lookup in `findTUIBinary` — documented trust-model decision in
  `ui.go:52-57`.
- Early watch events dropped / busy double-fire / non-width-aware `cell()` —
  not rejected; folded into plan 018 as included cleanups.
- Direction: legacy Bubble Tea dashboard freeze-vs-delete decision — surfaced,
  not planned this wave (owner call; the drift evidence is in plan 018's
  "Why this matters").
- Direction: hydrate progress streaming — deferred, low value until large-repo
  clones are a reported pain.

---

## 🎉 Wave 1 Status: All Initial Plans Completed

All implementation plans from the original SDD execution against commit `595d158` have been successfully completed and integrated into the repository. The hardening passes, FUSE spikes, and structural refactors were addressed comprehensively.

### Completed Execution Order

| Plan | Title | Priority | Effort | Status |
| ---- | ----- | -------- | ------ | ------ |
| 001 | Reject unsafe manifest project IDs (secrets path traversal) | P1 | S | DONE |
| 002 | Atomic writes for secrets, .env, and age identity | P1 | S | DONE |
| 003 | Safety-net tests: symlink escape + recipient listing | P1 | S | DONE |
| 004 | Validate manifest-supplied git remotes before clone | P2 | S | DONE |
| 005 | Hosted client: re-validate endpoint at use; env-var token | P2 | S | DONE |
| 006 | Fix mergeProject so rescans preserve user overrides | P1 | S | DONE |
| 007 | Bound hosted server's per-workspace mutex map (striped locks) | P2 | S | DONE |
| 008 | CI/Makefile lint + gofmt + govulncheck gates | P2 | S | DONE |
| 009 | Cross-process app-home locking for mutating commands | P2 | M | DONE |
| 010 | Scan: one project per monorepo, not per nested package.json | P2 | M | DONE |
| 011 | Watch: scoped refresh instead of full rescan per event | P3 | M | DONE |
| 012 | `devspace project remove` (untrack + cascade) | P2 | M | DONE |
| 013 | SPIKE: manifest conflict reconciliation design + prototype | P3 | M | DONE |
| 014 | SPIKE: access-role posture decision doc | P3 | M | DONE |
| 015 | SPIKE: FUSE-capable CI go/no-go + mount backlog | P3 | M | DONE |

---

## 📝 SDD Execution Notes & History

The following notes trace the historical completion of the SDD phases.

- **Phase 1 (Plans 001, 002, 003, 006):** Completed with targeted safety tests and coverage checks for recipient listing and export. Included atomic writes and merge overrides.
- **Phase 2 (Plans 004, 005, 007, 008):** Addressed Git remote validation, hosted endpoint environment token (`DEVSPACE_HOSTED_TOKEN`), bounded mutexes, and CI lint gates.
- **Phase 3 (Plans 009, 010, 011, 012):** Delivered cross-process app-home locking, monorepo scan descent, scoped watch refresh, and the `devspace project remove` command with full race and smoke tests.
- **Phase 4 (Spikes 013, 014, 015):** Completed via follow-up SDD-HTML specs. PR #25 validated the Linux hosted FUSE probe and integrated the mount CI job while keeping the default path FUSE-free.

---

## 🗃️ Backlog & Deferred Findings

The operational items discovered during planning have all since been resolved:

- **Extract `hosted serve` HTTP lifecycle out of `commands.go`:** DONE — landed in PR #27 (`hosted_sync.go`).
- **Dependabot/Renovate for `go.mod`:** DONE — `.github/dependabot.yml` tracks `gomod` and `github-actions` weekly. (The pinned `ko` base-image digest in `.goreleaser.yaml` is still bumped manually.)
- **Clean up dead code (`PlanSync`/`ApplySync`):** DONE — the wrappers no longer exist in `internal/devspace`.

Current follow-up work is tracked in the TODO tables above and
`docs/superpowers/plans/`.

*(Note: The documentation backlog items, such as detailing `DEVSPACE_HOME`, `DEVSPACE_HOSTED_TOKEN`, and `release-readiness.md` sync, were directly resolved in the root `README.md` and repository docs.)*
