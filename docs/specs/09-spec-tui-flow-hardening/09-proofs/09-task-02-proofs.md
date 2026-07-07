# Task 02 Proofs - protocol version handshake enforced and Go/TS contract locked by golden fixtures

## Task Summary

This task turned the CLAUDE.md "update `tui/src/protocol.ts` in lockstep"
convention into enforced gates: the client now hard-fails with one clear
message when `devspace ui-server` speaks a different protocol version, and six
checked-in golden JSON fixtures are verified by **both** `go test` (marshal
byte-compare with a `-update-ui-fixtures` regeneration flag) and `bun test`
(runtime type guards). Implemented per
`plans/017-protocol-handshake-and-contract-tests.md` plus the planning-audit
remediation (pure `helloProblem` decision function so the handshake logic is
behavior-tested). Executed by Codex under orchestrator review; gates run by
the orchestrator in an isolated worktree because a concurrent agent's
unrelated in-progress edits temporarily broke compilation of the shared
working tree.

## What This Task Proves

- Go DTO serialization matches the checked-in contract (`TestUIProtocolFixtures`).
- The TS types accept the same fixtures via runtime guards, and the guards
  actually discriminate (negative case).
- `PROTOCOL_VERSION` is enforced: `helloProblem` returns `null` on match and a
  message naming both versions on mismatch; `app.tsx` routes any problem — or
  a failed `hello` — to a terminal-restored fatal exit (code 1).
- Drift on either side of the boundary now fails CI (both existing CI jobs
  already run these suites).

## Evidence Summary

- `go test ./internal/devspace -run TestUIProtocolFixtures` passes without the
  update flag.
- `cd tui && bun test`: 22 pass, 0 fail (includes `protocol.test.ts` with
  fixture validation, negative guard case, lockstep assertion, and
  `helloProblem` mismatch/success cases).
- `make verify` and `make tui-verify` exit 0 (isolated worktree at HEAD + this
  task's files).
- Spot check: deleting `workspaceRoot` from `hello.json` makes `bun test` fail
  (2 failures), proving the contract gate bites; fixture restored.

## Artifact: Go-side contract verification

**What it proves:** Every DTO's marshaled form matches the six checked-in
fixtures; regeneration is explicit (`-update-ui-fixtures`), so accidental
drift fails.

**Command:**

~~~bash
go test ./internal/devspace -run TestUIProtocolFixtures -v
~~~

**Result summary:** 1 test passed (verifier mode, no update flag).

## Artifact: TS-side contract + handshake tests

**What it proves:** The fixtures satisfy the TS types via runtime guards, the
guards reject malformed data, and `helloProblem` implements the version
handshake decision (spec Unit 2 FRs, audit remediation).

**Command:**

~~~bash
cd tui && bun test
~~~

**Result summary:** 22 pass, 0 fail, 59 expect() calls across 3 files.

~~~text
 22 pass
 0 fail
 59 expect() calls
Ran 22 tests across 3 files. [121.00ms]
~~~

## Artifact: Contract gate bites on drift (negative spot check)

**What it proves:** A real field-level drift is caught, not just theorized.

**Command:**

~~~bash
python3 -c "delete workspaceRoot from tui/test/fixtures/hello.json"  # in disposable worktree
bun test test/protocol.test.ts
~~~

**Result summary:** 5 pass, **2 fail** with the field removed; fixture
restored afterward (main-repo fixtures verified intact: 6 files,
`workspaceRoot` present).

~~~text
 5 pass
 2 fail
 16 expect() calls
Ran 7 tests across 1 file. [6.00ms]
~~~

## Artifact: Handshake wired in the client

**What it proves:** Mismatch produces a fatal, terminal-restored error; hello
failure is no longer silently swallowed.

**Command:**

~~~bash
grep -n "helloProblem\|PROTOCOL_VERSION" tui/src/app.tsx tui/src/protocol.ts | head
~~~

**Result summary:** `app.tsx` calls `helloProblem(hello)` in the hello
handler and routes problems and rejections to `quit(message)`; `main.tsx`
writes `devspace-tui: <message>` to stderr after `renderer.destroy()` and
exits 1.

## Artifact: Full gates

**Command:**

~~~bash
make verify && make tui-verify
~~~

**Result summary:** Both exit 0 (run in an isolated worktree at HEAD plus this
task's files, to exclude a concurrent agent's unrelated in-progress edits;
`make verify` re-confirmed on the main tree at commit time by the Go suite
selection relevant to this task).

## Reviewer Conclusion

Unit 2's functional requirements are implemented and proven: the protocol
contract is now test-enforced on both sides of the Go/TypeScript boundary,
version skew fails fast with an actionable message, and the negative spot
check demonstrates the gate actually catches drift.
