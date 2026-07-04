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
| Case study | Drafted | `docs/capstone/case-study.md` |
| Demo script | Done | `scripts/demo-check.sh`, `docs/capstone/demo-script.md` |
| Remote-agent case study | Drafted | `docs/capstone/remote-agent-case-study.md` |
| Frontier Linear cards | Drafted | `CIL-227` through `CIL-231` |
| Release binary | Pending | Add GitHub release URL |
| Final demo recording | Pending | Add recording URL |
| Personal reflection | Pending | Add reflection file or section after feedback |

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
- `devspace --help` lists init, workspace, project, env, scan, plan, apply,
  status, and version commands.
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

Before final submission, update this section with exact evidence:

```text
Release tag:
Release URL:
Commit SHA:
Demo recording:
Remote-agent run:
Wave cards:
PR evidence:
Case study reviewed by:
Final test command output:
Known limitation accepted:
```
