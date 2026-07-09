# Spec 13 Task 2 Proofs

## Summary

PASS. Git-backed manifest synchronization is available only through `sync push|pull|diff|reconcile|remote`; project membership and repository updates are available only through `project list|track|untrack|update`. The isolated two-machine workflow recreated a tracked Git project from metadata, hydrated it explicitly, emitted parseable JSON, and proved that untracking retained the checkout. All paths, remotes, machine identifiers, project identifiers, commits, and timestamps below are sanitized.

## Test-first evidence

Sync command RED:

```text
$ go test ./internal/devspace -run '^TestSyncCommand' -count=1
--- FAIL: TestSyncCommandSurface
    sync commands = [], want [diff pull push reconcile remote]
--- FAIL: TestSyncCommandRemoteCreateSetGet
    unknown command "remote" for "devspace sync"
FAIL
```

Sync command GREEN:

```text
$ go test ./internal/devspace -run '^TestSyncCommand' -count=1
ok  github.com/liatrio-forge/devdrop-capstone/internal/devspace
```

Project command RED:

```text
$ go test ./internal/devspace -run 'TestProject(Command|Untrack|Help)' -count=1
--- FAIL: TestProjectCommandSurface
    project commands = [add hydrate remove update], want [list track untrack update]
--- FAIL: TestProjectCommandTrackAndStatus
    unknown command "track" for "devspace project"
--- FAIL: TestProjectUntrackCommandOutputRetainsSecrets
    unknown command "untrack" for "devspace project"
FAIL
```

Project command GREEN:

```text
$ go test ./internal/devspace -run 'TestProject(Command|Untrack|Help)' -count=1
ok  github.com/liatrio-forge/devdrop-capstone/internal/devspace
```

Project update progress regression RED/GREEN:

```text
$ go test ./internal/devspace -run '^TestProjectUpdateCommandHydratesWhenPiped$' -count=1
--- FAIL: TestProjectUpdateCommandHydratesWhenPiped
    expected plain progress line for project update, got "hydrate lazy: updated..."
FAIL

$ go test ./internal/devspace -run '^TestProjectUpdateCommandHydratesWhenPiped$' -count=1
ok  github.com/liatrio-forge/devdrop-capstone/internal/devspace
```

Focused preservation gates:

```text
$ go test ./internal/devspace -run 'Test(Workspace(Remote|Push|Pull|Diff|GitBacked|Reconcile)|Sync|Reconcile|Hosted.*(Sync|Reconcile)|Doctor|.*AccessRole)' -count=1
ok  github.com/liatrio-forge/devdrop-capstone/internal/devspace

$ go test ./internal/devspace -run 'TestSyncCommand|TestProject(Command|Update|Untrack|Track)' -count=1
ok  github.com/liatrio-forge/devdrop-capstone/internal/devspace

$ go test ./internal/devspace -run 'Test(ProjectUpdate|ProjectUntrack|RemoveProject|UIServerRemove|ProjectCommandList|ProjectList)' -count=1
ok  github.com/liatrio-forge/devdrop-capstone/internal/devspace
```

These gates cover metadata-only Git transport, remote validation, second-workspace localization, manifest backups, divergence and hash guards, reconcile force flags, advisory labels, missing and empty hydration, clean fast-forward, dirty/detached/local-only/no-remote/non-Git skips, retained files and encrypted profiles, and stable ANSI-free JSON.

## Isolated two-machine workflow

Fixture:

```text
DEVSPACE_HOME A: <proof-root>/home-a
Workspace A:     <proof-root>/machine-a/code
DEVSPACE_HOME B: <proof-root>/home-b
Workspace B:     <proof-root>/machine-b/code
Manifest remote: <proof-root>/manifest.git
Project remote:  <proof-root>/project-api.git
```

Machine A tracked an existing checkout and published only manifest metadata:

```text
$ devspace --no-color init --workspace <proof-root>/machine-a/code
Initialized DevSpace workspace: <proof-root>/machine-a/code
Machine: <machine-a> (<machine-a-id>)

$ devspace --no-color project track apps/api
Tracked project api at apps/api

$ devspace --no-color project list
NAME  PATH      TYPE  STATUS    DIRTY  BRANCH  ENV
api   apps/api  git   hydrated  no     main    no

$ devspace --no-color sync remote create local <proof-root>/manifest.git
<proof-root>/manifest.git

$ devspace --no-color sync push
Pushed workspace manifest.
```

Machine B pulled metadata, reviewed and applied the safe placeholder plan, then explicitly hydrated repositories:

```text
$ devspace --no-color init --workspace <proof-root>/machine-b/code
Initialized DevSpace workspace: <proof-root>/machine-b/code
Machine: <machine-b> (<machine-b-id>)

$ devspace --no-color sync remote set <proof-root>/manifest.git
<proof-root>/manifest.git

$ devspace --no-color sync pull
Pulled workspace manifest.
Next: devspace plan && devspace apply
Then: devspace project update --all

$ devspace --no-color plan
SAFE:
PLACEHOLDER apps/api
No destructive changes will be performed.

$ devspace --no-color apply
Applied safe plan actions.

$ devspace --no-color project update --all
Updating projects...
hydrate api: updated
Updated projects: 1 updated, 0 skipped, 0 failed
```

The canonical JSON list remained parseable and retained the established `project` plus `state` row contract:

```json
[
  {
    "project": {
      "id": "<project-id>",
      "name": "api",
      "path": "apps/api",
      "type": "git",
      "remote": "<proof-root>/project-api.git",
      "defaultBranch": "main",
      "hydrateMode": "on-demand"
    },
    "state": {
      "hydrated": true,
      "exists": true,
      "dirty": false,
      "currentBranch": "main",
      "lastCommit": "<commit>",
      "placeholder": false,
      "missing": false
    }
  }
]
```

Untracking removed metadata while preserving local source and Git data:

```text
$ devspace --no-color project untrack api
Untracked project api (apps/api) from the manifest. Files on disk were not touched.

$ test -d <proof-root>/machine-b/code/apps/api/.git
FILES_REMAIN=yes
```

Removed paths were also rejected by command-contract tests, including `workspace`, `workspace scan`, `workspace push`, `project add`, `project remove`, `project hydrate`, and `project status`.

## Full verification

```text
$ go test ./... -count=1
?   github.com/liatrio-forge/devdrop-capstone/cmd/devspace  [no test files]
ok  github.com/liatrio-forge/devdrop-capstone/internal/devspace

$ make precommit
0 issues.
ok  github.com/liatrio-forge/devdrop-capstone/internal/devspace

$ make verify
0 issues.
No vulnerabilities found.
```
