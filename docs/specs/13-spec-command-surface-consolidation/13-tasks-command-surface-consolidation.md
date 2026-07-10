# 13 Tasks - Command Surface Consolidation

## Standards Evidence Table

| Source File | Read | Standards Extracted | Conflicts |
| --- | --- | --- | --- |
| `AGENTS.md` | yes | Use standard Go formatting; keep tests beside implementation; run `make verify`; preserve local-first and security-sensitive command boundaries | none |
| `README.md` | yes | Manifest sync is metadata-only; JSON output is stable and ANSI-free; destructive or implicit source/setup/secret behavior is prohibited | none |
| `Makefile` | yes | `make verify` is the Go gate; `make tui-verify` is separate; release snapshots use GoReleaser; FUSE remains optional | none |
| `.github/workflows/ci.yml` | yes | CI runs Go verification, TUI verification, and bounded tagged FUSE integration independently | none |
| `.github/workflows/release-check.yml` | yes | Release configuration changes require a snapshot GoReleaser dry-run | none |
| `.golangci.yml` | yes | Standard linters plus `gosec`; formatting and operational error handling remain enforced | none |
| `go.mod` | yes | Go 1.26.5; Cobra v1.10.2 and Fang v2 are the existing CLI stack; no replacement framework is needed | none |
| `CONTRIBUTING.md` | not found | — | none |
| `.github/pull_request_template.md` | not found | — | none |

## Relevant Files

| File | Why It Is Relevant |
| --- | --- |
| `internal/devspace/commands.go` | Registers the root command tree and implements sync, hosted, project, env, status, setup, and experimental command wiring. |
| `internal/devspace/commands_test.go` | Primary command execution, help, argument-validation, output, and JSON contract tests. |
| `internal/devspace/output.go` | Renders project lists, workspace/project status, plans, setup results, and next-step guidance. |
| `internal/devspace/workspace_overview.go` | Builds the saved workspace overview that moves behind `status --verbose`. |
| `internal/devspace/workspace_overview_test.go` | Existing overview rendering/redaction/JSON tests to migrate to the status surface. |
| `internal/devspace/workspace_sync.go` | Git-backed manifest sync behavior and user-facing recovery guidance that must reference `sync`. |
| `internal/devspace/workspace_sync_test.go` | Manifest remote, push, pull, diff, safety, and two-machine regression coverage. |
| `internal/devspace/reconcile.go` | Reconcile drift errors currently point to the old workspace namespace. |
| `internal/devspace/reconcile_test.go` | Reconcile command JSON, force-flag, conflict, and apply-guard coverage. |
| `internal/devspace/hosted_sync.go` | Hosted client guidance currently references old Git-backed paths. |
| `internal/devspace/access_roles_test.go` | Asserts command labels used by sync, untrack, and env access advisories. |
| `internal/devspace/doctor.go` | Diagnostic remediation text currently points to workspace commands. |
| `internal/devspace/devspace_test.go` | Env write, root help, project safety, and general CLI regression coverage. |
| `internal/devspace/setup.go` | Existing single/all-project setup domain functions reused by `setup show` and `setup run`. |
| `internal/devspace/setup_test.go` | Setup command eligibility, execution, unknown/global command, and confirmation coverage. |
| `internal/devspace/interactive_test.go` | Piped confirmation and progress behavior affected by renamed setup/project commands. |
| `internal/devspace/mount.go` | Experimental mount implementation and user-facing update/preview guidance. |
| `internal/devspace/mount_test.go` | FUSE-free mount preview and mount safety regression coverage. |
| `internal/devspace/ui.go` | Defines the single visible UI command, companion lookup order, and legacy fallback message. |
| `internal/devspace/ui_server.go` | Hidden companion backend that must remain available after removing the installer command. |
| `internal/devspace/ui_server_test.go` | Companion discovery and advisory command-label coverage. |
| `internal/devspace/ui_actions.go` | TUI untrack action advisory label must match the canonical CLI vocabulary. |
| `internal/devspace/tui_install.go` | Obsolete release installer command and GitHub download helpers to remove. |
| `internal/devspace/tui_install_test.go` | Installer-only tests to delete or replace with bundled-companion tests. |
| `tui/build-all.sh` | Produces the four platform companion executables consumed by release packaging. |
| `.goreleaser.yaml` | Must place the matching companion executable inside each CLI archive. |
| `.github/workflows/release.yml` | Builds companion targets before the production GoReleaser run. |
| `.github/workflows/release-check.yml` | Must build companion targets and validate the real archive layout on release-related changes. |
| `Makefile` | Owns Go/TUI/release gates and any reusable command-surface documentation check. |
| `README.md` | Primary task-oriented command reference and pre-1.0 migration table. |
| `ARCHITECTURE.md` | Maintained command-layout, data-flow, and supported/experimental boundary documentation. |
| `docs/operations/release.md` | Consumer archive layout, extraction, verification, and release procedure. |
| `docs/operations/release-readiness.md` | Release evidence and supported/prototype command declarations. |
| `docs/operations/macos-fuse-run-playbook.md` | Maintained mount smoke commands that must use the experimental namespace. |
| `docs/architecture/*.md` | Maintained access-role, FUSE, and manifest-merge command references. |
| `docs/capstone/{README.md,spec.md,demo-script.md,proof-artifacts.md}` | Maintained capstone contract and proof instructions that must use canonical commands. |
| `docs/demos/{README.md,*.sh,*.tape}` | Active demo sources whose command paths must migrate without rewriting generated GIF history. |
| `scripts/demo-check.sh` | Executable two-machine release workflow that must prove the canonical surface. |
| `scripts/check-command-surface.sh` | New scoped check for removed command paths in maintained docs/scripts, excluding historical evidence. |
| `plans/README.md` | Existing plan-status authority; reconcile overlapping release-check/docs plans when this feature lands. |
| `docs/specs/13-spec-command-surface-consolidation/13-proofs/` | New sanitized, reproducible proof artifacts for each parent task. |

### Notes

- Write command tests first and preserve existing domain functions; this feature rewires user intent without duplicating sync, project, env, setup, mount, hosted, or UI behavior.
- Use isolated temporary `DEVSPACE_HOME` and workspace paths for every CLI workflow proof.
- Preserve unrelated worktree changes, including active demo-source edits; only update a maintained demo file after reading its live contents.
- Completed historical SDD specs/proofs remain unchanged and are excluded from command migration scans.
- Run targeted tests after each parent task, then `make verify`, `make tui-verify`, the demo check, and release validation in Task 5.

## Requirement Traceability

| Specification Requirements | Planned Tasks | Planned Test/Proof Coverage |
| --- | --- | --- |
| Unit 1 FR1-FR5 | 1.1-1.3 | `TestReleaseCommandTreeContract`; root help/version capture |
| Unit 1 FR6-FR7 | 1.4-1.5 | status overview/project JSON tests and CLI capture |
| Unit 1 FR8-FR10 | 1.1, 1.3, 1.6 | help example/group tests; removed-path rejection table |
| Unit 2 FR1-FR4 | 2.1-2.4 | sync command tests plus isolated two-machine workflow |
| Unit 2 FR5-FR9 | 2.5-2.8 | project list/track/untrack/update tests and CLI proof |
| Unit 2 FR10 | 1.5, 2.2, 2.8 | JSON cleanliness/schema assertions for status, sync, and project output |
| Unit 3 FR1-FR3 | 3.1-3.2 | env write round-trip, permission, redaction, and removed-path tests |
| Unit 3 FR4-FR6 | 3.3-3.4 | setup show/run/all, conflict validation, confirmation, and dry-run tests |
| Unit 3 FR7-FR8 | 3.5-3.7 | hosted/experimental help contracts, public-bind guard, and mount preview tests |
| Unit 3 FR9-FR11 | 4.1-4.6 | UI command tree, companion lookup/fallback, archive listing, and launch smoke |
| Unit 4 FR1-FR6 | 5.1-5.5 | maintained-doc migration scan and canonical demo workflow |
| Unit 4 FR7 | 5.6-5.8 | `make verify`, `make tui-verify`, demo, GoReleaser, and archive evidence |

## Tasks

### [x] 1.0 Establish the release command taxonomy, grouped help, and consolidated status surface

#### 1.0 Proof Artifact(s)

- CLI: `go run ./cmd/devspace --help` shows grouped core, management, diagnostics/automation, and experimental commands; it exposes no more than 14 product commands and omits `workspace`, `tui`, `mount`, and `version`.
- CLI: isolated `devspace status`, `devspace status --verbose`, and `devspace status <project> --json` output demonstrates workspace health, saved overview, and project-specific status through one canonical command.
- Test: `go test ./internal/devspace -run 'TestReleaseCommandTreeContract|TestStatusCommand' -count=1` passes and demonstrates canonical path resolution, removed-path rejection, help examples, JSON cleanliness, and status behavior.
- Evidence file: `docs/specs/13-spec-command-surface-consolidation/13-proofs/13-task-01-proofs.md` records sanitized help and status output.

#### 1.0 Tasks

- [x] 1.1 Add failing command-tree contract tests that define the 14-command maximum, help groups, canonical root names, `--version`, required examples, and rejection of removed root/group paths.
- [x] 1.2 Add failing status tests for workspace health, `--verbose` overview/redaction, `<project>` selection, project JSON output, ANSI-free JSON, and invalid argument/flag combinations.
- [x] 1.3 Rebuild `NewRootCommand` with Cobra groups and only the canonical root commands; remove `version`, `workspace`, `tui`, and root `mount` registration without aliases or deprecated wrappers.
- [x] 1.4 Extend the status command to reuse `buildWorkspaceOverview`, `printWorkspaceOverview`, and `ProjectListRow` for verbose and project-specific output without changing existing workspace status JSON fields.
- [x] 1.5 Make resource groups show focused help when no action is provided, and add concise `Long`/`Example` content for the root, status, and newly canonical commands.
- [x] 1.6 Update output guidance and tests that direct users to implicit project listing or removed root paths; keep JSON output byte-clean.
- [x] 1.7 Run the Task 1 targeted tests, capture sanitized root/status help and JSON output, and write `13-proofs/13-task-01-proofs.md`.

### [x] 2.0 Replace workspace/project overlap with canonical sync and project workflows

#### 2.0 Proof Artifact(s)

- CLI: an isolated two-machine workflow using `sync remote create local`, `sync push`, `sync pull`, `plan`, `apply`, and `project update --all` recreates and hydrates workspace structure without using a removed command.
- CLI: `devspace project list`, `track`, `untrack`, and `update` demonstrate explicit manifest membership and repository-update behavior; untrack output confirms files remain untouched.
- Test: `go test ./internal/devspace -run 'TestSyncCommand|TestProject(Command|Update|Untrack|Track)' -count=1` passes and demonstrates validation, backups, divergence protection, reconciliation, dirty-repo skips, non-destructive untracking, and stable JSON output.
- Evidence file: `docs/specs/13-spec-command-surface-consolidation/13-proofs/13-task-02-proofs.md` records the reproducible sanitized workflow.

#### 2.0 Tasks

- [x] 2.1 Add failing `sync` command tests for push, pull, diff, reconcile, remote set/get/create, inherited flags, JSON output, and absence of all `workspace ...` paths including `workspace scan`.
- [x] 2.2 Extract reusable command handlers only where needed, then wire `newSyncCommand` to the existing Git-backed domain functions, `withAppLock`, output helpers, force flags, and access advisories.
- [x] 2.3 Update Git/hosted sync, reconcile, and doctor remediation messages from `workspace ...` to `sync ...`; update exact-string tests for missing remotes, divergence, and retry guidance.
- [x] 2.4 Run focused sync/reconcile/access-role tests to prove metadata-only transport, validation, localization, backups, hash guards, force behavior, and advisory labels are unchanged.
- [x] 2.5 Add failing project command tests for explicit `list`, `track`, `untrack`, `update <project>`, `update --all`, group help, JSON list output, and rejection of `add`, `remove`, `hydrate`, and `project status`.
- [x] 2.6 Rewire project commands to the existing list/add/remove/update domain behavior under canonical names; make bare `project` show help and keep untrack files/secrets messaging intact.
- [x] 2.7 Update project guidance and TUI/ui-server access-advisory labels to `project track`, `project untrack`, and `project update` without changing TUI remove-action behavior.
- [x] 2.8 Run project domain/command tests covering missing/empty hydration, clean fast-forward, dirty/detached/local-only/no-remote/non-Git skips, non-destructive untrack, and JSON stability.
- [x] 2.9 Execute the isolated two-machine sync/project workflow and write `13-proofs/13-task-02-proofs.md` with sanitized paths/remotes.

### [x] 3.0 Consolidate env/setup verbs and move server/mount prototypes under experimental

#### 3.0 Proof Artifact(s)

- CLI: isolated `devspace env write demo` creates a `0600` `.env` file while captured output contains no decrypted value.
- CLI: `devspace setup show`, `setup run demo --dry-run`, and `setup run --all --dry-run` demonstrate one review verb and one execution verb; a conflicting project-plus-`--all` invocation returns a clear error.
- CLI: `devspace hosted --help` omits `serve`, while `devspace experimental --help`, `experimental hosted serve --help`, and `experimental mount --preview` demonstrate clearly labeled prototype paths with existing flags and guards.
- Test: `go test ./internal/devspace -run 'TestEnvWrite|TestSetup(Command|Run)|TestExperimental(Command|HostedServe|Mount)' -count=1` passes and demonstrates permissions, confirmation/dry-run safeguards, public-bind protection, and FUSE-free preview behavior.
- Evidence file: `docs/specs/13-spec-command-surface-consolidation/13-proofs/13-task-03-proofs.md` records sanitized output and file-mode evidence.

#### 3.0 Tasks

- [x] 3.1 Add failing env command tests for `env write`, profile selection, `0600` output, symlink-safe replacement, state refresh, redacted output, and rejection of `env pull`.
- [x] 3.2 Wire `env write` to the existing env materialization domain path, remove `env pull`, and update maintained user-facing guidance without changing encryption or recipient behavior.
- [x] 3.3 Add failing setup tests for `setup show`, `setup run <project>`, `setup run --all`, project/`--all` mutual exclusion, JSON show output, confirmations, dry-run, unknown-command, and global-install safeguards.
- [x] 3.4 Consolidate setup command wiring around existing `BuildSetupPlan`, `RunProjectSetup`, and `RunAllProjectSetups`; remove `setup plan`/`apply` and preserve actionable error/output behavior.
- [x] 3.5 Add failing help/contract tests showing hosted client commands remain under `hosted`, `hosted serve` is absent, and `experimental mount` plus `experimental hosted serve` retain all existing flags.
- [x] 3.6 Add the visible experimental group, move existing mount/server command constructors under it, and retain the server's loopback/public-HTTP/trusted-proxy guards plus mount preview/hydrate/debug behavior.
- [x] 3.7 Update mount diagnostics, logs, architecture comments, and tests to reference `experimental mount` and `project update` while preserving FUSE integration behavior.
- [x] 3.8 Run targeted env/setup/hosted/mount tests and write `13-proofs/13-task-03-proofs.md` with masked values, file-mode evidence, help, and preview output.

### [x] 4.0 Make `devspace ui` the only release UI entry point and bundle the companion in archives

#### 4.0 Proof Artifact(s)

- CLI: `devspace --help` contains `ui` and no `tui`; `devspace ui --help` documents bundled-companion preference and the legacy fallback.
- Archive listing: each Linux/macOS amd64/arm64 snapshot archive contains matching `devspace` and `devspace-tui` executables.
- CLI smoke: extracting one local-platform snapshot archive and running `devspace ui` with a controlled TTY demonstrates that the adjacent companion is selected without `tui install`.
- Test: `go test ./internal/devspace -run 'TestFindTUIBinary|TestUICommand|TestReleaseCommandTreeContract' -count=1` and `make tui-verify` pass.
- Evidence file: `docs/specs/13-spec-command-surface-consolidation/13-proofs/13-task-04-proofs.md` records archive contents and sanitized launch evidence.

#### 4.0 Tasks

- [x] 4.1 Add failing tests that require `ui` as the only visible UI command, preserve `ui-server` as hidden, prefer an adjacent companion, and retain app-home/PATH lookup plus legacy fallback.
- [x] 4.2 Remove `newTUICommand`, installer/download/checksum code, installer-only tests, and the fallback hint that recommends `tui install`; keep maintainer-local TUI installation through the Makefile.
- [x] 4.3 Update `ui --help` and fallback diagnostics to describe release-bundled companion preference and source-build legacy behavior without changing dashboard actions or RPC protocol.
- [x] 4.4 Configure GoReleaser archives so each Go target includes the matching `tui/dist/devspace-tui_<os>_<arch>` file as executable `devspace-tui`; keep checksums and release attachments consistent.
- [x] 4.5 Update `release-check.yml` path filters and job steps to install Bun, run `make tui-build-all`, perform the GoReleaser snapshot, and assert all four archives contain both executables.
- [x] 4.6 Add or update release/archive verification scripts/tests, then run `make tui-verify`, `goreleaser check`, and a snapshot dry-run where the environment supports Docker/ko.
- [x] 4.7 Extract and smoke the local-platform archive, verify adjacent companion discovery, and write `13-proofs/13-task-04-proofs.md`; record Docker-only limitations separately from source failures.

### [~] 5.0 Migrate maintained documentation and demos, then prove release readiness

#### 5.0 Proof Artifact(s)

- Documentation check: a scoped search of maintained README, architecture, operations, capstone, script, and active demo files finds canonical commands and zero removed paths; completed historical SDD evidence is excluded.
- Demo: `scripts/demo-check.sh` passes using only `sync`, `project list|track|untrack|update`, `env write`, `setup show|run`, `status`, `ui`, and `experimental` where relevant.
- Verification: `make verify`, `make tui-verify`, and `goreleaser check` complete successfully.
- Release proof: a snapshot or CI release dry-run builds the platform archives with the companion included and records any environment-only Docker limitation separately from source failures.
- Evidence file: `docs/specs/13-spec-command-surface-consolidation/13-proofs/13-task-05-proofs.md` records migration scan, demo, verification, and release-check outcomes without credentials or private workspace data.

#### 5.0 Tasks

- [x] 5.1 Add `scripts/check-command-surface.sh` with an explicit maintained-file allowlist and historical-artifact exclusions; make it fail on removed command paths and verify canonical paths are represented.
- [x] 5.2 Rewrite the README command section around capture, restore, maintain, and troubleshoot jobs; add the pre-1.0 old-to-new migration table and archive-based UI instructions.
- [x] 5.3 Update maintained architecture, operations, and capstone documents to the canonical vocabulary while preserving metadata-only, no-implicit-setup, no-source-sync, and no-secret-upload boundaries.
- [x] 5.4 Update `scripts/demo-check.sh` and live demo `.sh`/`.tape` sources to use canonical commands; preserve unrelated in-progress demo edits and do not rewrite completed historical SDD evidence or generated GIFs.
- [x] 5.5 Add the scoped command-surface check to an appropriate local/CI verification target and update release-check path filters for TUI build/archive inputs.
- [x] 5.6 Run the command migration scan, `scripts/demo-check.sh`, targeted CLI smoke flows, `make verify`, `make tui-verify`, and `goreleaser check`.
- [x] 5.7 Run the snapshot/archive proof path locally or in CI, confirm Linux/macOS amd64/arm64 archive contents, and distinguish Docker/ko environment failures from source/package failures.
- [x] 5.8 Reconcile overlapping entries in `plans/README.md` only after their acceptance criteria are satisfied, then write `13-proofs/13-task-05-proofs.md` with sanitized scan, demo, verification, and release evidence.
