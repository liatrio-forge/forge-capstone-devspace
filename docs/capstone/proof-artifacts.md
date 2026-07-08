# Proof Artifacts

Use this file as the final evidence checklist for Module 5. Fill in permanent
links as releases, PRs, and recordings are created.

## Product Evidence

| Artifact | Status | Evidence |
| --- | --- | --- |
| CLI implementation | Done | `cmd/devspace/main.go`, `internal/devspace/` |
| Manifest example | Done | `examples/manifest.json` |
| Release readiness notes | Done | `docs/operations/release-readiness.md` |
| Capstone spec | Done | `docs/capstone/spec.md` |
| Case study | Done | `docs/capstone/case-study.md` |
| Demo script | Done | `scripts/demo-check.sh`, `docs/capstone/demo-script.md` (demo-check.sh verified 2026-07-06) |
| Remote-agent case study | Done | `docs/capstone/remote-agent-case-study.md` |
| Frontier Linear cards | Drafted | `CIL-227` through `CIL-231` |
| Release binary | Done | https://github.com/liatrio-forge/devdrop-capstone/releases/tag/v0.2.0 |
| Final demo recording | Pending | Filled at end of wave 5 |
| Personal reflection | Pending | Filled at end of wave 5 |

## Verification Commands

Run these before final demo day:

```bash
go test ./...
go vet ./...
go build -o .tmp/devspace ./cmd/devspace
.tmp/devspace --help
scripts/demo-check.sh
```

Expected current baseline:

- `go test ./...` passes.
- The build produces a local `devspace` binary.
- `devspace --help` lists apply, completion, doctor, env, hosted, init, mount,
  plan, project, scan, setup, status, tui, ui, version, watch, and workspace
  commands.
- `scripts/demo-check.sh` passes without network access, GitHub auth, Linear
  auth, or real secrets.

## Demo Evidence To Capture

Run the executable walkthrough:

```bash
proof_dir="$(mktemp -d /tmp/devdrop-demo-proof.XXXXXX)"
scripts/demo-check.sh --output-dir "$proof_dir"
```

Capture the command output and the generated
`$proof_dir/demo-check-summary.txt`. The walkthrough proves:

1. Build the binary.
2. Create temporary workspaces and local bare Git remotes.
3. Initialize workspace A.
4. Clone one project into workspace A and scan it.
5. Create a local bare manifest remote.
6. Push the manifest.
7. Initialize workspace B.
8. Pull the manifest into workspace B.
9. Run `plan` and `apply` to create placeholder structure.
10. Hydrate the placeholder Git project.
11. Store, list, and pull an encrypted env value.
12. Assert the generated `.env` mode is `0600`.
13. Show final `devspace status` and `devspace project status`.
14. Show the remote-agent delivery workflow: `.claude/workflows/wave-ship.js`
    and `.claude/workflows/ship-card.js`.
15. Show the frontier track: hosted sync, daemon/watch, FUSE, team secrets, and
    explicit dependency install.

## Safety Proof Points

- Path traversal is rejected by manifest validation and project add.
- Invalid remote manifest JSON is rejected before local replacement.
- Pull refuses to overwrite local unpushed manifest changes.
- Apply refuses stale saved plans when the manifest hash changes.
- Hydrate refuses non-empty destinations.
- Secret files are encrypted at rest and list output masks values.

These are covered by tests in `internal/devspace/hardening_test.go`,
`internal/devspace/workspace_sync_test.go`, and `internal/devspace/devspace_test.go`.

## Release Gate

- Release tag: `v0.2.0`
- Release URL:
  https://github.com/liatrio-forge/devdrop-capstone/releases/tag/v0.2.0
- Commit SHA: `410f8e29c9318d90ea1262b71802d44531389ec6`
- Demo recording: Pending; filled at end of wave 5.
- Remote-agent run: case study at
  https://github.com/liatrio-forge/devdrop-capstone/blob/main/docs/capstone/remote-agent-case-study.md
- Wave cards: `CIL-227` through `CIL-231` plus the wave-5 fleet, including
  `CIL-242`, shipped by autonomous remote agents via
  `.claude/workflows/wave-ship.js` and `.claude/workflows/ship-card.js`.
- PR evidence:
  - #10 Prototype FUSE lazy workspace mount:
    https://github.com/liatrio-forge/devdrop-capstone/pull/10
  - #11 refactor: rename devdrop -> devspace with automatic legacy migration:
    https://github.com/liatrio-forge/devdrop-capstone/pull/11
  - #12 feat: add CI/CD pipeline with GoReleaser distribution:
    https://github.com/liatrio-forge/devdrop-capstone/pull/12
  - #13 fix: skip artifact attestation on private repositories:
    https://github.com/liatrio-forge/devdrop-capstone/pull/13
  - #14 fix: point CI/GoReleaser build at ./cmd/devspace after rename:
    https://github.com/liatrio-forge/devdrop-capstone/pull/14
  - #15 feat: publish hosted-server image via GoReleaser ko +
    container-ready server:
    https://github.com/liatrio-forge/devdrop-capstone/pull/15
  - #25 ci: probe FUSE support on hosted runners:
    https://github.com/liatrio-forge/devdrop-capstone/pull/25
  - #26 feat: styled terminal output with Charm:
    https://github.com/liatrio-forge/devdrop-capstone/pull/26
- Case study reviewed by: wave-5 remote-agent review loop with CodeRabbit and
  CI on the shipped PRs above.
- Final test command output: `go test ./...` passed in this worktree:
  `? github.com/liatrio-forge/devdrop-capstone/cmd/devspace [no test files]`
  and `ok github.com/liatrio-forge/devdrop-capstone/internal/devspace`. The
  full `go test ./...` suite passes.
- Known limitation accepted: hosted sync, daemon/watch, FUSE lazy mounting,
  managed team identity, and explicit dependency install remain frontier
  prototypes outside the completed local-first MVP baseline. MVP sync exchanges
  manifests through user-owned Git remotes only, secrets use explicit age
  recipients only, and dependency/setup commands are hints that are never
  auto-executed; see
  https://github.com/liatrio-forge/devdrop-capstone/blob/main/docs/operations/release-readiness.md#known-limitations.
