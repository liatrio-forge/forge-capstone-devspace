# Implementation Plans

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
| 016 | ui-server concurrent requests + cached sync status (no more network pull per watch event) | P1 | M | — | TODO |
| 017 | Protocol version handshake + Go/TS golden contract tests | P1 | M | — | TODO |
| 018 | TUI client stream-decode + watch failure visibility/recovery in both frontends | P2 | M | 016 | TODO |
| 019 | `devspace tui install` (download matching release asset to $DEVSPACE_HOME/bin) | P2 | M | — | TODO |

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

Current follow-up work is tracked in [`FOLLOWUP.md`](../FOLLOWUP.md) and `docs/superpowers/plans/`.

*(Note: The documentation backlog items, such as detailing `DEVSPACE_HOME`, `DEVSPACE_HOSTED_TOKEN`, and `release-readiness.md` sync, were directly resolved in the root `README.md` and repository docs.)*
