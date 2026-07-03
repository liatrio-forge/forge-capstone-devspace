# 02-tasks-hardening-plan-execution.md

Task list for `02-spec-hardening-plan-execution.md`.

## Relevant Files

| File | Why It Is Relevant |
| --- | --- |
| `plans/README.md` | Existing hardening plan index; statuses must be reconciled against SDD task progress and branch state. |
| `plans/001-validate-manifest-project-ids.md` | Source plan for unsafe manifest project ID validation. |
| `plans/002-atomic-secret-and-identity-writes.md` | Source plan for atomic writes of secret profiles, `.env`, and age identity. |
| `plans/003-safety-net-tests.md` | Source plan for symlink containment and recipient listing tests. |
| `plans/004-validate-git-remotes.md` | Source plan for rejecting unsafe manifest-supplied Git remotes. |
| `plans/005-hosted-client-hardening.md` | Source plan for hosted endpoint revalidation and env-var token configuration. |
| `plans/006-fix-mergeproject-preservation.md` | Source plan for preserving user overrides across rescans. |
| `plans/007-bound-workspace-mutex-map.md` | Source plan for bounded hosted server workspace locking. |
| `plans/008-ci-lint-and-vuln-gates.md` | Source plan for lint, format, and vulnerability gates. |
| `plans/009-cross-process-locking.md` | Source plan for process-wide app-home locking. |
| `plans/010-scan-nested-project-descent.md` | Source plan for monorepo scan descent behavior. |
| `plans/011-watch-incremental-refresh.md` | Source plan for scoped watch refresh behavior. |
| `plans/012-project-remove-command.md` | Source plan for project untracking and manifest cascade behavior. |
| `plans/013-spike-manifest-conflict-reconciliation.md` | Source spike for manifest merge design and prototype. |
| `plans/014-spike-access-role-enforcement.md` | Source spike for role posture and access-model decision. |
| `plans/015-spike-fuse-ci-and-status.md` | Source spike for FUSE CI feasibility and mount backlog. |
| `internal/devspace/manifest.go` | Manifest validation, `mergeProject`, referential integrity, and project IDs. |
| `internal/devspace/secrets.go` | Secret profile writes, `.env` writes, recipient export/list/invite/revoke, and secret paths. |
| `internal/devspace/init.go` | Age identity creation and app-home initialization behavior. |
| `internal/devspace/paths.go` | App-home resolution, workspace metadata paths, and symlink-aware safe workspace paths. |
| `internal/devspace/workspace.go` | Scan, add, hydrate, plan/apply, project lookup, and project lifecycle behavior. |
| `internal/devspace/watch.go` | Watch refresh loop and hosted/git sync refresh behavior. |
| `internal/devspace/commands.go` | Cobra command wiring and mutating CLI entry points. |
| `internal/devspace/hosted_sync.go` | Hosted sync client/server behavior, endpoint validation, rate limiting, and workspace locking. |
| `internal/devspace/workspace_sync.go` | Git-backed manifest sync, conflict detection, and future merge behavior. |
| `internal/devspace/git.go` | `git clone --` wrapper and remote handling boundary. |
| `internal/devspace/types.go` | Manifest schema, roles, access records, hydrate modes, and ignored directories. |
| `internal/devspace/*_test.go` | Existing and new Go tests for hardening, hosted sync, workspace sync, watch, mount, and CLI behavior. |
| `Makefile` | Local verification, lint, vulnerability, build, and clean targets. |
| `.github/workflows/ci.yml` | CI verification workflow and future lint/vulnerability/mount jobs. |
| `.github/dependabot.yml` | Dependency update automation if Plan 008 or branch reconciliation keeps it. |
| `.golangci.yml` | golangci-lint configuration if Plan 008 or branch reconciliation keeps it. |
| `go.mod` / `go.sum` | Go version and dependency changes, including any locking or vulnerability tooling dependencies. |
| `docs/manifest-merge.md` | Plan 013 design deliverable. |
| `docs/access-roles.md` | Plan 014 design deliverable. |
| `docs/fuse-lazy-mount.md` | Plan 015 feasibility and mount backlog deliverable. |
| `docs/specs/02-spec-hardening-plan-execution/02-proofs/` | Proof artifact directory for sanitized implementation evidence. |
| `docs/specs/02-spec-hardening-plan-execution/02-validation-hardening-plan-execution.md` | Final SDD validation report created in validation phase. |

### Notes

- Use `t.Setenv("DEVSPACE_HOME", t.TempDir())` in tests that touch app-home state.
- Keep proof artifacts sanitized: no real `.env` values, hosted tokens, age identities, `.devspace/`, or `.devdrop/` state.
- Run each source plan's drift check before implementing it; the plans were generated against `595d158`, while current `main` and `chore/hardening-pass` have moved.
- Treat `chore/hardening-pass` as evidence to reconcile, not as automatically correct work to merge wholesale.
- Preserve `plans/README.md` as the human-readable plan index during this workflow; SDD tasks are the execution blueprint. Keep both status surfaces consistent.

## Standards Evidence

| Source File | Read | Standards Extracted | Conflicts |
| --- | --- | --- | --- |
| `AGENTS.md` | yes | Operational silence; Go CLI structure; `make verify`; gofmt on changed Go files; Conventional Commit subjects; do not commit secrets or generated workspace state. | SDD requires a structured handoff; keep it minimal. |
| `CLAUDE.md` | yes | Single Go package under `internal/devspace`; Cobra command wiring in `commands.go`; preserve path safety, non-destructive plan/apply, idempotent init, and DevDrop-to-DevSpace migration compatibility. | none |
| `README.md` | yes | Local-first CLI behavior; hosted sync only sends normalized manifest metadata; env values are age-encrypted; `make verify` is the local CI gate; roadmap includes manifest conflict resolution and FUSE follow-up. | none |
| `Makefile` | yes | `verify` currently runs test, vet, build; `build` emits `bin/devspace`; `clean` removes `bin/` and `dist/`. | none |
| `.github/workflows/ci.yml` | yes | CI runs on PRs and pushes to `main`; permissions are `contents: read`; steps are checkout, setup-go from `go.mod`, test, vet, build. | none |
| `go.mod` | yes | Module is `github.com/HexSleeves/devspace`; Go version is `1.26`; toolchain is `go1.26.4`. | none |
| `CONTRIBUTING.md` | not found | none | none |
| `.github/pull_request_template.md` | not found | none | none |
| `.golangci.yml` | not found on `main` | none on current `main`; exists only on `chore/hardening-pass` branch and must be reconciled before Plan 008 work. | none |

## Planning Assumptions

- The first implementation task will decide, from evidence, whether to cherry-pick, rework, reject, or defer each existing `chore/hardening-pass` commit.
- `plans/README.md` remains the plan-bundle index; this SDD task file is the execution checklist. Implementation must update both when status changes.
- The open questions in the spec are operational choices for Task 1.0, not blockers for planning.

## Requirement Traceability

| Spec requirement | Task(s) | Planned test/proof artifact |
| --- | --- | --- |
| Identify SDD spec, tasks, audit, proof, and validation artifacts. | 1.1, 1.8, 5.6 | `assess-sdd-state.py` output in Task 1 proof; final validation report in Task 5. |
| Compare `plans/README.md` against `main` and `chore/hardening-pass`. | 1.1, 1.2, 1.3 | `git log` and `git diff --name-status main..chore/hardening-pass` proof. |
| Classify each plan status before implementation. | 1.4, 1.5, 1.6 | Updated status table or reconciliation notes in Task 1 proof. |
| Make executable plans inspectable without duplicate work. | 1.6, 3.1, 3.2 | Branch comparison summary and status updates. |
| Execute P1 hardening before lower-priority work unless blocked. | 2.1, 2.2, 2.3, 2.4 | Targeted P1 test command and `make verify`. |
| Preserve source plan STOP conditions and scope boundaries. | 1.7, 2.1, 3.1, 4.1 | Diff summaries mapped to source plans. |
| Update status surfaces after completed slices. | 1.6, 2.6, 3.7, 4.8, 5.5 | `plans/README.md` or SDD task status proof. |
| Provide targeted and full verification for nontrivial slices. | 2.5, 3.6, 4.7, 5.6 | Targeted Go tests, lint/vuln outputs, smoke output, `make verify`. |
| Reconcile Plan 008 before tooling changes. | 3.1, 3.2, 3.6 | Lint/vuln/make proof and branch comparison summary. |
| Reconcile hosted-client/server plans against branch commits. | 3.1, 3.3, 3.5 | `Hosted|Remote|Sync|Output|Hardening` targeted test output. |
| Reconcile sync identity and output-refactor commits. | 3.1, 3.4, 3.5 | Branch decision summary plus targeted sync/output tests. |
| Explain DONE/TODO/BLOCKED/REJECTED reasons. | 1.5, 3.7, 4.8, 5.5 | Status table updates with one-line rationale. |
| Validate implementation against spec goals and demoable units. | 5.6 | `02-validation-hardening-plan-execution.md`. |
| Record incomplete, rejected, and deferred work. | 1.5, 5.5, 5.6 | Final validation and plan status updates. |
| Prevent sensitive proof artifacts. | 1.8, 2.7, 3.8, 4.9, 5.7 | Sensitive-file check output and sanitized proof review. |
| Produce final validation with pass/fail and gaps. | 5.6, 5.8 | Final validation report. |

## Tasks

### [x] 1.0 Reconcile the hardening plan bundle against live branch state

#### 1.0 Proof Artifact(s)

- Markdown: `docs/specs/02-spec-hardening-plan-execution/02-proofs/02-task-01-proofs.md` containing `git log --graph --oneline --decorate --all --max-count=35` and `git diff --name-status main..chore/hardening-pass` demonstrates the current branch topology and overlapping work were reviewed.
- Markdown: updated `plans/README.md` or SDD task status notes demonstrating each plan has a reconciled status before implementation.
- CLI: `python3 .agents/skills/sdd/scripts/assess-sdd-state.py .` output demonstrates the SDD workflow routes to this spec after task planning.

#### 1.0 Tasks

- [x] 1.1 Create `docs/specs/02-spec-hardening-plan-execution/02-proofs/` and capture sanitized branch-state commands in `02-task-01-proofs.md`.
- [x] 1.2 Run every source plan drift check, at least in summary form, and record which plans changed in-scope files since `595d158`.
- [x] 1.3 Compare `main..chore/hardening-pass` commit-by-commit and file-by-file; classify each branch commit as cherry-pick candidate, rework candidate, reject, or defer.
- [x] 1.4 Classify Plans 001-015 as TODO, already implemented on branch, drifted, blocked, rejected, or ready.
- [x] 1.5 For any plan marked DONE, BLOCKED, or REJECTED, write a one-line rationale tied to concrete code, branch, or test evidence.
- [x] 1.6 Update `plans/README.md` and this SDD task file status notes so the plan index and SDD checklist do not disagree.
- [x] 1.7 Preserve each source plan's STOP conditions in the reconciliation notes; do not weaken them during grouping.
- [x] 1.8 Run `python3 .agents/skills/sdd/scripts/assess-sdd-state.py .` and append the output to the Task 1 proof artifact.

#### 1.0 Reconciliation Notes

All source-plan drift checks against `595d158..HEAD` returned no in-scope code drift. The active overlap is isolated to `chore/hardening-pass`, which contains CI, hosted, sync, output-helper, and documentation commits that must be reconciled before Task 3 work.

| Plan | Reconciled Status | Branch/Drift Rationale |
| --- | --- | --- |
| 001 | DONE | Implemented in Task 2 with unsafe-ID validation and secret-path defense-in-depth tests. |
| 002 | DONE | Implemented in Task 2 with atomic secret, `.env`, and age identity writes. |
| 003 | DONE | Implemented in Task 2 with symlink containment and recipient listing/export tests. |
| 004 | READY | No in-scope drift; no branch implementation found. Execute in Task 3. |
| 005 | DRIFTED | Branch commits `70509e8` and `ae5a61c` touch hosted client/server safety surfaces; rework against current hosted contracts in Task 3. |
| 006 | DONE | Implemented in Task 2 with mergeProject preservation and scan regression tests. |
| 007 | DRIFTED | Branch commit `70509e8` changes hosted rate-limit behavior near the workspace lock map; reconcile before implementing bounded locks. |
| 008 | ALREADY IMPLEMENTED ON BRANCH | Branch commit `c890b39` adds golangci-lint, govulncheck, Dependabot, and Makefile gates. Cherry-pick or rework in Task 3. |
| 009 | READY | No in-scope drift; branch command/output commits do not implement app-home locking. Execute in Task 4. |
| 010 | READY | No in-scope drift; depends on Plan 006. Execute in Task 4 after Plan 006 lands. |
| 011 | BLOCKED | Blocked by explicit source-plan dependencies on Plans 009 and 010. |
| 012 | READY | No in-scope drift; soft-depends on Plan 009. Execute in Task 4. |
| 013 | DRIFTED | Branch commits `e4f7bd0` and hosted changes touch sync inputs; design spike must reread current sync refusal paths first. |
| 014 | DRIFTED | Branch commits touch `types.go` and hosted sync surfaces; role-posture spike must account for configurable identity and hosted auth state. |
| 015 | READY | No mount/doc drift; CI feasibility still requires observed workflow evidence during Task 5. |

Branch commit decisions:

| Commit | Decision | Rationale |
| --- | --- | --- |
| `595d158` | reject | Ancestor ignore-rule commit already present on `main` as `50df691`. |
| `8d086ff` | reject | Repo-identity cleanup overlaps with already-landed rename/capstone cleanup and deletes local Lavish artifacts not part of hardening execution. |
| `c890b39` | cherry-pick candidate | Direct Plan 008 implementation candidate. |
| `70509e8` | rework candidate | Hosted rate-limit/XFF change overlaps hosted hardening but must be reconciled with current public-exposure contracts. |
| `ae5a61c` | rework candidate | Public bind guard overlaps hosted safety but current `main` already includes PR #16 hardening; re-evaluate before preserving. |
| `e4f7bd0` | defer | Configurable sync commit identity is related but outside P1/P2 hardening unless Task 3 keeps it. |
| `71919e8` | defer | Presentation-helper refactor may aid output cleanup but is not required by a source plan. |
| `9b4da48` | defer | Command-output tests are useful only if Task 3 keeps the output-helper refactor. |
| `973b6f1` | defer | SHA-1 documentation is directionally useful but not required for source-plan execution. |

Source-plan STOP conditions remain in force. Task grouping does not relax file scope, dependency order, no-secret requirements, or spike limits; each later parent task must reread the relevant source plans before editing implementation files.

### [x] 2.0 Implement the priority local safety and correctness slice

#### 2.0 Proof Artifact(s)

- Test: `go test ./internal/devspace -run 'ValidateManifest|EncryptedEnv|Init|Symlink|Recipient|MergeProject|Scan|Hardening' -v` output demonstrates the priority safety and correctness behavior.
- CLI: `make verify` output demonstrates the repo-level gate passes after the P1 slice.
- Git: diff summary in `docs/specs/02-spec-hardening-plan-execution/02-proofs/02-task-02-proofs.md` demonstrates changes map to Plans 001, 002, 003, and 006 and stay within intended files.

#### 2.0 Tasks

- [x] 2.1 Re-read Plans 001, 002, 003, and 006 fully and run their drift checks before editing any source file.
- [x] 2.2 Implement Plan 001: reject unsafe manifest project IDs and add tests for invalid ID segments and secret-path defense-in-depth.
- [x] 2.3 Implement Plan 002: route secret profile, `.env`, and age identity writes through `atomicWriteFile` with `backup=false`, adapting to any Plan 001 signature changes.
- [x] 2.4 Implement Plan 003: add test-only coverage for symlink escape containment, recipient listing, revoked recipient state, and local recipient export.
- [x] 2.5 Implement Plan 006: update `mergeProject` to preserve user-set ignore lists and hydrate mode according to the source plan's semantics table, with regression tests.
- [x] 2.6 Run the targeted P1 test command and `make verify`; save sanitized output plus `git diff --stat` to `02-task-02-proofs.md`.
- [x] 2.7 Update `plans/README.md` and this task file to reflect completed or blocked P1 plans, without committing real secrets or generated workspace state.

### [ ] 3.0 Reconcile and land CI, hosted, and sync hardening work without duplication

#### 3.0 Proof Artifact(s)

- Git: branch comparison summary showing which `chore/hardening-pass` commits were cherry-picked, reworked, rejected, or deferred demonstrates duplicate work was avoided.
- CI: `make lint`, `make vulncheck`, and `make verify` outputs, or documented network/tooling exceptions, demonstrate lint, vulnerability, test, vet, and build gates are wired when Plan 008 is in scope.
- Test: `go test ./internal/devspace -run 'Hosted|Remote|Sync|Output|Hardening' -v` output demonstrates hosted, remote, sync, and CLI presentation changes keep existing contracts.

#### 3.0 Tasks

- [ ] 3.1 Re-read Plans 004, 005, 007, 008, and any branch commits touching hosted/sync/output/CI before choosing cherry-pick or rework.
- [ ] 3.2 Reconcile Plan 008 against `c890b39` and current CI files; either preserve the branch implementation, rework it, or mark it rejected/deferred with evidence.
- [ ] 3.3 Implement or reconcile Plan 004 and Plan 005: validate unsafe Git remote forms, re-check hosted endpoint safety at point of use, and support hosted token configuration from environment where planned.
- [ ] 3.4 Implement or reconcile Plan 007 and hosted branch hardening: replace unbounded workspace mutex growth with bounded locking while preserving existing hosted hardening tests.
- [ ] 3.5 Reconcile existing sync identity and output-helper branch commits; keep only changes that support this spec or a source plan and document rejected extras.
- [ ] 3.6 Run `go test ./internal/devspace -run 'Hosted|Remote|Sync|Output|Hardening' -v`, `make lint`, `make vulncheck`, and `make verify`; document any unavailable network/tooling exception.
- [ ] 3.7 Update `plans/README.md`, branch-reconciliation notes, and SDD task statuses with DONE/TODO/BLOCKED/REJECTED reasons.
- [ ] 3.8 Save sanitized branch comparison, test output, CI/lint/vulnerability output, and diff summary to `02-task-03-proofs.md`.

### [ ] 4.0 Implement concurrency, scan, watch, and project lifecycle hardening

#### 4.0 Proof Artifact(s)

- Test: `go test ./internal/devspace -run 'Lock|Scan|Watch|Project|Remove|Hardening' -v` output demonstrates locking, scan descent, watch refresh, and project removal behavior.
- CLI: sanitized smoke output from temp `DEVSPACE_HOME` runs for `devspace scan`, `devspace watch --once`, and `devspace project remove` demonstrates end-to-end CLI behavior without real workspace state.
- CLI: `make verify` output demonstrates the full repo remains green after the concurrency/lifecycle slice.

#### 4.0 Tasks

- [ ] 4.1 Re-read Plans 009, 010, 011, and 012 fully and run their drift checks before editing source files.
- [ ] 4.2 Implement Plan 009: add cross-process app-home locking around mutating CLI entry points, using the approved dependency and timeout/error behavior.
- [ ] 4.3 Implement Plan 010 after Plan 006 is complete: stop nested non-git dependency markers from producing duplicate monorepo projects while preserving nested git repo behavior.
- [ ] 4.4 Implement Plan 011 only after Plans 009 and 010: add scoped watch refresh with a full-scan safety valve and tests for event scoping.
- [ ] 4.5 Implement Plan 012: add `devspace project remove`, cascade manifest access/state references, never delete project folders or secret files, and print retained secret location when relevant.
- [ ] 4.6 Build `bin/devspace` and run sanitized temp-home smoke flows covering `init`, `scan`, `watch --once`, `project add`, and `project remove`.
- [ ] 4.7 Run the targeted lifecycle test command and `make verify`; save sanitized outputs and `git diff --stat` to `02-task-04-proofs.md`.
- [ ] 4.8 Update `plans/README.md` and this task file with final status for Plans 009-012.
- [ ] 4.9 Check proof artifacts and diffs for accidental real `.env`, hosted token, age identity, `.devspace/`, or `.devdrop/` state before committing.

### [ ] 5.0 Complete direction spikes and final SDD validation handoff

#### 5.0 Proof Artifact(s)

- Markdown: `docs/manifest-merge.md`, `docs/access-roles.md`, and updated `docs/fuse-lazy-mount.md` sections demonstrate the spike decisions and evidence.
- Markdown: `docs/specs/02-spec-hardening-plan-execution/02-validation-hardening-plan-execution.md` demonstrates each spec demoable unit passed or lists remaining gaps.
- Git: final `git status --short` and sensitive-file check output demonstrates proof artifacts are committed intentionally and contain no real `.env`, hosted token, age identity, `.devspace/`, or `.devdrop/` state.

#### 5.0 Tasks

- [ ] 5.1 Re-read Plans 013, 014, and 015 fully and run their drift checks before creating or updating design documents.
- [ ] 5.2 Complete Plan 013 as a spike: create `docs/manifest-merge.md` and any unwired prototype/tests allowed by the plan, with explicit open questions and recommendations.
- [ ] 5.3 Complete Plan 014 as a spike: create `docs/access-roles.md` and only optional warning-only prototype code if the source plan still supports it after drift review.
- [ ] 5.4 Complete Plan 015 as a spike: run the FUSE feasibility probe, document GO/NO-GO evidence in `docs/fuse-lazy-mount.md`, and only add real CI/tests if the probe supports it.
- [ ] 5.5 Reconcile final statuses for all Plans 001-015, including incomplete, rejected, blocked, and deferred items with one-line rationale.
- [ ] 5.6 Run final `make verify`, collect final `git status --short`, and create `02-validation-hardening-plan-execution.md` mapping evidence to each demoable unit.
- [ ] 5.7 Run a sensitive-content review over source and proof artifacts for real `.env` values, hosted tokens, age private keys, and generated `.devspace/` or `.devdrop/` state.
- [ ] 5.8 Save final validation and handoff evidence to `02-task-05-proofs.md`, including remaining gaps and the recommended next SDD action.
