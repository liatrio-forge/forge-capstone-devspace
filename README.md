# DevSpace

> **A local-first "Dropbox for developers" CLI prototype.**

A developer working across multiple machines and many repos knows the drill: the
workspace layout drifts between laptops, clone URLs and remotes live in whoever
set them up last, setup steps rot in a README nobody re-reads, and `.env` files
end up scattered, unsynced, or emailed around as plaintext. Copying the whole
workspace isn't a fix either — it drags along secrets, client code, generated
build output, and stale dependency folders. DevSpace keeps the *structure* of a
workspace in sync across machines — project metadata, safe placeholder folders,
Git remotes, setup hints, and encrypted env profiles — without ever syncing file
contents, dependency trees, or plaintext secrets.

---

## 📖 Table of Contents

- [Overview](#overview)
  - [Current MVP Status](#current-mvp-status)
  - [Building from Source](#building-from-source)
- [Topology](#topology)
- [Environment Variables](#environment-variables)
- [Release Packaging](#release-packaging)
- [Capstone Artifacts](#capstone-artifacts)
- [Supported Commands](#supported-commands)
  - [Capture a Workspace](#capture-a-workspace)
  - [Restore a Workspace](#restore-a-workspace)
  - [Maintain a Workspace](#maintain-a-workspace)
  - [Troubleshoot Safely](#troubleshoot-safely)
  - [Command Reference](#command-reference)
  - [Pre-1.0 Command Migration](#pre-10-command-migration)
- [Walkthrough: Two Machines, One Workspace](#walkthrough-two-machines-one-workspace)
- [Architecture & Safety](#architecture--safety)
  - [Safety Guarantees](#safety-guarantees)
  - [Access Roles](#access-roles)
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
- **Tracking:** Show workspace and project status. Track local projects in the manifest.
- **Planning & Execution:** Diagnose local readiness. Plan and apply safe missing project structure as empty placeholder folders.
- **Project updates:** Materialize placeholder Git projects with normal `git clone` and fast-forward eligible clean checkouts.
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

## Topology

A workspace is a root directory containing tracked projects and one manifest.
Machines are peers that all sync against that same manifest:

```text
<workspace root>/
├── .devspace/
│   └── manifest.json        # shared, syncable source of truth
├── work/
│   └── client-a-api/        # project path relative to workspace root,
│       ...                  #   tracked in the manifest with remote,
│                             #   default branch, setup hints, env profiles
└── ...                      # more tracked project directories

machine "mac-mini" (id, os, arch, own workspaceRoot) ─┐
machine "thinkpad"  (id, os, arch, own workspaceRoot) ─┼─ peers registered in the same manifest.json
machine "..."                                          ┘
```

Every project entry in the manifest records its relative `path`, `remote`,
`defaultBranch`, `setup` hints, and `envProfiles` — see
[examples/manifest.json](examples/manifest.json).

State is split into three tiers, all plain JSON + `age` files, no database:

| Tier               | Location                                          | Scope                                    |
| ------------------ | ------------------------------------------------- | ---------------------------------------- |
| App home           | `~/.devspace/`                                    | Per-machine: config, state, age identity |
| Workspace manifest | `<workspace>/.devspace/manifest.json`             | Shared, synced across machines           |
| Secrets            | Encrypted per-project env profiles under app home | Per-machine, per-project                 |

This hierarchy is why the CLI is shaped the way it is: `devspace sync …`
commands operate on the shared manifest and its Git remote; explicit
`devspace project list|track|untrack|update` commands manage manifest membership
and project checkouts; `scan`, `plan`, `apply`, `status`, and `doctor` act on the
whole workspace.

---

## Environment Variables

DevSpace respects the following environment variables to configure its runtime behavior:

| Variable                | Description                                                                                                                                                         |
| ----------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `DEVSPACE_HOME`         | The root directory for DevSpace application data, configuration, and state. Defaults to `~/.devspace`.                                                              |
| `DEV_DROP_HOME`         | Deprecated fallback alias for `DEVSPACE_HOME`. Maintained for backward compatibility.                                                                               |
| `DEVSPACE_HOSTED_TOKEN` | Bearer token used for hosted sync authentication. Prefer this over the `--token` CLI flag to prevent the token from appearing in shell history or process listings. |

---

## Release Packaging

Releases are automated with GoReleaser: pushing a `v*` tag publishes a GitHub Release with Linux and macOS archives (amd64/arm64), SHA256 checksums, and build-provenance attestation. Each archive contains the matching `devspace` CLI and `devspace-tui` companion. Download an archive from the [releases page](https://github.com/liatrio-forge/forge-capstone-devspace/releases), verify it with `sha256sum -c` and `gh attestation verify`, extract both executables into the same directory, and run `devspace ui`. The CLI discovers the adjacent companion without a separate installer command.

CI runs `go test`, `go vet`, build, `make tui-verify` for the `devspace-tui` companion, and a `mount-integration` FUSE job on every PR and push to `main`. The same core gate is available locally:

```bash
make verify
```

`make verify` runs tests, vet, lint, govulncheck, and the build — the local CI gate.

See [docs/operations/release.md](docs/operations/release.md) for the full release process, consumer verification steps, install-from-source instructions, and the local `make snapshot` dry-run.

---

## Capstone Artifacts

This repository is being prepared as a Liatrio Forge Module 5 capstone. See [docs/capstone/README.md](docs/capstone/README.md) for the capstone spec, proof checklist, case study, demo script, and playbook contribution. Open [docs/capstone/index.html](docs/capstone/index.html) for an interactive HTML reader generated from the repository Markdown.

---

## Supported Commands

Output is styled when stdout is a terminal and automatically becomes plain text
when piped, redirected, or disabled with `NO_COLOR`, `CLICOLOR_FORCE=0`, or
`--no-color`. Supported `--json` commands emit ANSI-free output.

### Capture a Workspace

```bash
devspace init --workspace ~/code
devspace scan
devspace project list
devspace sync remote create github your-org/devspace-manifest --private
devspace sync push
```

`init` creates machine-local config and identity; `scan` records workspace
structure and Git/setup metadata. `sync push` publishes only validated manifest
metadata. It never uploads repositories, dependency folders, plaintext `.env`
files, age identities, or encrypted secret payloads.

### Restore a Workspace

```bash
devspace init --workspace ~/code
devspace sync remote set <git-url-or-local-path>
devspace sync pull
devspace plan
devspace apply
devspace project update --all
devspace env write client-a-api
devspace setup show
devspace setup run --all --dry-run
```

Restore is intentionally staged. Pull retrieves metadata only; `plan` and
`apply` create safe empty placeholders; `project update --all` explicitly
clones or fast-forwards eligible repositories. Env materialization and setup
execution are separate explicit commands. DevSpace never performs them as a
side effect of sync, apply, watch, status, or UI startup.

### Maintain a Workspace

```bash
devspace scan
devspace watch --sync off
devspace status --verbose
devspace status client-a-api --json
devspace project list --json
devspace project track work/client-a-api
devspace project untrack client-a-api
devspace project update client-a-api
devspace sync diff
devspace sync reconcile
```

`project untrack` removes manifest/access/state references but retains local
files and encrypted profiles. `sync reconcile` is review-first; applying a
merge still requires `--apply`, and conflicts require explicit directional
force flags.

### Troubleshoot Safely

```bash
devspace doctor
devspace doctor --json
devspace status client-a-api
devspace sync diff --json
devspace sync reconcile --json
devspace experimental mount ~/devspace-view --preview
```

Diagnostics and previews do not run setup, retrieve project source, or write
plaintext secrets. Prototype server and mount operations live under the
clearly labeled `experimental` group.

### Command Reference

- `init`, `scan`, `plan`, and `apply` capture local structure and create only
  reviewed safe placeholders.
- `status [<project>] [--verbose] [--json]` owns workspace, detailed overview,
  and project-specific status.
- `sync remote|push|pull|diff|reconcile` owns Git-backed manifest transport and
  conflict review.
- `project list|track|untrack|update` owns explicit inventory and repository
  maintenance.
- `env set|list|write|recipient` manages age-encrypted profiles. `env write`
  creates the selected local `.env` with `0600` permissions without printing
  decrypted values.
- `setup show` reviews captured hints. `setup run <project>` and
  `setup run --all` execute them only after explicit invocation and retain
  confirmation/dry-run safeguards.
- `hosted config|push|pull|reconcile` remains the opt-in hosted client. The
  prototype server is `experimental hosted serve` and defaults to loopback.
- `ui` is the sole visible dashboard command. Release archives include the
  matching adjacent `devspace-tui`; `--legacy` forces the built-in fallback.
- `watch` defaults to local-only metadata refresh. Opt-in sync never pulls,
  applies, updates repositories, writes env files, or runs setup.
- `experimental mount` is the FUSE prototype; `--preview` remains FUSE-free.
- `doctor` reports readiness without changing workspace files.
- `devspace --version` prints the CLI version; there is no visible version
  subcommand.

Hosted sync accepts manifest metadata only. It never receives source files,
dependency folders, `.env` files, identities, or encrypted/plaintext secret
payloads. Prefer `DEVSPACE_HOSTED_TOKEN` over a token flag.

### Pre-1.0 Command Migration

This is an intentional pre-1.0 clean break. Removed paths have no aliases or
hidden compatibility wrappers.

<!-- command-surface-migration:start -->
| Before | Use now |
| --- | --- |
| `devspace workspace` | `devspace status --verbose` |
| `devspace workspace scan` | `devspace scan` |
| `devspace workspace sync` | `devspace plan`, then `devspace apply` |
| `devspace workspace remote|push|pull|diff|reconcile` | `devspace sync remote|push|pull|diff|reconcile` |
| bare `devspace project` | `devspace project list` |
| `devspace project add` | `devspace project track` |
| `devspace project remove` | `devspace project untrack` |
| `devspace project hydrate` | `devspace project update` |
| `devspace project status <project>` | `devspace status <project>` |
| `devspace env pull` | `devspace env write` |
| `devspace setup plan` | `devspace setup show` |
| `devspace setup apply` | `devspace setup run --all` |
| `devspace hosted serve` | `devspace experimental hosted serve` |
| `devspace mount` | `devspace experimental mount` |
| `devspace version` | `devspace --version` |
<!-- command-surface-migration:end -->

---

## Walkthrough: Two Machines, One Workspace

New laptop, twenty repos, half-remembered clone URLs — this is the workflow
DevSpace exists for. Machine A has a working set of projects and pushes the
*shape* of its workspace through a Git remote it owns; Machine B pulls that
shape and rebuilds it, safely, without a single byte of source code moving
until it explicitly asks for one project.

### Machine A (the laptop)

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
```

`init` writes the machine identity, age key, and an empty manifest; `scan`
walks the workspace and records `client-a-api` as a tracked Git project with
its remote, branch, and dirty state. `init`/`scan` are idempotent — safe to
re-run, they never rotate the machine ID or age key.

```bash
bin/devspace sync remote create local "$manifest_remote"
bin/devspace sync push
```

This creates a bare Git repo to sync the manifest through (a real setup would
use `sync remote create github <owner/repo> --private` instead) and
pushes `manifest.json` to it — a single committed JSON file, not a DevSpace
server.

```bash
bin/devspace plan
bin/devspace apply
```

`plan` diffs the manifest against what's on disk locally; since Machine A
already has `client-a-api` checked out, there's nothing unsafe to create here.

### Machine B (the new machine)

```bash
workspace_b="$tmp/workspace-b"

export DEVSPACE_HOME="$tmp/home-b"
bin/devspace init --workspace "$workspace_b"
bin/devspace sync remote set "$manifest_remote"
bin/devspace sync pull
```

A fresh `DEVSPACE_HOME` and an empty workspace stand in for a brand-new
machine. Pointing it at the same manifest remote and pulling brings over the
*shape* of the workspace — no project code yet, just the plan of what should
exist.

```bash
bin/devspace plan
bin/devspace apply
```

`plan` shows one `SAFE` action — `PLACEHOLDER client-a-api` — and `apply`
creates only that empty directory. Nothing is cloned, nothing is deleted, and
apply re-checks the destination is still empty before writing.

```bash
bin/devspace project update client-a-api
bin/devspace status
```

`project update` turns the placeholder into a real checkout with a normal `git clone` (refusing any non-empty destination), so Machine B now has an actual
`client-a-api` repo, not just a folder. `status` confirms it: one project
tracked, one hydrated, zero placeholders left.

The core loop is **scan → plan → apply → project update**, safe at every step. See
[docs/demos/capstone-runbook.md](docs/demos/capstone-runbook.md) for the
fully narrated demo, and the [demo index](docs/demos/README.md) for recorded
GIFs covering the full command surface (sync, reconcile, secrets, setup,
watch, and more).

---

## Architecture & Safety

### Safety Guarantees

- Project paths in the manifest are relative to the workspace root. Absolute paths and parent-directory escapes are rejected.
- `plan` reports planned changes before writing anything. `apply` strictly enforces hash-matching from the last plan.
- `apply` creates missing directories only; it skips non-empty destinations.
- Hydration clones only into missing or empty directories.
- Env values are encrypted at rest with `age`. `env write` writes `.env` with `0600` permissions without printing decrypted values.
- Manifest sync strips machine-local workspace paths and only shares `manifest.json`.

### Access Roles

Access roles are advisory metadata today. DevSpace records owners, maintainers, developers, and viewers to help teams describe intended responsibility, but the CLI does not refuse commands or change exit codes based on these roles. Selected shared-mutation commands print warning-only advisory messages when the local effective role falls outside the recommended boundary.

Encrypted env access is controlled by age recipients, not by the role field. Inviting a recipient records access metadata and includes that recipient in future encrypted writes; revocation removes a recipient from future encrypted writes but cannot delete copied `.env` files or values already decrypted.

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

- `devspace` **not found?** Build it with `go build -o bin/devspace ./cmd/devspace`.
- **Manifest remote not ready?** Create it first with `sync remote create local` or `github`.
- **Wrong workspace?** Check `DEVSPACE_HOME` (or `DEV_DROP_HOME`) and `~/.devspace/config.json`.
- **Project update fails?** Confirm the project has a remote URL in `manifest.json` and `git clone <remote>` works.
- **Secrets missing?** Run `devspace env list <project>` to verify keys exist before writing the local `.env`.

### Migration

On first run, an existing `~/.devdrop` application home is automatically migrated to `~/.devspace`. `DEV_DROP_HOME` still works as a deprecated alias for `DEVSPACE_HOME`.

---

## Roadmap

- Hosted sync: grow the shipped prototype into a managed service.
- Daemon process management for running watch mode outside a terminal.
- FUSE lazy mount: grow the shipped prototype into a supported feature.
- Managed team identity provider & OS keychain integration.
- Release-readiness checklist automation.

---

## Manifest Structure

See [examples/manifest.json](examples/manifest.json). The manifest is a versioned JSON file stored at `<workspace>/.devspace/manifest.json`. Project paths are always relative to the workspace root. User, team, team-member, and project-access role fields are advisory metadata for intended responsibility; they are not a command-permission system.
