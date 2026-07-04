# Implementation Plans

This document tracks the execution and status of Liatrio Spec-Driven Development (SDD) plans for DevSpace.

## 🎉 Status: All Initial Plans Completed

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

A few operational items were discovered during planning but were not part of the core SDD implementation. They have been recorded here as backlog items for future consideration:

- **Extract `hosted serve` HTTP lifecycle out of `commands.go`:** Move the `http.Server` signals/shutdown out of the CLI layer to improve testability.
- **Dependabot/Renovate for `go.mod`:** Set up Dependabot to track `go.mod` updates and the pinned `ko` base-image digest in `.goreleaser.yaml`.
- **Clean up dead code (`PlanSync`/`ApplySync`):** Delete these thin wrappers in a future refactor, as they lack production callers and are only exercised in tests.

*(Note: The documentation backlog items, such as detailing `DEVSPACE_HOME`, `DEVSPACE_HOSTED_TOKEN`, and `release-readiness.md` sync, were directly resolved in the root `README.md` and repository docs.)*
