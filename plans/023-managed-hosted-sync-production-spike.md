# Plan 023: Define the managed hosted sync production contract

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report -- do not improvise. When done, update the status row for this plan
> in `plans/README.md`.
>
> **Drift check (run first)**: `git diff --stat cedcbc7..HEAD -- README.md ARCHITECTURE.md docs/operations/release-readiness.md internal/devspace/hosted_sync.go .github/workflows/release.yml plans/README.md`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against live code before proceeding; on mismatch,
> treat it as a STOP condition.

## Status

- **Priority**: P2
- **Effort**: M
- **Risk**: LOW
- **Depends on**: plans/022-reconcile-docs-with-shipped-state.md
- **Category**: direction
- **Planned at**: commit `cedcbc7`, 2026-07-08

## Why this matters

DevSpace has outgrown a pure capstone prototype, but the repo still mixes
current production-grade pieces with prototype language and a single-token
hosted sync server. The next useful step is not to build a whole SaaS surface
in one pass. It is to write the production contract for managed hosted sync:
who owns a workspace, how auth works, what data is stored, what operations are
observable, and which current CLI guarantees must remain true.

This is a design/spike plan. It should produce a concrete architecture and
rollout document that future implementation plans can split into small, safe
slices.

## Current state

Relevant files:

- `README.md` describes DevSpace as a local-first prototype and calls hosted
  sync a prototype:

```md
# README.md:3
> **A local-first "Dropbox for developers" CLI prototype.**
```

```md
# README.md:266-279
Hosted sync is an opt-in control plane prototype separate from Git-backed sync.
...
The prototype server accepts `manifest.json` metadata via API. It **never**
receives source files, dependency folders, `.env` files, or encrypted/plaintext
secret payloads.
```

- `README.md` and release-readiness docs name the managed-service gap:

```md
# README.md:398-399
- Hosted manifest sync is a runnable prototype, not a managed deployment.
- Placeholder hydration uses full `git clone`; partial clone and sparse checkout are not implemented.
```

```md
# docs/operations/release-readiness.md:57-62
- Hosted sync, daemon/watch mode, FUSE lazy mounting, managed team identity, and
  explicit dependency install are shipped as prototypes (capstone frontier work),
  not part of the completed local-first MVP baseline.
```

- `internal/devspace/hosted_sync.go` currently has a single bearer token for
  the server and file-backed workspace storage:

```go
// internal/devspace/hosted_sync.go:524-532
func NewHostedSyncServer(opts HostedSyncServerOptions) (http.Handler, error) {
    storeDir := strings.TrimSpace(opts.StoreDir)
    if storeDir == "" {
        return nil, fmt.Errorf("hosted sync store directory is required")
    }
    token := strings.TrimSpace(opts.Token)
    if token == "" {
        return nil, fmt.Errorf("hosted sync auth token is required")
    }
```

```go
// internal/devspace/hosted_sync.go:742-755
func (s *hostedSyncServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    if r.URL.Path == "/healthz" {
        w.WriteHeader(http.StatusOK)
        _, _ = w.Write([]byte("ok"))
        return
    }
    if !s.allowRequest(r) {
        http.Error(w, "too many requests\n", http.StatusTooManyRequests)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    expected := "Bearer " + s.token
    if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte(expected)) != 1 {
```

```go
// internal/devspace/hosted_sync.go:847-852
func (s *hostedSyncServer) save(envelope hostedManifestEnvelope) error {
    return writeJSON(s.path(envelope.Workspace), envelope, 0o600)
}

func (s *hostedSyncServer) path(workspace string) string {
    return filepath.Join(s.storeDir, workspace+".json")
}
```

- `.github/workflows/release.yml` already has a gated Railway deployment path
  for the hosted server image:

```yaml
# .github/workflows/release.yml:63-83
# Gated live deploy: waits for manual approval on the `production` environment
# before pointing Railway at the freshly published image. Stable tags only.
deploy-railway:
  needs: goreleaser
  if: ${{ !contains(github.ref_name, '-') }}
```

Repo conventions to match:

- Product/architecture work lives under `docs/architecture/` and
  `docs/operations/`; feature specs live under `docs/specs/<NN>-spec-*`.
- Keep safety language explicit: manifest sync must not upload source files,
  dependency folders, `.env` files, or secret payloads.
- Keep implementation follow-ups small. Do not turn this plan into a large
  hosted-service rewrite.

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| Go tests | `go test ./... -count=1` | exit 0 |
| TUI tests | `cd tui && bun test` | all tests pass |
| TUI typecheck | `cd tui && bun run typecheck` | exit 0, no TypeScript errors |
| Release config | `goreleaser check` | `1 configuration file(s) validated` |
| Docs grep | `rg -n "managed hosted|workspace owner|tenant|audit|backup|token" docs README.md ARCHITECTURE.md plans/README.md` | shows the new production contract terms |

## Scope

**In scope**:

- `docs/architecture/hosted-sync-production.md` (create)
- `README.md`
- `ARCHITECTURE.md`
- `docs/operations/release-readiness.md`
- `plans/README.md`

**Out of scope**:

- Any source code under `internal/`, `cmd/`, or `tui/`.
- Changing hosted API behavior.
- Adding a database, auth provider, web UI, billing, or deployment secrets.
- Publishing GitHub issues or opening a PR.

## Git workflow

- Branch: `advisor/023-hosted-sync-production-contract`
- Commit message style: conventional commit, e.g.
  `docs: define hosted sync production contract`.
- Do not push or open a PR unless instructed.

## Steps

### Step 1: Create the production contract document

Create `docs/architecture/hosted-sync-production.md` with these sections:

- `Production goal`: managed manifest sync for teams while preserving the
  local-first CLI and "metadata only" guarantee.
- `Non-goals`: source-code sync, secret sync, dependency folder sync, automatic
  project Git operations, and implicit setup execution.
- `Current prototype`: summarize the single bearer token, file-backed JSON
  storage, version/hash envelope, rate limiting, and Railway deployment path.
- `Production requirements`: workspace ownership, per-user auth, workspace
  membership/roles, token rotation, audit log, encrypted-at-rest metadata,
  backups/restore, service SLO, `/healthz` plus readiness, and migration from
  file-backed storage.
- `Small implementation slices`: split future work into independent plans:
  auth model, persistence abstraction, audit log, backup/restore, service
  operations, CLI config migration, and production deploy smoke.
- `Open decisions`: auth provider, data store, tenant naming, retention policy,
  public/private deployment model, and support boundary.

Keep it concise. This is a contract and backlog map, not a full design novel.

**Verify**: `test -s docs/architecture/hosted-sync-production.md` -> exit 0.

### Step 2: Wire the contract into existing docs

Update:

- `README.md`: change the hosted-sync roadmap item to point at
  `docs/architecture/hosted-sync-production.md`.
- `ARCHITECTURE.md`: under Current Gaps, replace vague "prototype" wording with
  a pointer to the production contract and a one-line summary of the gap.
- `docs/operations/release-readiness.md`: keep current MVP limitations, but
  point managed hosted sync to the new contract instead of leaving it as a
  vague frontier item.

Do not remove the existing safety promises.

**Verify**: `rg -n "hosted-sync-production|managed hosted sync|metadata only" README.md ARCHITECTURE.md docs/operations/release-readiness.md docs/architecture/hosted-sync-production.md` -> at least one match in each touched docs file.

### Step 3: Run the cheap non-doc gates

Because this is docs-only, do not run full release builds. Run the cheap gates
that prove the repo is still healthy and the release config still parses.

**Verify**:

- `go test ./... -count=1` -> exit 0.
- `cd tui && bun test` -> all tests pass.
- `cd tui && bun run typecheck` -> exit 0.
- `goreleaser check` -> `1 configuration file(s) validated`.

## Test plan

This plan is docs-only. The main regression check is that the docs now give
future executors a concrete managed-hosted-sync contract and do not imply that
source files or secrets will be uploaded. The Go/TUI/release checks are
confidence gates that the repo remained stable while editing docs.

## Done criteria

- [ ] `docs/architecture/hosted-sync-production.md` exists and contains the
  sections named in Step 1.
- [ ] README, ARCHITECTURE, and release-readiness docs link to the new contract.
- [ ] The new contract preserves the metadata-only sync guarantee.
- [ ] The new contract splits production hosted sync into small future
  implementation slices.
- [ ] `go test ./... -count=1` exits 0.
- [ ] `cd tui && bun test` exits 0.
- [ ] `cd tui && bun run typecheck` exits 0.
- [ ] `goreleaser check` exits 0.
- [ ] No source files are modified.
- [ ] `plans/README.md` row 023 is updated when complete.

## STOP conditions

Stop and report back if:

- Plan 022 has not landed and the docs still contain stale reconcile/FUSE
  claims that would make the production contract contradictory.
- The hosted server code no longer uses the single-token/file-backed shape
  described in Current state.
- The contract cannot preserve the metadata-only sync guarantee.
- You need to choose a real auth provider, database vendor, or hosting vendor
  to complete this plan.

## Maintenance notes

Future hosted-sync implementation plans should point back to this contract and
modify one production slice at a time. Reviewers should reject broad "make it
SaaS" diffs that combine auth, persistence, billing, UI, and deploy changes in
one PR.
