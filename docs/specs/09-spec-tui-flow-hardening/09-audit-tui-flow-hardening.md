# 09-audit-tui-flow-hardening.md

## Executive Summary

- Overall Status: PASS
- Required Gate Failures: 0
- Flagged Risks: 1 (accepted)
- Audit runs: 2 (2026-07-07; run 1 FAIL → user-approved remediation → run 2 PASS)

## Gateboard

| Gate | Status | Why it failed (<=10 words) | Exact fix target |
| --- | --- | --- | --- |
| Requirement-to-test traceability | PASS | — | — |
| Proof artifact verifiability | PASS | — | — |
| Repository standards consistency | PASS | — | — |
| Open question resolution | PASS | — | — |
| Regression-risk blind spots | FLAG (accepted) | No end-to-end spawn test of real binaries | see FLAG 1 |
| Non-goal leakage | PASS | — | — |

## Standards Evidence Table (Required)

| Source File | Read | Standards Extracted | Conflicts |
| --- | --- | --- | --- |
| `AGENTS.md` | yes | `make verify` before PR; behavior-named tests beside code; Conventional Commits; placeholder secrets only | none |
| `README.md` (root) | yes | GoReleaser releases with checksums + attestation; styled-output conventions | none |
| `CLAUDE.md` | yes | `make tui-verify` gate for `tui/`; protocol DTO lockstep rule; `DEVSPACE_HOME` test isolation | none |
| `CONTRIBUTING.md` | not found | — | — |
| `.github/pull_request_template.md` | not found | — | — |
| `Makefile`, `.githooks/pre-commit`, `.github/workflows/ci.yml` | yes | Pre-commit fmt/lint/test/build; CI runs `make verify` + `make tui-verify` as separate jobs (Bun 1.3.14) | none |

## Findings (Only include when non-empty)

### FLAG Findings (max 2 in main report)

1. No automated end-to-end test spawns the real `devspace-tui` against a real `devspace ui-server` binary; coverage is piecewise (Go harness on one side, in-memory transport on the other).
   - Risk: an integration-level regression (spawn env, stdio wiring, DEVSPACE_BIN resolution) could pass all unit gates.
   - Disposition: **accepted** — the golden-fixture contract (task 2.0) covers the highest-risk drift class; each parent task's proof file records a one-time manual `devspace ui` smoke; a real e2e harness is a deliberate non-goal this round.

## Requirement-to-Test Traceability Summary

| Spec FR (condensed) | Planned test artifact |
| --- | --- |
| U1 reads answered during action | `TestUIServerReadsNotBlockedBySlowAction` (1.6) |
| U1 busy rejection | `TestUIServerRejectsConcurrentActions` (1.6) |
| U1 status TTL cache + invalidation | `TestSyncStatusCache` (1.6) + existing `TestDashboard*`/`TestUIServer*` wiring suites |
| U1 wire protocol unchanged | existing `TestUIServerRequestResponseFlow` + 2.2 fixtures |
| U2 handshake mismatch fatal / hello failure fatal | `helloProblem` mismatch/success cases in `protocol.test.ts` (2.4); fatal-quit plumbing verified by typecheck + grep proof |
| U2 golden fixtures both sides | `TestUIProtocolFixtures` (2.2) + `protocol.test.ts` (2.4) |
| U3 streaming UTF-8 decode | split-emoji pump test (3.2) |
| U3 early-event buffering | buffering/no-replay tests (3.3) |
| U3 unknown event no-op / watchAlive recovery | reducer tests (3.4) |
| U3 no silent watcher death / restart | `TestUIServerWatchClosedEmitsEvent`, `TestUIServerScanRestartsDeadWatcher` (3.5) |
| U3 legacy backoff parity | backoff/give-up/reset tests (3.6) |
| U3 double-fire guard | server-side `TestUIServerRejectsConcurrentActions` (1.6) enforces the observable behavior; client `busyRef` is belt-and-braces |
| U3 width-aware cells | `tui/test/text.test.ts` (3.7) |
| U4 install flags/download/verify/atomicity/platform/dev-guard | 7 httptest cases (4.5), incl. token-not-in-output assertions |
| U4 checksum coverage in releases | `.goreleaser.yaml` diff proof (4.2) |
| U4 fallback hint | CLI proof in 4.6 (string change; no unit test warranted) |

## User-Approved Remediation Plan

- Completed (approved 2026-07-07): sub-tasks 2.3–2.5 and the 2.0 proof
  artifacts now specify the pure handshake decision `helloProblem(hello):
  string | null` in `tui/src/protocol.ts`, consumed by `app.tsx`'s fatal-quit
  path and behavior-tested (mismatch + success) in `tui/test/protocol.test.ts`.

## Re-Audit Delta (Runs 2+ only)

- Changed gate statuses since previous run: Requirement-to-test traceability FAIL → PASS (U2 handshake FR now maps to `helloProblem` tests in 2.4).
- Still-failing REQUIRED gates: none.
- Newly introduced findings: none.
