# Release Readiness

## What Was Tested

- Full Go unit/regression suite with `go test ./...`.
- Static checks with `go vet ./...`.
- CLI build with `go build -o .tmp/devspace ./cmd/devspace`.
- Top-level command help for the `devspace` command surface.
- Local two-machine simulation using temporary directories and a local bare Git remote.
- Git-backed manifest push/pull using local bare Git remotes only.
- Safety cases for path traversal, invalid JSON, non-empty destination folders, dirty repos, and missing Git.

## What Passed

- Repeated `init` preserves machine identity, existing config, existing manifest projects, and age identity.
- Workspace scan ignores dependency folders and does not recurse into nested Git repos inside a parent repo.
- Manifest path validation rejects absolute paths and `..` escapes.
- Manifest writes create `.bak` backups before replacing existing files.
- `workspace remote set/get` stores manifest remote configuration outside the manifest.
- `workspace remote create local` creates a local bare Git manifest remote and sets it.
- `workspace remote create github` has an explicit GitHub CLI path for creating hosted manifest repos.
- `workspace push` clones the manifest repo cache, writes only `manifest.json`, commits changed manifests, pushes to the configured remote, and is idempotent with no changes.
- `workspace pull` validates the remote manifest before replacement, localizes the workspace root, creates a manifest backup, and does not run apply automatically.
- Git-backed manifest pull works into a second workspace, after which plan/apply recreates placeholder folder structure.
- Hydration after Git-backed manifest pull works against local bare Git project remotes.
- `plan` creates safe/skip actions and `plan --json` returns structured JSON.
- `apply` uses the saved plan and rejects manifest drift.
- `apply` creates only safe missing directories and skips non-empty destinations.
- Hydration works against a local bare Git repository with no network access.
- Hydration refuses missing remotes and non-empty destination folders.
- Dirty repos are detected and listed as skipped.
- Workspace paths and project paths with spaces work.
- Secret values remain encrypted at rest and masked in list output.
- Manifest sync refuses invalid JSON, path traversal, dirty manifest repo state, and local unpushed manifest changes.
- Missing manifest remotes get a purpose-built recovery message with local and GitHub create commands.

## What Failed

- Initial audit found a temporary compile break from partial path-hardening edits.
- The original hydrate path deleted placeholder directories before cloning.
- The original sync path recomputed actions during apply and had no saved plan hash.
- The original docs still described `workspace sync` as the primary flow.

## What Was Fixed

- Replaced direct placeholder marker deletion with empty-directory placeholders.
- Added a saved plan file with manifest hash validation.
- Added top-level `scan`, `plan`, and `apply` commands for the requested workflow.
- Kept `workspace sync` only as a deprecated compatibility alias.
- Added atomic JSON writes with backups.
- Centralized workspace-relative path validation and used it at mutating call sites.
- Improved Git clone errors with remote and next-step guidance.
- Added regression tests for the hardening requirements and local two-machine simulation.
- Added Git-backed manifest sync commands and regression tests for local bare remotes, conflict handling, backups, paths with spaces, plan/apply after pull, and hydrate after pull.
- Updated README command examples, safety guarantees, troubleshooting, and roadmap.

## Known Limitations

- Hosted sync, daemon/watch mode, FUSE lazy mounting, managed team identity, and explicit dependency install are shipped as prototypes (capstone frontier work), not part of the completed local-first MVP baseline.
- No source-code syncing (manifest exchange between machines in the MVP uses user-owned Git remotes only). There is no partial clone or sparse checkout.
- Secret profile sharing uses explicit age recipients only; there is no OS keychain integration, remote backup, managed identity provider, or guaranteed clawback after a recipient has copied or decrypted material.
- Dependency/setup commands are detected only as hints and are never executed automatically.

## Remaining Risks

- Plan/apply is intentionally conservative and may require manual cleanup or explicit future flags for advanced cases.
- Manifest sync has no force flag or merge UI; conflicts intentionally require manual reconciliation.
- Git inspection still avoids mutating repos, so stale/outdated remote commit detection remains shallow.
- Encrypted `.env` generation overwrites the target `.env` only when explicitly requested via `env pull`.

## Recommended Next Feature

Manifest conflict reconciliation and clearer multi-machine history.
