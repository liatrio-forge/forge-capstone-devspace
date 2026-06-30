# DevDrop

DevDrop is a local-first "Dropbox for developers" CLI prototype. It keeps a
developer workspace structurally aligned across machines by tracking project
metadata, safe placeholder folders, Git remotes, setup hints, and encrypted env
profiles.

This MVP is intentionally local-only. It proves the workflow before adding hosted
sync, background daemons, or filesystem-level lazy loading.

## What Works

- Initialize a workspace and local machine identity.
- Generate a versioned workspace manifest.
- Scan existing projects for Git metadata, `.env` presence, dependency files,
  and setup hints.
- Show workspace and project status.
- Add local projects to the manifest.
- Recreate missing project structure as placeholders.
- Hydrate placeholder Git projects with normal `git clone`.
- Store encrypted per-project env profiles with native age encryption.
- Generate local `.env` files with `0600` permissions.

## Install For Development

```bash
go test ./...
go run ./cmd/devdrop --help
```

To build a local binary:

```bash
go build -o bin/devdrop ./cmd/devdrop
```

## Local Demo

This demo uses temporary directories and a local bare Git remote, so it does not
need network access.

```bash
tmp="$(mktemp -d)"
export DEV_DROP_HOME="$tmp/home"
workspace_a="$tmp/workspace-a"
workspace_b="$tmp/workspace-b"
remote_src="$tmp/remote-src"
remote_bare="$tmp/client-a-api.git"

mkdir -p "$remote_src"
git -C "$remote_src" init -b main
git -C "$remote_src" config user.email demo@example.com
git -C "$remote_src" config user.name "Demo User"
printf '# client-a-api\n' > "$remote_src/README.md"
git -C "$remote_src" add README.md
git -C "$remote_src" commit -m "initial"
git clone --bare "$remote_src" "$remote_bare"

go run ./cmd/devdrop init --workspace "$workspace_a"
mkdir -p "$workspace_a/work/client-a-api"
git clone "$remote_bare" "$workspace_a/work/client-a-api"
printf '{"scripts":{"dev":"vite"}}\n' > "$workspace_a/work/client-a-api/package.json"
go run ./cmd/devdrop workspace scan
printf 'postgres://demo\n' | go run ./cmd/devdrop env set client-a-api DATABASE_URL
go run ./cmd/devdrop env list client-a-api
go run ./cmd/devdrop env pull client-a-api
go run ./cmd/devdrop status
```

To simulate a second workspace with the same manifest:

```bash
go run ./cmd/devdrop init --workspace "$workspace_b"
mkdir -p "$workspace_b/.devdrop"
cp "$workspace_a/.devdrop/manifest.json" "$workspace_b/.devdrop/manifest.json"
go run ./cmd/devdrop workspace sync --dry-run
go run ./cmd/devdrop workspace sync
go run ./cmd/devdrop status
```

The second workspace now contains placeholder directories for tracked projects.
Run `devdrop project hydrate <project>` to clone a placeholder Git project when
the manifest includes its remote.

## Commands

### `devdrop init`

```bash
devdrop init --workspace ~/code
```

Creates:

- `~/.devdrop/config.json`
- `~/.devdrop/state.json`
- `~/.devdrop/identity.txt`
- `<workspace>/.devdrop/manifest.json`

The command is idempotent and does not rotate the machine ID or age identity on
repeat runs.

### `devdrop workspace scan`

```bash
devdrop workspace scan
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

### `devdrop status`

```bash
devdrop status
devdrop project status client-a-api
```

Shows tracked projects, hydrated projects, placeholders, dirty repos, missing env
files, stale/missing projects, and last scan/sync timestamps.

### `devdrop project add`

```bash
devdrop project add work/client-a-api
```

Adds a relative workspace path to the manifest. Existing Git repositories are
recorded as Git projects; otherwise the project is tracked as local-only.

### `devdrop workspace sync`

```bash
devdrop workspace sync --dry-run
devdrop workspace sync
```

Creates missing namespace folders and safe placeholder directories. It reports
path/remote conflicts and never deletes or overwrites local work.

### `devdrop project hydrate`

```bash
devdrop project hydrate client-a-api
```

Hydrates a placeholder Git project with normal `git clone`. The placeholder is
removed only after clone succeeds.

### `devdrop env`

```bash
printf 'postgres://local\n' | devdrop env set client-a-api DATABASE_URL
devdrop env list client-a-api
devdrop env pull client-a-api
```

`env set` reads from stdin when piped, or prompts for a hidden value when run in a
terminal. `env list` prints keys only, with values masked. `env pull` writes the
local project `.env` with `0600` permissions.

Encrypted profiles are stored under:

```text
<workspace>/.devdrop/secrets/<project-id>/<profile>.age
```

## Manifest

See [`examples/manifest.json`](examples/manifest.json).

The manifest is a versioned JSON file at:

```text
<workspace>/.devdrop/manifest.json
```

Project paths are always relative to the workspace root. Absolute paths and
parent-directory escapes are rejected.

## Deferred

These are deliberately not part of the local MVP:

- Git-backed manifest push/pull.
- Hosted API or cloud object storage.
- Background daemon and filesystem watchers.
- FUSE or virtual filesystem lazy loading.
- Git partial clone or sparse checkout.
- Team/shared secret profiles.
- OS keychain integration.
- Secret rotation.
- Editor settings, VS Code extensions, devcontainer, Nix, or mise/asdf sync.
- Dependency auto-install.
- Destructive project deletion or cleanup.

