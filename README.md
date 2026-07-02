# DevSpace

DevSpace is a local-first "Dropbox for developers" CLI prototype. It keeps a
developer workspace structurally aligned across machines by tracking project
metadata, safe placeholder folders, Git remotes, setup hints, and encrypted env
profiles.

The intended command name is `devspace`. The current binary can also be built
from this repository:

```bash
go build -o bin/devspace ./cmd/devspace
```

During development, you can still run the command directly from source:

```bash
go test ./...
go run ./cmd/devspace --help
```

## Release Packaging

Releases are automated with GoReleaser: pushing a `v*` tag publishes a GitHub
Release with prebuilt `devspace` archives for Linux and macOS (amd64/arm64),
SHA256 checksums, and build-provenance attestation. Download archives from the
[releases page](https://github.com/HexSleeves/devdrop/releases) and verify them
with `sha256sum -c` and `gh attestation verify`.

CI (`go test`, `go vet`, build) runs on every PR and push to `main`. The same
gate is available locally:

```bash
make verify
```

See [`docs/release.md`](docs/release.md) for the full release process,
consumer verification steps, install-from-source instructions, and the manual
`make release` fallback.

## Current MVP Status

This MVP is local-first. It proves the workflow before adding filesystem-level
lazy loading.

What works today:

- Initialize a workspace and local machine identity.
- Generate a versioned workspace manifest.
- Scan existing projects for Git metadata, `.env` presence, dependency files,
  and setup hints.
- Show workspace and project status.
- Diagnose local readiness before syncing, planning, applying, or hydrating.
- Add local projects to the manifest.
- Plan and apply safe missing project structure as empty placeholder folders.
- Hydrate placeholder Git projects with normal `git clone`.
- Push and pull the workspace manifest through a user-owned Git repository.
- Opt into a hosted manifest sync control-plane prototype with explicit
  endpoint/token configuration.
- Review detected dependency install/dev commands and run them only through an
  explicit setup command.
- Store encrypted per-project env profiles with native age encryption.
- Generate local `.env` files with `0600` permissions.
- Keep workspace metadata fresh with an event-driven `devspace watch` mode.
- Preview a FUSE-backed lazy workspace mount prototype without requiring FUSE for
  normal CLI workflows.

## Capstone Artifacts

This repository is being prepared as a Liatrio Forge Module 5 capstone. See
[`docs/capstone/README.md`](docs/capstone/README.md) for the capstone spec,
proof checklist, case study, demo script, and playbook contribution. Open
[`docs/capstone/index.html`](docs/capstone/index.html) for an interactive HTML
reader generated from the repository Markdown.

## Supported Commands

### `devspace init`

```bash
devspace init --workspace ~/code
```

Creates:

- `~/.devspace/config.json`
- `~/.devspace/state.json`
- `~/.devspace/identity.txt`
- `<workspace>/.devspace/manifest.json`

The command is idempotent and does not rotate the machine ID or age identity on
repeat runs.

### `devspace scan`

```bash
devspace scan
```

Scans the configured workspace and updates the manifest/state with:

- Git remote URL
- Current branch
- Last commit
- Dirty working tree status
- `.env` file presence
- Dependency/setup hints

Ignored by default:

```text
node_modules/
dist/
build/
.next/
turbo/
target/
vendor/
coverage/
.cache/
.DS_Store
*.log
```

### `devspace watch`

```bash
devspace watch
devspace watch --debounce 3s
devspace watch --sync git
devspace watch --sync hosted
devspace watch --once
```

Runs a long-lived workspace watcher that debounces filesystem events and refreshes
the same manifest/state metadata as `devspace scan`. It watches the configured
workspace root, skips `.devspace/` and the existing dependency/build/cache ignore
rules, and tracks manifest-relevant additions, removals, package marker changes,
`.env` presence changes, and Git branch/index/ref changes.

Default behavior is local-only: after each refresh, the watcher writes the local
manifest and state files atomically and continues watching. `--sync git`
explicitly pushes the refreshed
manifest to the configured Git-backed manifest remote after each refresh.
`--sync hosted` explicitly pushes the normalized manifest to the configured
hosted control-plane prototype after each refresh. Watch mode never pulls remote
manifests, applies saved plans, hydrates repositories, installs dependencies,
runs setup commands, uploads secrets, or uploads source files. Use `--once` for
local smoke tests and demos that should perform one refresh and exit.

### `devspace status`

```bash
devspace status
devspace project status client-a-api
```

Shows tracked projects, hydrated projects, placeholders, dirty repos, missing env
files, stale/missing projects, and last scan/sync timestamps.

### `devspace doctor`

```bash
devspace doctor
```

Checks local readiness without changing files or contacting hosted services. It
reports config, workspace, manifest, Git, manifest remote/cache, saved plan, and
tracked project path status. It exits non-zero only for hard failures that block
core commands; stale plans, dirty repos, placeholders, and missing `.env` files
are reported as warnings.

### `devspace mount`

```bash
devspace mount ~/devspace-view
devspace mount ~/devspace-view --preview
```

Prototype read-only FUSE workspace view. Manifest project paths appear as mount
entries before they are hydrated, and accessing an on-demand Git project through
the mount uses the same safe hydration checks as `devspace project hydrate`.
`--preview` prints the projected entries without mounting and is the fallback for
machines without macFUSE, `/dev/fuse`, or FUSE mount permissions. See
[`docs/fuse-lazy-mount.md`](docs/fuse-lazy-mount.md) for platform requirements,
library selection, and follow-up work.

### `devspace project add`

```bash
devspace project add work/client-a-api
```

Adds a relative workspace path to the manifest. Existing Git repositories are
recorded as Git projects; otherwise the project is tracked as local-only.

### `devspace plan`

```bash
devspace plan
devspace plan --json
```

Builds a deterministic plan of safe and skipped actions, saves it to
`<workspace>/.devspace/last-plan.json`, and prints a human-readable report.
`--json` prints the same saved plan as structured JSON for automation.

### `devspace apply`

```bash
devspace apply
```

Applies the last saved plan only if the manifest hash still matches. If the
manifest changed after `plan`, `apply` refuses to run and asks you to re-plan.
Only safe actions are executed. Skipped actions remain listed and untouched.

### `devspace workspace sync`

```bash
devspace workspace sync --dry-run
devspace workspace sync
```

Compatibility alias. Prefer `devspace plan` and `devspace apply`.

### `devspace workspace remote`

```bash
devspace workspace remote set <git-url-or-local-path>
devspace workspace remote create local ~/Projects/devspace-manifest.git
devspace workspace remote create github HexSleeves/devspace-manifest --private
devspace workspace remote get
```

Stores the Git remote used for manifest sync in local config. The remote setting
is not written into the workspace manifest. `remote create local` initializes a
local bare Git repository and sets it as the remote. `remote create github` uses
the GitHub CLI (`gh`) to create the repository, then sets the SSH remote.

### `devspace workspace push`

```bash
devspace workspace push
```

Validates the local manifest, clones the manifest repo cache if needed, writes
only `manifest.json`, commits only when the manifest changed, and pushes to the
configured Git remote. If the manifest repo is dirty or the remote branch is
newer/diverged, the command stops and asks you to pull or reconcile first.

### `devspace workspace pull`

```bash
devspace workspace pull
devspace plan
devspace apply
```

Fetches the configured manifest repo, validates the remote `manifest.json`, backs
up the current local manifest, and atomically replaces it. It does not run
`apply`, hydrate projects, pull project repos, install dependencies, or overwrite
project contents.

### `devspace workspace diff`

```bash
devspace workspace diff
```

Fetches the configured manifest repo cache, validates the remote
`manifest.json`, localizes it to the current workspace, and reports projects
that would be added, removed, or changed by a future pull. It does not replace
the local manifest, apply plans, hydrate projects, pull source-code repos, or
write to the manifest remote.

### `devspace hosted`

Hosted sync is opt-in and separate from Git-backed sync. Configure it only when
you want the manifest copied to a hosted control plane:

```bash
devspace hosted serve --addr 127.0.0.1:8787 --store ~/.devspace/hosted-control-plane --token dev-token
devspace hosted config set http://127.0.0.1:8787 --token dev-token --workspace team-a
devspace hosted push
devspace hosted pull
```

The runnable prototype server exposes:

```text
GET /v1/workspaces/{workspace}/manifest
PUT /v1/workspaces/{workspace}/manifest
Authorization: Bearer <token>
```

Storage is one JSON envelope per hosted workspace under the server `--store`
directory. Each envelope contains the API version, hosted workspace id, monotonic
sync version, manifest hash, update timestamp, and normalized `manifest.json`.
The server does not receive source files, dependency folders, `.env` files,
encrypted secret payloads, plaintext secrets, or local workspace roots.

Hosted `push` sends only the normalized manifest (`workspaceRoot: "."`, no
machine-local paths) and records the returned hosted version/hash in local state.
Hosted `pull` localizes the manifest to the current workspace, writes the usual
`.bak` backup, and then expects the operator to run `devspace plan &&
devspace apply`.

Hosted conflict behavior is optimistic:

- `PUT` requires the caller's expected hosted version; stale writes return 409.
- `push` refuses when the hosted manifest changed since the last local hosted
  sync.
- `pull` refuses when the local manifest changed since the last local hosted
  sync.
- A first-time `pull` refuses to overwrite a non-empty local manifest that
  differs from hosted.

Hosted path safety runs client-side and server-side. Project paths must remain
relative to the workspace and hosted sync rejects paths containing `.env`,
`.git`, dependency, build, cache, coverage, or vendored directory components.
Git-backed `devspace workspace push/pull/diff` remains the fallback when hosted
sync is not configured or unavailable.

### `devspace project hydrate`

```bash
devspace project hydrate client-a-api
```

Hydrates a placeholder Git project with normal `git clone`. The placeholder is
an empty directory; hydration refuses to clone into non-empty directories and
does not delete files.

### `devspace setup`

```bash
devspace setup plan
devspace setup plan --json
devspace setup run client-a-api --yes
devspace setup run client-a-api --command dev --yes
devspace setup apply --yes
```

`setup plan` reports dependency setup hints already captured by `scan`, including
the detected package manager, install command, and dev command. It does not run
anything.

`setup run <project>` executes one reviewed command in the validated project
directory. By default it runs the install command and prompts for confirmation;
use `--command dev` to run a detected dev command, `--dry-run` to preview, or
`--yes` for non-interactive execution after review.

`setup apply` runs install commands for all detected projects after confirmation.
Known package managers are executed without a shell. Unknown snippets require
`--allow-unknown`, and commands that appear to install globally also require
`--allow-global`.

### `devspace env`

```bash
printf '%s\n' "$CLIENT_A_DATABASE_URL" | devspace env set client-a-api DATABASE_URL
devspace env list client-a-api
devspace env pull client-a-api
devspace env recipient export
devspace env recipient invite client-a-api teammate age1...
devspace env recipient revoke client-a-api teammate
```

`env set` reads from stdin when piped, or prompts for a hidden value when run in a
terminal. `env list` prints keys only, with values masked. `env pull` writes the
local project `.env` with `0600` permissions.

`env recipient export` prints this machine's public age recipient so another
developer can invite it. `env recipient invite` decrypts the local profile,
updates public recipient/access metadata in `manifest.json`, and re-encrypts the
profile for the local user plus the invited recipient. Use `--team <name>` on
`invite` to record team-oriented project access metadata.

`env recipient revoke` removes a recipient from future encrypted profile writes
and records revocation metadata. This is an envelope rewrap, not a clawback:
previously copied ciphertext, pulled `.env` files, and values already decrypted
by that recipient remain outside DevSpace's control. After changing team
membership, `env recipient rotate <project>` rewraps the profile for the current
active recipients without changing secret values.

Encrypted profiles are stored under:

```text
<workspace>/.devspace/secrets/<project-id>/<profile>.age
```

## Example Local Workflow

This workflow uses temporary directories and a local bare Git remote, so it does
not need network access.

```bash
go build -o bin/devspace ./cmd/devspace

tmp="$(mktemp -d)"
export DEVSPACE_HOME="$tmp/home"
workspace_a="$tmp/workspace-a"
remote_src="$tmp/remote-src"
remote_bare="$tmp/client-a-api.git"
manifest_remote="$tmp/manifest-sync.git"

mkdir -p "$remote_src"
git -C "$remote_src" init -b main
git -C "$remote_src" config user.email demo@example.com
git -C "$remote_src" config user.name "Demo User"
printf '# client-a-api\n' > "$remote_src/README.md"
git -C "$remote_src" add README.md
git -C "$remote_src" commit -m "initial"
git clone --bare "$remote_src" "$remote_bare"

bin/devspace init --workspace "$workspace_a"
mkdir -p "$workspace_a/work/client-a-api"
git clone "$remote_bare" "$workspace_a/work/client-a-api"
printf '{"scripts":{"dev":"vite"}}\n' > "$workspace_a/work/client-a-api/package.json"
bin/devspace scan
bin/devspace workspace remote create local "$manifest_remote"
bin/devspace workspace push
bin/devspace plan
bin/devspace apply
printf '%s\n' "$CLIENT_A_DATABASE_URL" | bin/devspace env set client-a-api DATABASE_URL
bin/devspace env list client-a-api
bin/devspace env pull client-a-api
bin/devspace status
```

## Two-Machine Git Sync Workflow

To simulate a second machine, use a local bare Git repo for the manifest.

```bash
workspace_b="$tmp/workspace-b"
manifest_remote="$tmp/manifest-sync.git"

export DEVSPACE_HOME="$tmp/home-a"
bin/devspace init --workspace "$workspace_a"
bin/devspace scan
bin/devspace workspace remote create local "$manifest_remote"
bin/devspace workspace push

export DEVSPACE_HOME="$tmp/home-b"
bin/devspace init --workspace "$workspace_b"
bin/devspace workspace remote set "$manifest_remote"
bin/devspace workspace pull
bin/devspace plan
bin/devspace apply
bin/devspace status
```

The second workspace now contains placeholder directories for tracked projects.
Run `bin/devspace project hydrate <project>` to clone a placeholder Git project
when the manifest includes its remote.

## Safety Guarantees

- Project paths in the manifest are relative to the workspace root.
- Absolute paths and parent-directory escapes are rejected.
- `plan` reports planned filesystem changes before writing anything.
- `plan --json` emits the same plan as machine-readable JSON.
- `apply` executes only a saved plan whose manifest hash still matches.
- `apply` creates missing directories only; it skips non-empty destinations.
- Plan reports path, dirty-repo, and remote conflicts instead of overwriting local work.
- Hydration clones only into missing or empty directories.
- `watch` refreshes manifest/state metadata only unless an explicit `--sync`
  push mode is selected.
- `watch --sync git` pushes only `manifest.json` through Git-backed sync.
- `watch --sync hosted` pushes only the normalized manifest envelope.
- Env values are encrypted at rest with age.
- Env profiles can be encrypted to multiple explicit age recipients.
- `env list` masks secret values.
- `env pull` writes `.env` with `0600` permissions.
- Git-backed sync stores only `manifest.json`.
- Manifest sync strips machine-local workspace paths from the synced manifest
  and localizes the manifest on pull.
- Hosted sync is opt-in, stores only normalized manifest metadata, and never
  uploads source code, dependency folders, `.env` files, encrypted secret
  payloads, or plaintext secret values.
- The MVP has no background process.

## Conflict Behavior

Manifest sync stops with a clear error when:

- No manifest remote is configured.
- Git is not installed.
- The configured manifest remote does not exist yet.
- The manifest repo cannot be cloned, fetched, pulled, or pushed.
- The manifest repo has uncommitted changes.
- The remote branch is newer or diverged.
- The pulled manifest is invalid JSON or fails manifest validation.
- Pull would overwrite local manifest changes.
- Hosted push/pull would overwrite local or remote manifest changes without a
  matching hosted version/hash baseline.

## What This Tool Will Not Do Without Permission

- Delete local projects, files, or directories.
- Overwrite existing project contents during apply or hydrate.
- Auto-pull, rebase, merge, or push project Git repositories.
- Auto-pull Git-backed or hosted workspace manifests from watch mode.
- Auto-apply plans or hydrate repositories from watch mode.
- Resolve Git conflicts.
- Install dependencies or run project setup commands during scan, pull, apply,
  hydrate, watch, daemon, or filesystem reads.
- Upload secrets, source code, or project files.
- Share env profiles with teammates without an explicit recipient invite.
- Rotate recipient access or replace local `.env` values without an explicit
  command.
- Watch the filesystem in the background.
- Mount a FUSE or virtual filesystem.

## Known Limitations

- Hosted manifest sync is a local runnable prototype, not a managed deployment.
- Placeholder hydration uses full `git clone`; partial clone and sparse checkout
  are not implemented.
- Secret profile sharing uses public age recipients and local re-encryption.
  There is no remote backup, OS keychain integration, managed identity provider,
  or guaranteed revocation of previously copied/decrypted material.
- Setup hints are informational during scan and sync; installs only run through
  explicit `devspace setup` commands.
- Editor settings, VS Code extensions, devcontainers, Nix, mise, and asdf are
  outside the MVP.

## Migration

On first run, an existing `~/.devdrop` application home is automatically
migrated to `~/.devspace`. `DEV_DROP_HOME` still works as a deprecated alias
for `DEVSPACE_HOME`.

## Troubleshooting

- If `devspace` is not found, build it with
  `go build -o bin/devspace ./cmd/devspace` and run `bin/devspace`.
- If `workspace push` says the manifest remote is not ready, create it first with
  `devspace workspace remote create github <owner/repo> --private` or
  `devspace workspace remote create local ~/Projects/devspace-manifest.git`.
- If commands use the wrong workspace, check `DEVSPACE_HOME` (or the deprecated
  `DEV_DROP_HOME` fallback) and `~/.devspace/config.json`.
- If `plan` reports a path conflict, inspect the existing directory and
  decide whether it should be tracked, renamed, or left unmanaged.
- If `project hydrate` fails, confirm the project has a remote URL in
  `<workspace>/.devspace/manifest.json` and that `git clone <remote> <path>` works
  by itself.
- If `env pull` cannot write `.env`, check project path permissions and whether a
  directory or symlink exists at `.env`.
- If secret values appear missing, run `devspace env list <project>` to verify
  the key exists before pulling.

## Roadmap

- Manifest conflict resolution and force flags.
- Hosted API or cloud object storage.
- Daemon process management for running watch mode outside a terminal.
- FUSE or virtual filesystem lazy loading.
- Git partial clone or sparse checkout.
- Managed team identity provider integration.
- OS keychain integration.
- Secret value rotation.
- Editor settings, devcontainer, Nix, mise, or asdf sync.
- Release-readiness checklist automation.

## Manifest

See [`examples/manifest.json`](examples/manifest.json).

The manifest is a versioned JSON file at:

```text
<workspace>/.devspace/manifest.json
```

Project paths are always relative to the workspace root. Absolute paths and
parent-directory escapes are rejected.
