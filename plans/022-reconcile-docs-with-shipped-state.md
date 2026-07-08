# Plan 022: Reconcile README, architecture, and follow-up docs with shipped DevSpace state

> **Executor instructions**: Follow this plan step by step. Run every verification command and confirm the expected result before moving to the next step. If anything in the "STOP conditions" section occurs, stop and report — do not improvise. When done, update the status row for this plan in `plans/README.md`.
>
> **Drift check (run first)**: `git diff --stat cedcbc7..HEAD -- README.md ARCHITECTURE.md FOLLOWUP.md docs/operations/release-readiness.md`
> If any in-scope file changed since this plan was written, compare the "Current state" excerpts against live docs before proceeding; on mismatch, treat it as a STOP condition.

## Status

- **Priority**: P2
- **Effort**: S
- **Risk**: LOW
- **Depends on**: none
- **Category**: docs
- **Planned at**: commit `cedcbc7`, 2026-07-08

## Why this matters

Several root docs still describe pre-reconcile or pre-TUI-hardening gaps as current limitations even though the code and `plans/README.md` say those plans are done. This wastes future audit time and can mislead executors into planning duplicate work. A small docs reconciliation keeps the user-facing README, architecture guide, and follow-up tracker aligned with the current CLI surface.

## Current state

Evidence to reconcile:

- `README.md` under-reports CI and still describes some shipped work as roadmap:

```md
# README.md:87-93
CI (`go test`, `go vet`, build) runs on every PR and push to `main`. The same gate is available locally:

make verify
```

```md
# README.md:416-423
## Roadmap

- Per-project conflict choices in `workspace reconcile` (global resolution and `--force-local` / `--force-remote` shipped).
- Hosted sync: grow the shipped prototype into a managed service.
- Daemon process management for running watch mode outside a terminal.
- FUSE lazy mount: grow the shipped prototype into a supported feature (macOS local proof pending).
- Managed team identity provider & OS keychain integration.
- Release-readiness checklist automation.
```

- `ARCHITECTURE.md` says per-project reconcile force is deferred/global-only, but current commands expose repeatable `--force-project` for both Git and hosted reconcile:

```md
# ARCHITECTURE.md:173-175
Real conflicts block `--apply` unless the caller passes exactly one of the
mutually exclusive, global-only `--force-local` / `--force-remote` flags
(per-project force selection is deferred, see Current Gaps).
```

```md
# ARCHITECTURE.md:248-251
## Current Gaps

- Force resolution (`--force-local`/`--force-remote`) is global-only; there is
  no per-project conflict selection yet.
```

- `ARCHITECTURE.md` sync loop still says `future reconcile`:

```text
# ARCHITECTURE.md:221-229
local Manifest
  -> manifestForSync
  -> Git remote or hosted envelope
  -> localized remote Manifest
  -> diff / pull / future reconcile
  -> local Manifest
```

- `FOLLOWUP.md` says Plan 015 still needs GitHub Actions FUSE probe evidence, while `plans/README.md` marks Plan 015 DONE and `.github/workflows/ci.yml` contains the `mount-integration` job:

```md
# FOLLOWUP.md:7
F1 ... Done — real mount smoke test passed 2026-07-06 ... Spec 02 Plan 015 stays open separately — it needs GitHub Actions FUSE probe evidence, not a local run.
```

- `plans/README.md` is the current execution source and says Plans 001-019 are DONE. Match that status rather than reopening those plans.

Repo conventions:

- Root docs use concise Markdown and command examples.
- Generated or bulky HTML under `docs/capstone/index.html` should not be hand-edited; regenerate it only if the repo already has a documented generator and the operator asks.

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| Stale phrase check | `rg -n "future reconcile|per-project force selection is deferred|global-only|macOS local proof pending|Plan 015 stays open" README.md ARCHITECTURE.md FOLLOWUP.md docs/operations/release-readiness.md` | no matches for stale claims, unless the text is quoted as historical context |
| Docs grep sanity | `rg -n "--force-project|mount-integration|make verify|make tui-verify" README.md ARCHITECTURE.md FOLLOWUP.md docs/operations/release-readiness.md` | shows updated current-state wording |
| Optional full tests | `go test ./...` | exit 0 if run; docs-only change does not require it |

## Scope

**In scope**:

- `README.md`
- `ARCHITECTURE.md`
- `FOLLOWUP.md`
- `docs/operations/release-readiness.md`

**Out of scope**:

- Source code.
- `docs/capstone/index.html` and other generated HTML unless a documented regeneration command exists and the operator asks.
- Rewriting the whole roadmap; only remove or update stale claims.
- Changing product posture: hosted sync can remain a prototype, access roles remain advisory.

## Git workflow

- Branch: `advisor/022-docs-state-reconcile`
- Commit message style: conventional commit, e.g. `docs: reconcile roadmap with shipped state`.
- Do not push or open a PR unless instructed.

## Steps

### Step 1: Update README current-state claims

In `README.md`:

- Replace the CI sentence with the current gate shape: CI runs `make verify` for Go, `make tui-verify` for the TUI, and the FUSE mount integration job. Mention that `make verify` includes tests, vet, lint, vulncheck, and build.
- Remove or rewrite the roadmap item that says per-project conflict choices are still future. Current CLI supports repeatable `--force-project <projectID>=<local|remote>` for both `workspace reconcile` and `hosted reconcile`.
- Rewrite the FUSE roadmap note so it does not say macOS local proof is pending. If keeping a FUSE roadmap item, make it about turning the prototype into a supported feature, not proving the local smoke.

**Verify**: `rg -n "per-project conflict choices|macOS local proof pending|go test`, `go vet`, build" README.md` → no stale matches.

### Step 2: Update ARCHITECTURE reconcile and gap sections

In `ARCHITECTURE.md`:

- Replace "global-only"/"per-project force selection is deferred" wording with current behavior: global `--force-local` / `--force-remote` still exist, and repeatable `--force-project <projectID>=<local|remote>` resolves selected project conflicts.
- Change the sync loop from `diff / pull / future reconcile` to `diff / pull / reconcile`.
- Remove the Current Gaps bullet claiming no per-project conflict selection.
- Keep the real remaining gap: Users/Teams reconciliation is still record-level, not field-level.

**Verify**: `rg -n "future reconcile|per-project force selection is deferred|global-only|no per-project conflict" ARCHITECTURE.md` → no stale matches.

### Step 3: Update FOLLOWUP and release-readiness stale rows

In `FOLLOWUP.md`:

- Change the F1 row to say both the macOS proof and GitHub Actions mount-integration proof are done if the current files support that statement. Cite `.github/workflows/ci.yml` mount-integration at a high level; do not paste workflow logs.
- Leave F3 manual capstone work alone unless you have current evidence it changed.

In `docs/operations/release-readiness.md`:

- Replace any "manifest sync has no force flag or merge UI" wording with current reconcile behavior, or remove it if no longer useful.
- Preserve limitations that are still true: hosted sync managed-service gap, full clone hydration, managed identity/keychain gap.

**Verify**: `rg -n "Plan 015 stays open|manifest sync has no force flag|macOS local proof pending" FOLLOWUP.md docs/operations/release-readiness.md` → no matches.

### Step 4: Run docs sanity greps

Run both commands from "Commands you will need".

**Verify**:

- The stale phrase grep has no matches.
- The current-state grep shows at least one current mention of `--force-project`, `make tui-verify`, and/or `mount-integration` where appropriate.

## Test plan

Docs-only change. Use grep checks as the required verification. If the executor touches command examples or code blocks in a way that could affect CLI usage, run `go test ./...`; otherwise do not spend time on a full source test for text-only edits.

## Done criteria

- [ ] README no longer claims per-project reconcile choices or macOS FUSE proof are future work.
- [ ] README CI wording matches `Makefile` and `.github/workflows/ci.yml`.
- [ ] ARCHITECTURE no longer says per-project force is deferred/global-only.
- [ ] FOLLOWUP no longer says Plan 015 is open for missing CI FUSE evidence.
- [ ] `docs/operations/release-readiness.md` no longer claims manifest sync has no force/reconcile path.
- [ ] Stale phrase grep returns no matches.
- [ ] No source files are modified.
- [ ] No generated HTML is hand-edited.
- [ ] `plans/README.md` row 022 is updated when complete.

## STOP conditions

Stop and report back if:

- Live code no longer supports `--force-project` despite the current command excerpts.
- The FUSE CI proof cannot be substantiated from current repo files.
- The only way to keep capstone docs consistent is hand-editing generated HTML.
- The docs have moved enough that this plan's line-level excerpts no longer match.

## Maintenance notes

This repo moves quickly through SDD waves. When a plan row flips to DONE, do a small docs grep for the same feature terms (`future`, `pending`, `deferred`, `prototype`) before closing the wave; stale root docs are now a recurring source of duplicate planning.
