# DevSpace Follow-ups Work Plan

Consolidated from a full docs/plans sweep on 2026-07-05 (capstone docs, specs
01–06 validations/audits, operations + architecture docs, `plans/` card index).
Each card is scoped to be executable on its own. Owner decisions are called out
explicitly — nothing here assumes them.

## Already closed by the doc-sync PR (2026-07-05)

- README roadmap updated: reconciliation shipped (spec 06), prototypes named as prototypes.
- Capstone README "Remaining Module 5 Work": tagged release + `.devspace` migration moved to done.
- `docs/operations/release.md`: DevDrop → DevSpace header; added "Recovering from a failed release" (closes spec-01 audit FLAG 2).
- `docs/capstone/index.html`: removed nonexistent `make release` / `dist/SHA256SUMS` claims, fixed dead `docs/release.md` links (closes spec-01 audit FLAG 1 remnant).
- `plans/README.md` backlog: all three items verified done (PR #27 extraction, dependabot gomod, dead wrappers deleted).
- The `workspace diff` → `workspace pull` false-refusal bug: fixed and merged in PR #31.

Deliberately left alone: `CHANGELOG.md` stub (pointing at GitHub Releases is
the release-please flow, not staleness) and the spec-04 tasks-doc file-list
drift (validation rates it optional; SDD artifacts are historical records).

---

## F1 — macOS FUSE local proof (P1 · S · requires Jacob's Mac)

**Why first:** the single item blocking two specs. `docs/architecture/fuse-lazy-mount.md`
carries a PENDING local-proof marker, and spec 02's Plan 015 validation gap
("PASS WITH GAP") needs an observed FUSE run. Hosted macOS runners are already
ruled out, so a local run is the only unlock.

**Work:**
1. Install macFUSE, approve the kernel/system extension.
2. Run `docs/operations/macos-fuse-run-playbook.md` end to end: mount, list
   placeholders, lazy-hydrate one project, unmount, run diagnostics.
3. Capture the terminal session as a proof artifact (vhs tape or transcript)
   under `docs/specs/03-spec-fuse-lazy-mount/03-proofs/`.
4. Flip the PENDING marker in `fuse-lazy-mount.md`; record Plan 015 closure in
   `docs/specs/02-spec-hardening-plan-execution/02-validation-hardening-plan-execution.md`.

**Acceptance:** proof artifact committed; both docs updated; spec 02 no longer
"PASS WITH GAP".

**Can't be delegated:** needs physical hardware and a GUI security-approval
click. Everything after step 2 can be agent-driven.

## F2 — Close spec-01 attestation finding M-1 (P2 · XS · owner decision)

**Decision required:** make the repo public, or accept M-1 permanently.

- If public: run `gh attestation verify checksums.txt --repo liatrio-forge/devdrop-capstone`
  against the latest release assets and record the passing output in
  `docs/specs/01-spec-cicd-goreleaser/01-validation-cicd-goreleaser.md`.
- If it stays private: amend the validation to mark M-1 "accepted — blocked by
  repo visibility policy" so it stops surfacing as open work.

Either path is under 30 minutes; the only real content is the visibility call.

## F3 — Capstone wave-5 deliverables (P1 by deadline · M)

From `proof-artifacts.md` and the capstone README. These run on the wave-5
clock regardless of engineering work:

1. Finalize `case-study.md` and `remote-agent-case-study.md` (both "Drafted").
2. Record the demo per `demo-script.md` — the per-feature vhs tapes/gifs in
   `docs/demos/` already cover the segments; the remaining work is the narrated
   end-to-end recording.
3. Update `proof-artifacts.md` rows from Pending/Drafted with final links.
4. Personal reflection after demo feedback (blocked on 2).
5. **Owner decision:** execute the stretch cards via a wave-ship run or
   manually — or explicitly descope them. Recording the decision is itself the
   deliverable; the cards stop being ambient "remaining work".

## F4 — Spec 07: access-role warning tier (P2 · M · design phase first)

`docs/architecture/access-roles.md` is explicit that roles are advisory-only
and lists seven open design questions plus concrete follow-up cards. This is
the natural next SDD spec, but it is **not implementation-ready**: the seven
questions (grant precedence, member role capping, unknown-user handling,
developer push rights, viewer env access, hosted enforcement timing,
field-level merge interaction) must be answered in the spec's questions phase
before tasks exist.

**Scope for the spec (from the doc's own follow-up cards):**
- Computed `effectiveRole` surfaced as warnings only — no enforcement.
- CLI warnings on risky mutations (e.g. a viewer-scoped identity editing env
  profiles or manifest records).
- Doc language updated to consistently say "advisory".
- Explicitly out of scope: hosted per-user tokens (its own spec later) and any
  hard enforcement.

**Kickoff:** run the SDD workflow (`/sdd-html`) with the seven questions as the
questions-phase input. Do not start tasks from this card directly.

## F5 — Reconcile scope extensions (P3 · M · after F4)

ARCHITECTURE.md records spec 06's deliberate cuts. In value order:

1. **Surface sync/reconcile in `devspace ui`** — the dashboard currently shows
   local state only; pull/push/diff/reconcile status is invisible. Highest
   user-visible payoff, moderate effort (spec 05 laid the TUI plumbing).
2. **Per-project force resolution** — `reconcile --force` is global-only today.
   Needs a small UX design (per-project selection flags or interactive pick).
3. **Deferred unless demanded (YAGNI):** field-level users/teams merge and
   machines reconciliation — record-level merge has no reported failure cases.

Ship 1 and 2 as one spec 08 or two small specs; don't start 3 without a
concrete conflict report.

## F6 — Backlog nits (P3 · XS · opportunistic)

- `ko` base-image digest in `.goreleaser.yaml` is still bumped manually
  (dependabot's gomod ecosystem doesn't see it). Options: a scheduled digest
  bump workflow, or accept manual bumps — decide when it next goes stale.
- Access-roles doc language pass ("advisory" wording) can ride along with F4
  rather than shipping alone.

## Sequencing

```
now:        doc-sync PR (this)     F2 decision (visibility)
this week:  F1 (Mac proof)         F3.1–F3.3 (capstone artifacts)
next:       F4 spec-07 kickoff  →  F5 spec-08 (ui surfacing, per-project force)
ambient:    F6 when touched
```

F1 and F3 are independent and can run in parallel. F4 blocks F5 only because
both touch manifest semantics and F4 is the smaller, better-defined spec.
