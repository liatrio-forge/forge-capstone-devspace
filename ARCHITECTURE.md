# DevSpace Architecture

DevSpace is a local-first CLI for keeping a developer workspace structurally
aligned across machines. It tracks workspace metadata in a shared
`manifest.json`, records machine-local state separately, and only performs
explicit, reviewable filesystem operations.

The user-facing command name is `devspace`. The repository is still named
`devdrop`, but the product and code now consistently use the DevSpace naming.

## What It Does

DevSpace answers a narrow question: "Which projects should exist in this
workspace, what metadata do they need, and what is safe to do automatically?"

It does this by:

- scanning a workspace for projects, Git metadata, `.env` presence, and setup
  hints;
- saving shared workspace structure in `<workspace>/.devspace/manifest.json`;
- saving machine-local runtime state in `DEVSPACE_HOME` / `~/.devspace`;
- syncing only the manifest through either a user-owned Git remote or an opt-in
  hosted manifest control plane;
- planning and applying safe placeholder directory creation before hydration;
- hydrating Git projects with explicit `git clone`;
- managing encrypted per-project environment profiles with `age`;
- exposing a terminal dashboard for safe workspace operations.

DevSpace does not merge project source code, install dependencies implicitly,
delete projects, auto-pull project repositories, or upload secrets/source files
as part of manifest sync.

## Main Data Model

The core shared types live in `internal/devspace/types.go`.

- `Config` is machine-local configuration: machine identity, workspace root,
  manifest remote, hosted endpoint/token, and commit attribution.
- `State` is machine-local observed state: project hydration, dirty status,
  current branch, last scan/sync timestamps, and hosted sync baseline.
- `Manifest` is the shared workspace document. It contains a version, localized
  workspace root, machines, projects, users, teams, and access records.
- `Project` is the primary sync unit. It includes stable ID, display name,
  relative path, type (`git`, `local`, `external`), remote, hydrate mode,
  env profiles, ignore rules, and setup hints.
- `ProjectState` is deliberately separate from `Project`; it captures what this
  machine currently sees on disk.

`internal/devspace/manifest.go` owns manifest validation and persistence. The
important invariant is that project paths must be relative to the workspace root
and pass `safeWorkspacePath`; synced manifests are treated as untrusted input.

## Command Layout

`cmd/devspace/main.go` starts the CLI and `internal/devspace/commands.go`
registers commands.

Top-level commands cover local workflow:

- `init` creates config, state, identity, and the initial workspace manifest.
- `scan` refreshes manifest/state from the workspace.
- `watch` runs an event-driven scanner.
- `ui` opens the interactive dashboard.
- `status`, `doctor`, `plan`, and `apply` expose read/diagnose/review/apply
  loops.
- `project hydrate` clones placeholder Git projects.
- `env` manages encrypted project env profiles.
- `setup` reviews and runs explicit setup commands.
- `mount` previews a read-only FUSE workspace view.

The `workspace` command group owns Git-backed manifest sync:

- `workspace remote` configures or creates the manifest remote.
- `workspace push` writes only `manifest.json` to the remote repo.
- `workspace pull` validates, localizes, and replaces the local manifest.
- `workspace diff` previews remote manifest differences as `ManifestDiff`.
- `workspace reconcile` computes a three-way (or two-way fallback) merge
  between local and remote manifests, reviewable before an explicit `--apply`.

The `hosted` command group owns the opt-in hosted control-plane prototype:

- `hosted serve` runs a local HTTP manifest server.
- `hosted config` stores endpoint/workspace/token configuration.
- `hosted push` and `hosted pull` sync manifest metadata with version/hash
  conflict checks.
- `hosted reconcile` runs the same merge/review/apply flow as `workspace
  reconcile`, resolving hosted 409/version conflicts.

## Manifest Sync Flow

Git-backed sync lives in `internal/devspace/workspace_sync.go`.

`PushWorkspaceManifest` loads the local manifest, normalizes it for sync with
`manifestForSync`, ensures the manifest repo is clean and not behind, writes
`manifest.json`, commits, and pushes. It refuses when the remote branch is newer
or diverged.

`PullWorkspaceManifest` loads the previous remote manifest from the local
manifest repo cache, pulls the remote repo, validates the new remote manifest,
localizes it for the current machine, and refuses if local unpushed manifest
changes would be overwritten.

`DiffWorkspaceManifest` shares the same remote localization path, then compares
local and remote manifests and returns added, removed, and changed projects.

Hosted sync lives in `internal/devspace/hosted_sync.go`. It uses a versioned
HTTP envelope with a manifest hash. Push refuses if the hosted version changed
since the local baseline; pull refuses if the local manifest changed since the
last hosted sync. Both paths sync manifest metadata only.

## Tasks TUI Dashboard

The TUI dashboard (spec 05) added the built-in Bubble Tea dashboard:

```text
cd3cd8d feat: add devspace ui interactive dashboard (spec 05) (#30)
```

The spec and task artifacts are under
`docs/specs/05-spec-tui-dashboard/`.

The implementation is split into three files:

- `internal/devspace/ui.go` defines the `devspace ui` Cobra command, the
  `--no-watch` flag, the TTY guard, and the Bubble Tea program launch.
- `internal/devspace/ui_model.go` defines the dashboard model, messages,
  update loop, rendering, project rows, status labels, summary, events pane,
  selection handling, and key bindings.
- `internal/devspace/ui_actions.go` defines dashboard commands for scan, plan,
  apply-safe, hydrate, manual refresh, and the live fsnotify watcher.

The dashboard intentionally exposes only safe operations:

- `s` runs scan.
- `p` builds and saves a plan.
- `a` applies the saved safe plan.
- `h` hydrates the selected project.
- `r` refreshes via the watch refresh path.

It does not expose sync, hosted configuration, env-secret editing, destructive
operations, or project Git operations. The action path is single-flight: a busy
dashboard rejects a second action until the current one completes. Each action
goes through `withAppLock` at the dashboard boundary, and comments in
`ui_actions.go` call out that the domain functions used there must not acquire
the non-reentrant app lock themselves.

The model renders from shared domain state:

1. Scan/refresh updates manifest and state.
2. `dashboardRowsFromState` loads `Config`, `Manifest`, and `State`.
3. Rows are sorted by manifest path.
4. `dashboardStatus` maps `ProjectState` into `Hydrated`, `Placeholder`, or
   `Missing`.
5. The view renders project, type, status, dirty flag, branch, env presence,
   summary counts, and recent watch events.

## Manifest Reconciliation Flow

Manifest reconciliation (spec 06) is implemented. `internal/devspace/reconcile.go`
wraps the merge engine in `internal/devspace/manifest_merge.go`
(`mergeManifests(base, ours, theirs)`) with `reconcileManifests(base *Manifest,
local, remote Manifest) (ReconcileResult, error)`:

- **With a base** (`<app home>/last-synced-manifest.json`, written after
  successful Git/hosted push, pull, and reconcile apply — see
  `base_manifest.go`), it runs the three-way engine: `Projects` are merged by
  `Project.Path` (a same-path pair with different `Project.ID` is reported as
  an `"id"` conflict, not auto-unioned); `Users` and `Teams` get a record-level
  three-way merge (one-sided change wins, both-sided-and-different is a
  conflict); `Machines` are excluded (stripped by `manifestForSync` before
  reconciliation).
- **Without a base** (first run after upgrade, or no prior sync), it falls
  back to a documented two-way merge (`ReconcileResult.TwoWay`): one-sided
  adds/removes still merge automatically, but any same-path/same-key
  difference between local and remote becomes a conflict. It never refuses
  outright and never silently picks a side.
- Real conflicts block `--apply` unless the caller passes the mutually
  exclusive global `--force-local` / `--force-remote` flags, or repeats
  `--force-project <projectID>=<local|remote>` to resolve individual project
  conflicts.

Two commands drive this: `devspace workspace reconcile` (Git-backed sync) and
`devspace hosted reconcile` (hosted 409/version conflicts), both wired in
`commands.go` and taking `withAppLock` at the command boundary. Each supports
`--json` and `--apply`. Review-first by default: the plan (ops summary +
conflicts) is written to `DEVSPACE_HOME/last-reconcile.json`
(`reconcilePlanPath`) without changing the manifest. `--apply` re-checks the
local manifest hash against the plan, backs up the pre-reconcile manifest to
`DEVSPACE_HOME/manifest-backup.json` (`manifestBackupPath`), writes the merged
manifest, and leaves the base snapshot at the last published sync point until
the merged manifest is pushed or pulled. Plain `push`/`pull` keep their existing
refuse-on-divergence behavior; `reconcile` is the opt-in path for divergence
that the plain flow refuses.

The design questions the earlier spike (`docs/architecture/manifest-merge.md`)
left open are resolved: two-way fallback (not refuse) when there's no base;
keying by `Project.Path` (not `Project.ID`), with same-path/different-ID as a
conflict; and `--force-local`/`--force-remote`/`--force-project` flags (not
`--force-mine`/`--force-theirs`).

## How The Pieces Connect

The architecture is intentionally layered:

1. CLI commands (`commands.go`, `ui.go`) parse user intent and acquire
   `withAppLock` at command/action boundaries.
2. Domain services (`workspace.go`, `workspace_sync.go`, `hosted_sync.go`,
   `watch.go`, `mount.go`, `setup.go`, `secrets.go`) perform operations.
3. Persistence helpers (`manifest.go`, `config.go`, `jsonio.go`, `paths.go`,
   `lock.go`) validate paths, read/write JSON atomically, and maintain app-home
   state.
4. Output helpers (`output.go`, `styles.go`, `diagnostics.go`) render either
   styled terminal output or stable JSON.

The main data loop is:

```text
workspace files
  -> scan/watch
  -> Manifest + State
  -> status/doctor/plan/ui
  -> apply/hydrate/setup/env actions
  -> updated workspace files and app-home state
```

Sync adds a second loop:

```text
local Manifest
  -> manifestForSync
  -> Git remote or hosted envelope
  -> localized remote Manifest
  -> diff / pull / reconcile
  -> local Manifest
```

## Recent Commit Context

`main` history (release of record: `v0.2.0`) shows the project moving from
release hardening into an interactive, sync-aware workspace manager:

- `feat: add devspace ui interactive dashboard (spec 05) (#30)` added the TUI.
- `docs: spec 06 manifest reconciliation planning artifacts (#31)` and the
  subsequent `reconcile.go` work implemented three-way/two-way manifest
  reconciliation.
- `feat: warning-only access-role advisories (spec 07) (#34)` added advisory
  access-role warnings.
- `feat: per-project reconcile force and read-only sync status in devspace ui
  (spec 08) (#35)` added `--force-project` and sync status in the dashboard.
- `feat(ui): opentui-based devspace-tui companion dashboard (#39)` introduced
  the external `devspace-tui` companion frontend.

## Current Gaps

- Users/Teams reconciliation is record-level (whole record wins or conflicts),
  not field-level; a losing side's unrelated field changes are not preserved.
  Machines are excluded from reconciliation entirely.
- `devspace ui` surfaces read-only sync/reconcile status (last sync/base time,
  remote configuration state) but does not itself initiate push/pull/reconcile
  actions; it remains intentionally limited to safe, local operations.
- Hosted sync remains a runnable prototype rather than a managed service.
- Hydration is full `git clone`; partial clone and sparse checkout are not yet
  implemented.

## Review Notes For Future Work

Spec 06 (manifest reconciliation) established the following safety shape; keep
it for future work in this area:

- add tests against temp `DEVSPACE_HOME` and temp workspaces only;
- keep `withAppLock` at command boundaries and avoid nested lock acquisition;
- use existing manifest validation, localization, and atomic JSON helpers;
- make conflict output text-label based, not color-only;
- document any force flags as explicit conflict-resolution choices, not as a
  generic overwrite escape hatch;
- preserve the current invariant that manifest sync never uploads project
  source, dependency folders, `.env` files, or secret payloads.
