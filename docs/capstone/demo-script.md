# Demo Script

This script is designed for the final Module 5 recording. The executable version
is checked in as `scripts/demo-check.sh`; it uses temporary directories and local
bare Git remotes so the demo works without network access.

## Talk Track

DevSpace solves workspace recovery without syncing everything. The demo shows one
machine publishing safe workspace metadata, a second machine pulling that
metadata, creating placeholders, updating a Git project, and materializing an
encrypted env value only when requested.

## Run The Demo Check

Start from the repo root:

```bash
scripts/demo-check.sh
```

For proof capture, keep the generated workspaces, local bare remotes, pushed
manifest, and summary file outside the repo with an explicit output directory:

```bash
proof_dir="$(mktemp -d /tmp/devspace-demo-proof.XXXXXX)"
scripts/demo-check.sh --output-dir "$proof_dir"
```

The script builds `devspace`, creates two temporary machine homes, creates a
local bare project remote and a local bare manifest remote, runs the full
workflow, asserts the generated `.env` mode is `0600`, and prints
`DevSpace demo-check passed.` on success.

## Machine A: Publish Workspace Metadata

Narration:

- The workspace contains a real Git project on machine A.
- `scan` captures metadata and setup hints.
- `sync push` sends only validated `manifest.json` metadata to the manifest remote; source, dependencies, env files, identities, and secret payloads stay local.

## Machine B: Pull, Plan, Apply, Update

Narration:

- Pull localizes the workspace root for machine B.
- `plan` shows safe placeholder actions before mutation.
- `apply` creates structure only.
- `project update` uses normal Git clone for placeholders and refuses unsafe destinations.

## Encrypted Env Profile

Narration:

- Env values are stored encrypted under the workspace `.devspace/secrets`
  directory.
- `env list` masks values.
- `env write` writes the local `.env` file only on explicit request and does not print decrypted values.
- The generated `.env` has `0600` permissions.
- `scripts/demo-check.sh` enforces the `0600` assertion automatically.

## Close

Show the release-readiness file and proof checklist:

```bash
sed -n '1,120p' docs/operations/release-readiness.md
sed -n '1,160p' docs/capstone/proof-artifacts.md
```

Show the remote-agent delivery case study:

```bash
sed -n '1,220p' docs/capstone/remote-agent-case-study.md
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

DevSpace is intentionally conservative. It does not delete projects, auto-run
setup commands, sync source code, or upload secrets. The capstone value is the
safe recovery workflow, the evidence that destructive edges are guarded, and the
agent-delivery process that made the work auditable.
