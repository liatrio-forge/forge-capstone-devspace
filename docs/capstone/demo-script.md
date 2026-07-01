# Demo Script

This script is designed for the final Module 5 recording. It uses temporary
directories and local bare Git remotes so the demo works without network access.

## Talk Track

DevDrop solves workspace recovery without syncing everything. The demo shows one
machine publishing safe workspace metadata, a second machine pulling that
metadata, creating placeholders, hydrating a Git project, and materializing an
encrypted env value only when requested.

## Setup

Start from the repo root:

```bash
go build -o .tmp/devspace ./cmd/devdrop

tmp="$(mktemp -d)"
workspace_a="$tmp/workspace-a"
workspace_b="$tmp/workspace-b"
remote_src="$tmp/remote-src"
project_remote="$tmp/client-a-api.git"
manifest_remote="$tmp/manifest-sync.git"
```

Create a demo project remote:

```bash
mkdir -p "$remote_src"
git -C "$remote_src" init -b main
git -C "$remote_src" config user.email demo@example.com
git -C "$remote_src" config user.name "Demo User"
printf '# client-a-api\n' > "$remote_src/README.md"
git -C "$remote_src" add README.md
git -C "$remote_src" commit -m "initial"
git clone --bare "$remote_src" "$project_remote"
```

## Machine A: Publish Workspace Metadata

```bash
export DEV_DROP_HOME="$tmp/home-a"
.tmp/devspace init --workspace "$workspace_a"
mkdir -p "$workspace_a/work"
git clone "$project_remote" "$workspace_a/work/client-a-api"
printf '{"scripts":{"dev":"vite"}}\n' > "$workspace_a/work/client-a-api/package.json"
.tmp/devspace scan
.tmp/devspace workspace remote create local "$manifest_remote"
.tmp/devspace workspace push
.tmp/devspace status
```

Narration:

- The workspace contains a real Git project on machine A.
- `scan` captures metadata and setup hints.
- `workspace push` sends only `manifest.json` to the manifest remote.

## Machine B: Pull, Plan, Apply, Hydrate

```bash
export DEV_DROP_HOME="$tmp/home-b"
.tmp/devspace init --workspace "$workspace_b"
.tmp/devspace workspace remote set "$manifest_remote"
.tmp/devspace workspace pull
.tmp/devspace plan
.tmp/devspace apply
.tmp/devspace status
.tmp/devspace project hydrate client-a-api
.tmp/devspace status
```

Narration:

- Pull localizes the workspace root for machine B.
- `plan` shows safe placeholder actions before mutation.
- `apply` creates structure only.
- Hydration uses normal Git clone and refuses unsafe destinations.

## Encrypted Env Profile

```bash
printf 'postgres://demo\n' | .tmp/devspace env set client-a-api DATABASE_URL
.tmp/devspace env list client-a-api
.tmp/devspace env pull client-a-api
ls -l "$workspace_b/work/client-a-api/.env"
```

Narration:

- Env values are stored encrypted under the workspace `.devdrop/secrets`
  directory.
- `env list` masks values.
- `env pull` writes the local `.env` file only on explicit request.
- The generated `.env` has `0600` permissions.

## Close

Show the release-readiness file and proof checklist:

```bash
sed -n '1,120p' docs/release-readiness.md
sed -n '1,160p' docs/capstone/proof-artifacts.md
```

Show the remote-agent delivery case study:

```bash
sed -n '1,220p' docs/capstone/remote-agent-case-study.md
sed -n '1,220p' ops/wave-ship/devdrop-mvp.args.json
sed -n '1,220p' ops/wave-ship/devdrop-capstone.args.json
```

Narration:

- The product was not built as one large ambiguous prompt.
- The MVP was decomposed into cards with dependencies and explicit non-goals.
- `wave-ship` can dispatch those cards through isolated remote workers with a
  concurrency cap.
- Serialized merge keeps parallel agent work from racing on the base branch.
- The same workflow now has a capstone stretch wave for release packaging,
  demo verification, diagnostics, manifest diff preview, and final evidence.

Closing line:

DevDrop is intentionally conservative. It does not delete projects, auto-run
setup commands, sync source code, or upload secrets. The capstone value is the
safe recovery workflow, the evidence that destructive edges are guarded, and the
agent-delivery process that made the work auditable.
