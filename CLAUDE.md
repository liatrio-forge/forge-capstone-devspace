# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

DevSpace is a local-first "Dropbox for developers" CLI (Go). It keeps a developer
workspace structurally aligned across machines by tracking project metadata, safe
placeholder folders, Git remotes, setup hints, and encrypted env profiles. The
binary is `devspace`; the repo directory is still named `devdrop` (a rename is in
progress — see Migration below).

## Commands

```bash
make verify        # test + vet + lint + govulncheck + build — the local gate; use before pushing
make test          # go test ./...
make build         # builds bin/devspace (-trimpath)
make vulncheck     # govulncheck (also run as part of make verify)
make tui-verify    # Bun typecheck + tests for the devspace-tui companion (tui/)
go run ./cmd/devspace --help

# Run one test / package
go test ./internal/devspace -run TestName -v
```

There is a single Go module and effectively a single package worth testing
(`internal/devspace`). CI (`.github/workflows/ci.yml`) runs `go test`, `go vet`,
lint, govulncheck, and a build on every PR and push to `main`.

## Architecture

Everything lives in one package, `internal/devspace`; `cmd/devspace/main.go` is a
thin entrypoint that calls `NewRootCommand`. Commands are Cobra-based and wired in
`commands.go` — read that file first to find the handler for any subcommand, then
follow it into the domain function it calls.

### Three storage tiers (all JSON, no database)

1. **App home** (`~/.devspace/`, override with `DEVSPACE_HOME`): `config.json`
   (machine identity, workspace root, remotes, hosted-sync settings),
   `state.json` (per-project runtime state), `identity.txt` (age private key).
   Resolved by `appHome()` / `configPath()` / `statePath()` in `paths.go`.
2. **Workspace manifest** (`<workspace>/.devspace/manifest.json`): the shared,
   syncable source of truth — projects, machines, users, teams, access grants.
   The `Manifest`/`Project` types in `types.go` define the schema; bump
   `ManifestVersion` on schema changes.
3. **Secrets**: encrypted per-project env profiles under app home, using native
   `age` encryption (`secrets.go`). Generated `.env` files are written `0600`.

### Core data model (`types.go`)

`Config` and `State` are local-machine scoped. `Manifest` is the shared document.
The `devspace` workflow is a pipeline over these: **scan** (`ScanWorkspace`)
inspects the filesystem and updates manifest + state → **plan** (`BuildPlan`)
diffs desired manifest against reality and emits `PlanAction`s tagged with a
`Safety` level → **apply** (`ApplyLastPlan`) executes only safe actions (creating
empty placeholder folders, never deleting) → **hydrate** (`HydrateProject`) turns
a placeholder into a real checkout via `git clone`. Plans are persisted to
`last-plan.json` so `apply` acts on a reviewed plan. Most of this lives in
`workspace.go`.

### Sync — two independent backends

- **Git remote** (`workspace_sync.go`): push/pull the manifest through a
  user-owned Git repo (`devspace workspace push/pull`, `remote set/create`).
- **Hosted control-plane** (`hosted_sync.go`): opt-in HTTP prototype. The client
  side is configured via `devspace hosted config`; the server is
  `NewHostedSyncServer` exposed through `devspace hosted serve` and also shipped
  as a container image (`ghcr.io/liatrio-forge/devspace-hosted`, built by
  GoReleaser `ko`). The API is `GET/PUT /v1/workspaces/{workspace}/manifest` with
  bearer-token auth. The server is hardened for public exposure (constant-time
  auth, atomic PUT, rate limiting, HTTPS-only) — see `hardening_test.go` for the
  contract before changing server behavior.

### Other subsystems

- `watch.go` — event-driven `devspace watch` (fsnotify) that keeps state fresh.
- `mount.go` — FUSE-backed lazy-workspace mount *prototype*; guarded so normal
  CLI workflows never require FUSE (`go-fuse`).
- `setup.go` — detects and, only via the explicit `devspace setup` command, runs
  install/dev commands. Never auto-executes project commands.
- `doctor.go` — `devspace doctor` readiness diagnostics run before sync/apply.
- `ui.go` / `ui_server.go` / `tui/` — `devspace ui` launches the `devspace-tui`
  companion (OpenTUI/Bun/React app in `tui/`) when installed, falling back to
  the built-in Bubble Tea dashboard (`ui_model.go`; `--legacy` forces it). The
  companion spawns the hidden `devspace ui-server` subcommand and talks stdio
  NDJSON JSON-RPC (`ui_server.go`); both frontends reuse the same
  `dashboard*Cmd` domain closures in `ui_actions.go`. Protocol DTO changes must
  update `tui/src/protocol.ts` in lockstep.

## Invariants to preserve

- **Path safety**: any workspace-relative path from user input or the manifest
  must go through `safeWorkspacePath` (`paths.go`). It rejects absolute paths and
  `..` escapes *and* resolves symlinks to catch link-based escapes. Do not join
  workspace paths by hand.
- **Non-destructive by default**: `plan`/`apply` only create safe placeholders;
  they must never delete or overwrite user data. Preserve the `Safety` tagging.
- **Idempotency**: `init` and scan-like operations must be safe to re-run and must
  not rotate the machine ID or age identity.

## Migration: devdrop → devspace

The project was renamed from `devdrop`. Compatibility is deliberate and must be
kept until the transition window closes:

- App home migrates `~/.devdrop` → `~/.devspace` automatically via
  `migrateLegacyHome` (`migrate.go`), unless `DEVSPACE_HOME`/`DEV_DROP_HOME` is
  set.
- Workspace metadata dir: `workspaceDevdrop()` prefers `.devspace`, falls back to
  reading an existing `.devdrop`, and resolves reads and writes to the **same**
  directory so a workspace never straddles both.
- Both `DEVSPACE_HOME` (current) and `DEV_DROP_HOME` (legacy) env overrides are
  honored.

When touching path/home resolution, run the migration and hardening test suites
(`migrate_test.go`, `hardening_test.go`).

## Testing conventions

Tests isolate state by setting `DEVSPACE_HOME` (via `t.Setenv`) to a temp dir and
operating on a temp workspace, so they never touch the real `~/.devspace`. Follow
that pattern for any new test that reads or writes config/state/manifest.

## Capstone

This repo is a Liatrio Forge Module 5 capstone. Supporting docs (spec, proof
checklist, case study, demo script) live in `docs/capstone/`; `docs/` also holds
`docs/operations/release.md` and design notes like `docs/architecture/fuse-lazy-mount.md`.
