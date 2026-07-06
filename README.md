# DevSpace

> **A local-first "Dropbox for developers" CLI prototype.**

DevSpace keeps a developer workspace structurally aligned across machines by tracking project metadata, safe placeholder folders, Git remotes, setup hints, and encrypted env profiles.

---

## 📖 Table of Contents

- [Overview](#overview)
  - [Current MVP Status](#current-mvp-status)
  - [Building from Source](#building-from-source)
- [Environment Variables](#environment-variables)
- [Release Packaging](#release-packaging)
- [Capstone Artifacts](#capstone-artifacts)
- [Supported Commands](#supported-commands)
  - [Core Workflow](#core-workflow)
  - [Git-Backed Workspace Sync](#git-backed-workspace-sync)
  - [Hosted Workspace Sync](#hosted-workspace-sync)
  - [Secrets & Environment](#secrets--environment)
- [Example Workflows](#example-workflows)
- [Architecture & Safety](#architecture--safety)
  - [Safety Guarantees](#safety-guarantees)
  - [Conflict Behavior](#conflict-behavior)
  - [What This Tool Will Not Do](#what-this-tool-will-not-do-without-permission)
- [Troubleshooting & Limitations](#troubleshooting--limitations)
  - [Migration](#migration)
- [Roadmap](#roadmap)
- [Manifest Structure](#manifest-structure)

---

## Overview

### Current MVP Status

This MVP is local-first. It proves the workflow before adding filesystem-level lazy loading.

**What works today:**

- **Initialization:** Initialize a workspace and local machine identity.
- **Scanning:** Generate a versioned workspace manifest. Scan existing projects for Git metadata, `.env` presence, dependency files, and setup hints.
- **Tracking:** Show workspace and project status. Add local projects to the manifest.
- **Planning & Execution:** Diagnose local readiness. Plan and apply safe missing project structure as empty placeholder folders.
- **Hydration:** Hydrate placeholder Git projects with normal `git clone`.
- **Syncing:** Push and pull the workspace manifest through a user-owned Git repository. Opt into a hosted manifest sync control-plane prototype with explicit endpoint/token configuration.
- **Setup:** Review detected dependency install/dev commands and run them only through an explicit setup command.
- **Secrets:** Store encrypted per-project env profiles with native `age` encryption. Generate local `.env` files with `0600` permissions.
- **Watching:** Keep workspace metadata fresh with an event-driven `devspace watch` mode.
- **FUSE:** Preview a FUSE-backed lazy workspace mount prototype without requiring FUSE for normal CLI workflows.

### Building from Source

The intended command name is `devspace`. The binary can be built from this repository:

```bash
go build -o bin/devspace ./cmd/devspace
```

During development, you can still run the command directly from source:

```bash
go test ./...
go run ./cmd/devspace --help
```

---

## Environment Variables

DevSpace respects the following environment variables to configure its runtime behavior:

| Variable | Description |
| -------- | ----------- |
| `DEVSPACE_HOME` | The root directory for DevSpace application data, configuration, and state. Defaults to `~/.devspace`. |
| `DEV_DROP_HOME` | Deprecated fallback alias for `DEVSPACE_HOME`. Maintained for backward compatibility. |
| `DEVSPACE_HOSTED_TOKEN` | Bearer token used for hosted sync authentication. Prefer this over the `--token` CLI flag to prevent the token from appearing in shell history or process listings. |

---

## Release Packaging

Releases are automated with GoReleaser: pushing a `v*` tag publishes a GitHub Release with prebuilt `devspace` archives for Linux and macOS (amd64/arm64), SHA256 checksums, and build-provenance attestation. Download archives from the [releases page](https://github.com/liatrio-forge/devdrop-capstone/releases) and verify them with `sha256sum -c` and `gh attestation verify`.

CI (`go test`, `go vet`, build) runs on every PR and push to `main`. The same gate is available locally:

```bash
make verify
```

See [`docs/operations/release.md`](docs/operations/release.md) for the full release process, consumer verification steps, install-from-source instructions, and the manual `make release` fallback.

---

## Capstone Artifacts

This repository is being prepared as a Liatrio Forge Module 5 capstone. See [`docs/capstone/README.md`](docs/capstone/README.md) for the capstone spec, proof checklist, case study, demo script, and playbook contribution. Open [`docs/capstone/index.html`](docs/capstone/index.html) for an interactive HTML reader generated from the repository Markdown.

---

## Supported Commands

Output is styled (color, headers, tables) when stdout is a terminal, and automatically falls back to plain text when piped, redirected, or when `NO_COLOR`/`CLICOLOR_FORCE=0` is set. Pass the persistent `--no-color` flag to force plain output on any command regardless of terminal capability. Commands that support `--json` (`status`, `doctor`, `workspace diff`, `mount --preview`, `plan`, `setup plan`) always emit clean JSON with no ANSI content.

### Core Workflow

#### `devspace init`

```bash
devspace init --workspace ~/code
```

Creates `~/.devspace/config.json`, `~/.devspace/state.json`, `~/.devspace/identity.txt`, and `<workspace>/.devspace/manifest.json`. The command is idempotent and does not rotate the machine ID or age identity on repeat runs.

#### `devspace scan`

```bash
devspace scan
```

Scans the configured workspace and updates the manifest/state with Git remote URL, current branch, last commit, dirty working tree status, `.env` file presence, and dependency/setup hints. Common build output directories (`node_modules/`, `dist/`, `build/`, etc.) are ignored by default.

#### `devspace watch`

```bash
devspace watch
devspace watch --debounce 3s
devspace watch --sync git
devspace watch --sync hosted
devspace watch --once
```

Runs a long-lived workspace watcher that debounces filesystem events and refreshes the same manifest/state metadata as `devspace scan`.

Default behavior is local-only. `--sync git` and `--sync hosted` explicitly push the refreshed manifest to the configured remote. Watch mode **never** pulls remote manifests, applies saved plans, hydrates repositories, installs dependencies, runs setup commands, or uploads secrets/source files.

#### `devspace ui`

```bash
devspace ui
devspace ui --no-watch
```

Opens a full-screen workspace dashboard with project hydration, dirty, branch, env, scan summary, and recent refresh events. The dashboard exposes only safe actions: scan, plan, apply-safe, and hydrate selected. Use `--no-watch` to disable live filesystem watching and refresh manually with `r`.

#### `devspace status`

```bash
devspace status
devspace status --json
devspace project status client-a-api
```

Shows tracked projects, hydrated projects, placeholders, dirty repos, missing env files, stale/missing projects, and last scan/sync timestamps. `--json` prints the same counts as a stable, machine-readable `WorkspaceStatusReport`.

#### `devspace doctor`

```bash
devspace doctor
devspace doctor --json
```

Checks local readiness without changing files or contacting hosted services. Reports config, workspace, manifest, Git, manifest remote/cache, saved plan, and tracked project path status. `--json` prints the full check list and hard-failure count.

#### `devspace mount`

```bash
devspace mount ~/devspace-view
devspace mount ~/devspace-view --preview
devspace mount ~/devspace-view --preview --json
```

Prototype read-only FUSE workspace view. Manifest project paths appear as mount entries before they are hydrated. See [`docs/architecture/fuse-lazy-mount.md`](docs/architecture/fuse-lazy-mount.md) for platform requirements.
For macOS smoke testing, use [`docs/operations/macos-fuse-run-playbook.md`](docs/operations/macos-fuse-run-playbook.md).

#### `devspace project add`

```bash
devspace project add work/client-a-api
```

Adds a relative workspace path to the manifest. Existing Git repositories are recorded as Git projects; otherwise tracked as local-only.

#### `devspace project hydrate`

```bash
devspace project hydrate client-a-api
```

Hydrates a placeholder Git project with normal `git clone`. Refuses to clone into non-empty directories.

#### `devspace plan` & `devspace apply`

```bash
devspace plan
devspace plan --json
devspace apply
```

Builds a deterministic plan of safe and skipped actions, saves it to `<workspace>/.devspace/last-plan.json`. `apply` executes the last saved plan only if the manifest hash still matches, executing only safe actions.

### Git-Backed Workspace Sync

#### `devspace workspace remote`

```bash
devspace workspace remote set <git-url-or-local-path>
devspace workspace remote create local ~/Projects/devspace-manifest.git
devspace workspace remote create github your-org/devspace-manifest --private
devspace workspace remote get
```

Stores the Git remote used for manifest sync in local config. The remote setting is not written into the workspace manifest.

#### `devspace workspace push` & `devspace workspace pull`

```bash
devspace workspace push
devspace workspace pull
```

Validates, caches, and pushes/pulls the `manifest.json` from the configured Git remote. It does **not** pull project repos, install dependencies, or overwrite project contents.

#### `devspace workspace reconcile`

```bash
devspace workspace reconcile            # review-first: writes a plan, changes nothing
devspace workspace reconcile --json     # machine-readable reconcile plan
devspace workspace reconcile --apply    # apply the merged manifest (backup + hash guard)
devspace workspace reconcile --force-local|--force-remote --apply
```

When local and remote manifests diverge (the push/pull "diverged, reconcile manually" dead end), `reconcile` performs a three-way, project-level merge against the last-synced base manifest. Non-conflicting changes (each side added/removed/changed different projects) merge automatically; a project changed differently on both sides is a **conflict** that is never auto-resolved — apply is blocked until you pass `--force-local` or `--force-remote`. The command is review-first: it writes the plan to `DEVSPACE_HOME/last-reconcile.json`, and `--apply` backs up the previous manifest to `DEVSPACE_HOME/manifest-backup.json` before writing, guarded by a manifest-hash check. Without a base snapshot (first run after upgrade), it falls back to a conservative two-way union where every same-project difference is a conflict.

#### `devspace workspace diff`

```bash
devspace workspace diff
devspace workspace diff --json
```

Localizes the remote manifest and reports projects that would be added, removed, or changed by a future pull. `--json` prints the same diff as a stable `ManifestDiff` document.

### Hosted Workspace Sync

Hosted sync is an opt-in control plane prototype separate from Git-backed sync.

```bash
devspace hosted serve --addr 127.0.0.1:8787 --store ~/.devspace/hosted-control-plane --token dev-token
devspace hosted config set http://127.0.0.1:8787 --token dev-token --workspace team-a
devspace hosted push
devspace hosted pull
devspace hosted reconcile [--json] [--apply] [--force-local|--force-remote]
```

`devspace hosted reconcile` resolves hosted version conflicts (HTTP 409) the same way as `workspace reconcile`: three-way merge against the base snapshot, review-first plan, backup + hash-guarded apply, and explicit force flags for genuine conflicts. On apply it pushes the merged manifest with version-conflict protection, then refreshes the local manifest and sync baseline.

The prototype server accepts `manifest.json` metadata via API. It **never** receives source files, dependency folders, `.env` files, or encrypted/plaintext secret payloads.
Prefer setting the `DEVSPACE_HOSTED_TOKEN` environment variable over using the `--token` flag for security.

### Secrets & Environment

#### `devspace env`

```bash
printf '%s\n' "$CLIENT_A_DATABASE_URL" | devspace env set client-a-api DATABASE_URL
devspace env list client-a-api
devspace env pull client-a-api
devspace env recipient export
devspace env recipient invite client-a-api teammate age1...
devspace env recipient revoke client-a-api teammate
```

Manages per-project environment variables using `age` encryption. `env pull` writes the local project `.env` with `0600` permissions. Profile sharing uses public `age` recipients.

#### `devspace setup`

```bash
devspace setup plan
devspace setup run client-a-api --yes
devspace setup run client-a-api --command dev --yes
devspace setup apply --yes
```

Reviews and executes dependency setup hints captured by `scan`.

---

## Example Workflows

### Local Workflow (No Network Required)

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
git -C "$remote_src" config user.name "DevSpace Demo"
git -C "$remote_src" config user.email "devspace@example.com"
git -C "$remote_src" commit --allow-empty -m "initial"
git clone --bare "$remote_src" "$remote_bare"

bin/devspace init --workspace "$workspace_a"
mkdir -p "$workspace_a/work/client-a-api"
git clone "$remote_bare" "$workspace_a/work/client-a-api"
bin/devspace scan

bin/devspace workspace remote create local "$manifest_remote"
bin/devspace workspace push
bin/devspace plan
bin/devspace apply
```

### Two-Machine Git Sync Workflow

```bash
workspace_b="$tmp/workspace-b"

export DEVSPACE_HOME="$tmp/home-b"
bin/devspace init --workspace "$workspace_b"
bin/devspace workspace remote set "$manifest_remote"
bin/devspace workspace pull
bin/devspace plan
bin/devspace apply
bin/devspace status
```

---

## Architecture & Safety

### Safety Guarantees

- Project paths in the manifest are relative to the workspace root. Absolute paths and parent-directory escapes are rejected.
- `plan` reports planned changes before writing anything. `apply` strictly enforces hash-matching from the last plan.
- `apply` creates missing directories only; it skips non-empty destinations.
- Hydration clones only into missing or empty directories.
- Env values are encrypted at rest with `age`. `env pull` writes `.env` with `0600` permissions.
- Manifest sync strips machine-local workspace paths and only shares `manifest.json`.

### Conflict Behavior

Manifest sync stops with a clear error when:

- No manifest remote is configured, or it does not exist yet.
- The manifest repo has uncommitted changes, or the remote branch is newer/diverged.
- Pull would overwrite local manifest changes.
- Hosted push/pull would overwrite manifest changes without a matching hosted version/hash baseline.

### What This Tool Will Not Do Without Permission

- Delete local projects, files, or directories.
- Overwrite existing project contents during apply or hydrate.
- Auto-pull, rebase, merge, or push project Git repositories.
- Auto-pull workspace manifests or auto-apply plans from watch mode.
- Install dependencies or run project setup commands during implicit background actions.
- Upload secrets, source code, or project files.

---

## Troubleshooting & Limitations

### Known Limitations

- Hosted manifest sync is a runnable prototype, not a managed deployment.
- Placeholder hydration uses full `git clone`; partial clone and sparse checkout are not implemented.
- Secret profile sharing uses public `age` recipients and local re-encryption (no OS keychain or identity provider integration).

### Troubleshooting Tips

- **`devspace` not found?** Build it with `go build -o bin/devspace ./cmd/devspace`.
- **Workspace remote not ready?** Create it first with `workspace remote create local` or `github`.
- **Wrong workspace?** Check `DEVSPACE_HOME` (or `DEV_DROP_HOME`) and `~/.devspace/config.json`.
- **Hydrate fails?** Confirm the project has a remote URL in `manifest.json` and `git clone <remote>` works.
- **Secrets missing?** Run `devspace env list <project>` to verify keys exist before pulling.

### Migration

On first run, an existing `~/.devdrop` application home is automatically migrated to `~/.devspace`. `DEV_DROP_HOME` still works as a deprecated alias for `DEVSPACE_HOME`.

---

## Roadmap

- Per-project conflict choices in `workspace reconcile` (global resolution and `--force-local` / `--force-remote` shipped).
- Hosted sync: grow the shipped prototype into a managed service.
- Daemon process management for running watch mode outside a terminal.
- FUSE lazy mount: grow the shipped prototype into a supported feature (macOS local proof pending).
- Managed team identity provider & OS keychain integration.
- Release-readiness checklist automation.

---

## Manifest Structure

See [`examples/manifest.json`](examples/manifest.json). The manifest is a versioned JSON file stored at `<workspace>/.devspace/manifest.json`. Project paths are always relative to the workspace root.
