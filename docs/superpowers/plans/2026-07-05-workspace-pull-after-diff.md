# Workspace Pull-After-Diff Fix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `devspace workspace pull` succeed after `devspace workspace diff` when the local manifest has no unpushed changes.

**Architecture:** `PullWorkspaceManifest` currently infers "what did I last sync?" from the cached manifest clone's pre-pull `manifest.json` (`internal/devspace/workspace_sync.go:188`). `DiffWorkspaceManifest` fast-forwards that same cached clone (`fetchLocalizedWorkspaceRemoteManifest` → `pullManifestRepo`, workspace_sync.go:259), so a diff run before a pull makes the "previous remote" look identical to the new remote, and the unpushed-changes guard (workspace_sync.go:207) wrongly reports `local manifest differs from remote manifest`. The fix: prefer the purpose-built base snapshot `base-manifest.json` (written by `recordBaseManifest` on every push/pull/reconcile, read by `loadBaseManifest` in `internal/devspace/base_manifest.go`) as the guard's baseline, falling back to the clone's pre-pull contents only when no base has been recorded (first pull on a machine). No changes to `DiffWorkspaceManifest` — once the guard stops depending on the clone's incidental state, diff advancing the cache is harmless.

**Tech Stack:** Go, stdlib `testing`, single package `internal/devspace`.

## Global Constraints

- All state must be test-isolated via `t.Setenv(envHome, ...)` pointing at temp dirs (repo testing convention; never touch the real `~/.devspace`).
- `plan`/`pull` must never overwrite local user data that hasn't been synced — the guard's refusal behavior for genuinely-unpushed local changes must be preserved exactly (existing test `TestWorkspacePullRefusesToOverwriteLocalUnpushedChanges` must keep passing).
- Local gate before pushing: `make verify` (test + vet + lint + build).
- Error message text `"local manifest differs from remote manifest; push or reconcile local changes before pulling"` is asserted by existing tests via substring `"local manifest differs"` — do not change it.

## Known behavior note (accepted, not in scope)

`base-manifest.json` is shared between the Git-remote and hosted-sync backends (both call `recordBaseManifest`). A user actively mixing both backends could have a base recorded by hosted sync used as the git-pull baseline. This matches how `reconcile.go` already consumes the base and means "last state synced with *a* remote," which is the intended semantic. No task addresses this.

---

### Task 1: Pull guard uses recorded base manifest

**Files:**

- Modify: `internal/devspace/workspace_sync.go:176-225` (`PullWorkspaceManifest`)
- Test: `internal/devspace/workspace_sync_test.go` (append new test after `TestWorkspacePullAllowsFastForwardWhenLocalMatchesPreviousRemoteManifest`, which ends at line 465)

**Interfaces:**

- Consumes: `loadBaseManifest() (Manifest, bool, error)` from `internal/devspace/base_manifest.go` (already exists — do not redefine it). Test helpers already in the package: `workspaceSyncBareRepo(t)`, `hardeningProject(path, projectType, remote string) Project`, `findProject(m Manifest, path string) (Project, bool)`, `InitWorkspace`, `SetManifestRemote`, `SaveManifest`, `PushWorkspaceManifest`, `PullWorkspaceManifest`, `DiffWorkspaceManifest`, `LoadManifest`, constants `envHome`, `ManifestVersion`, `ProjectTypeLocal`.
- Produces: no new exported API. Behavior change only: `PullWorkspaceManifest` fast-forwards when the local manifest matches the recorded base, even if the cached clone was advanced by an earlier `diff`.

- [ ] **Step 1: Write the failing test**

Append to `internal/devspace/workspace_sync_test.go`:

```go
func TestWorkspacePullSucceedsAfterDiffWhenLocalHasNoUnpushedChanges(t *testing.T) {
 root := t.TempDir()
 remote := workspaceSyncBareRepo(t)
 workspaceA := filepath.Join(root, "a")
 workspaceB := filepath.Join(root, "b")

 // Machine A publishes the initial manifest.
 t.Setenv(envHome, filepath.Join(root, "home-a"))
 if _, err := InitWorkspace(workspaceA); err != nil {
  t.Fatal(err)
 }
 if _, err := SetManifestRemote(remote); err != nil {
  t.Fatal(err)
 }
 if err := SaveManifest(workspaceA, Manifest{
  Version:       ManifestVersion,
  WorkspaceRoot: workspaceA,
  Projects:      []Project{hardeningProject("apps/one", ProjectTypeLocal, "")},
 }); err != nil {
  t.Fatal(err)
 }
 if _, err := PushWorkspaceManifest(); err != nil {
  t.Fatal(err)
 }

 // Machine B pulls, adds a project, and pushes.
 t.Setenv(envHome, filepath.Join(root, "home-b"))
 if _, err := InitWorkspace(workspaceB); err != nil {
  t.Fatal(err)
 }
 if _, err := SetManifestRemote(remote); err != nil {
  t.Fatal(err)
 }
 if _, err := PullWorkspaceManifest(); err != nil {
  t.Fatal(err)
 }
 updated, err := LoadManifest(workspaceB)
 if err != nil {
  t.Fatal(err)
 }
 updated.Projects = append(updated.Projects, hardeningProject("apps/two", ProjectTypeLocal, ""))
 if err := SaveManifest(workspaceB, updated); err != nil {
  t.Fatal(err)
 }
 if _, err := PushWorkspaceManifest(); err != nil {
  t.Fatal(err)
 }

 // Machine A runs diff first — this fast-forwards the cached manifest
 // clone — then pulls. Machine A has no unpushed local changes, so the
 // pull must fast-forward, not refuse.
 t.Setenv(envHome, filepath.Join(root, "home-a"))
 if _, err := DiffWorkspaceManifest(); err != nil {
  t.Fatal(err)
 }
 changed, err := PullWorkspaceManifest()
 if err != nil {
  t.Fatalf("pull after diff failed: %v", err)
 }
 if !changed {
  t.Fatal("pull after diff reported no change")
 }
 pulled, err := LoadManifest(workspaceA)
 if err != nil {
  t.Fatal(err)
 }
 if _, ok := findProject(pulled, "apps/two"); !ok {
  t.Fatalf("pull after diff missing project: %+v", pulled.Projects)
 }
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/devspace -run TestWorkspacePullSucceedsAfterDiffWhenLocalHasNoUnpushedChanges -v`

Expected: FAIL with `pull after diff failed: local manifest differs from remote manifest; push or reconcile local changes before pulling`

If it fails any other way (compile error, helper missing), fix the test — the failure must be the guard refusing the pull.

- [ ] **Step 3: Implement the fix**

In `internal/devspace/workspace_sync.go`, inside `PullWorkspaceManifest`, replace:

```go
 previousRemote, hasPreviousRemote, err := loadSyncedManifestIfExists(repo)
 if err != nil {
  return false, err
 }
```

with:

```go
 previousRemote, hasPreviousRemote, err := loadSyncedManifestIfExists(repo)
 if err != nil {
  return false, err
 }
 // The clone's pre-pull contents are only a proxy for the last synced
 // state and go stale whenever something else advances the cache (e.g.
 // `workspace diff` fast-forwards the same clone). Prefer the base
 // snapshot recorded on every push/pull/reconcile when one exists.
 base, hasBase, err := loadBaseManifest()
 if err != nil {
  return false, err
 }
 if hasBase {
  previousRemote, hasPreviousRemote = base, true
 }
```

No other changes. `localHasUnpushedManifestChanges` already normalizes `previousRemote` through `manifestForSync`, and the base is stored in that normalized form, so the comparison is byte-stable.

- [ ] **Step 4: Run the new test to verify it passes**

Run: `go test ./internal/devspace -run TestWorkspacePullSucceedsAfterDiffWhenLocalHasNoUnpushedChanges -v`

Expected: PASS

- [ ] **Step 5: Run the guard-behavior regression tests**

Run: `go test ./internal/devspace -run 'TestWorkspacePull|TestWorkspaceDiff|TestReconcile' -v`

Expected: all PASS. `TestWorkspacePullRefusesToOverwriteLocalUnpushedChanges` still passes because its second machine (`home-b`) has never pushed or pulled, so no `base-manifest.json` exists there and the guard falls back to the old clone-based baseline.

- [ ] **Step 6: Run the full local gate**

Run: `make verify`

Expected: tests, vet, lint, and build all succeed.

- [ ] **Step 7: Commit**

```bash
git add internal/devspace/workspace_sync.go internal/devspace/workspace_sync_test.go
git commit -m "fix: workspace pull no longer refuses after diff advances manifest cache

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```
