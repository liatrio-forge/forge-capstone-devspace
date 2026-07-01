# Capstone Spec: DevDrop

## Problem

A developer's workspace is more than source code. It includes which projects are
active, where those projects live, which remotes they came from, which repos are
dirty, which `.env` files are expected, and which setup commands are likely
needed. Today that context is rebuilt manually after a new laptop, client
rotation, or cleanup pass.

The unsafe shortcut is to sync entire folders. That risks copying secrets,
large dependency trees, generated files, and client source code. DevDrop takes a
metadata-first approach.

## Users

- Consultants who rotate across client projects and need predictable workspace
  recovery.
- Developers with multiple machines who want the same project map everywhere.
- Team leads who need a teachable example of safe AI-assisted product delivery.

## Goal

Ship a local-first CLI that can recreate workspace structure on another machine
from a synced manifest while preserving local control over source code and
secrets.

Use the delivery process itself as part of the capstone: demonstrate how
remote/cloud agents can execute well-scoped cards through the checked-in
`.claude/workflows/wave-ship.js` workflow, then include that evidence in the
case study and final demo.

## Non-Goals

- Hosted sync service.
- Filesystem daemon or watcher.
- FUSE or virtual filesystem.
- Full source-code syncing.
- Dependency installation automation.
- Team secret sharing or remote secret backup.
- Automatic Git pull, merge, rebase, or conflict resolution for project repos.

## User Stories

### Initialize a workspace

As a developer, I can run `devspace init --workspace ~/code` so DevDrop creates
local config, a machine identity, an age identity, and a workspace manifest.

Acceptance:

- Repeat runs do not rotate the machine ID.
- Repeat runs do not rotate the age identity.
- Existing manifest projects are preserved.

### Discover workspace projects

As a developer, I can run `devspace scan` so DevDrop records project metadata
without crawling dependency folders or nested repos inside a parent repo.

Acceptance:

- Git remotes, current branch, last commit, dirty state, `.env` presence, and
  setup hints are captured.
- Default ignored folders include `node_modules`, `dist`, `build`, `.next`,
  `turbo`, `target`, `vendor`, `coverage`, `.cache`, `.DS_Store`, and `*.log`.
- Duplicate project basenames are disambiguated predictably.

### Review before writing

As a developer, I can run `devspace plan` before `devspace apply` so filesystem
changes are visible before they happen.

Acceptance:

- `plan` saves a plan with the current manifest hash.
- `plan --json` emits machine-readable plan output.
- `apply` refuses to run if the manifest changed after the plan was generated.
- Safe actions create missing directories only.
- Skipped actions identify the reason.

### Sync workspace metadata through Git

As a developer, I can push and pull the manifest through a user-owned Git remote
so another machine can recreate the workspace shape.

Acceptance:

- `workspace remote create local <path>` creates a local bare Git remote.
- `workspace remote create github <owner/repo> --private` uses the GitHub CLI.
- `workspace push` writes only `manifest.json`.
- Synced manifests do not include machine-local workspace roots.
- `workspace pull` validates remote JSON and manifest paths before replacing
  the local manifest.
- Pull creates a `.bak` backup of the previous local manifest.
- Pull refuses to overwrite local manifest changes that have not been pushed or
  reconciled.

### Hydrate projects on demand

As a developer, I can run `devspace project hydrate <project>` so a placeholder
Git project becomes a real clone when I need it.

Acceptance:

- Hydration clones only into missing or empty directories.
- Hydration refuses non-empty destination folders.
- Hydration gives useful errors when a remote is missing or unreachable.
- Suggested setup commands are printed but not executed.

### Manage local encrypted env profiles

As a developer, I can store env values locally with age encryption and generate
`.env` files only when I explicitly request them.

Acceptance:

- `env set` accepts stdin or a hidden prompt.
- Empty or invalid keys are rejected.
- `env list` masks values.
- Secret profile files do not contain plaintext values.
- `env pull` writes the local `.env` file with `0600` permissions.

## Quality Bar

- `go test ./...` passes.
- `go vet ./...` passes before release.
- `go build -o .tmp/devspace ./cmd/devdrop` produces the demo binary.
- The release readiness document states tested workflows, known limitations,
  and remaining risks.
- The demo uses temporary directories and local bare remotes so it can run
  without network access.
- Remote-agent delivery evidence identifies cards, PRs, verification commands,
  blockers, and human decisions without exposing secrets or hidden local state.
