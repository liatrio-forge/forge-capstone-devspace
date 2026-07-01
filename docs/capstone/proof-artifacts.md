# Proof Artifacts

Use this file as the final evidence checklist for Module 5. Fill in permanent
links as releases, PRs, and recordings are created.

## Product Evidence

| Artifact | Status | Evidence |
| --- | --- | --- |
| CLI implementation | Done | `cmd/devdrop/main.go`, `internal/devdrop/` |
| Manifest example | Done | `examples/manifest.json` |
| Release readiness notes | Done | `docs/release-readiness.md` |
| Capstone spec | Done | `docs/capstone/spec.md` |
| Case study | Drafted | `docs/capstone/case-study.md` |
| Demo script | Drafted | `docs/capstone/demo-script.md` |
| Remote-agent case study | Drafted | `docs/capstone/remote-agent-case-study.md` |
| MVP wave args | Done | `ops/wave-ship/devdrop-mvp.args.json` |
| Capstone stretch wave args | Drafted | `ops/wave-ship/devdrop-capstone.args.json` |
| Release binary | Pending | Add GitHub release URL |
| Final demo recording | Pending | Add recording URL |
| Personal reflection | Pending | Add reflection file or section after feedback |

## Verification Commands

Run these before final demo day:

```bash
go test ./...
go vet ./...
go build -o .tmp/devspace ./cmd/devdrop
.tmp/devspace --help
```

Expected current baseline:

- `go test ./...` passes.
- The build produces a local `devspace` binary.
- `devspace --help` lists workspace, project, env, scan, plan, apply, status,
  and version commands.

## Demo Evidence To Capture

Record a terminal walkthrough that shows:

1. Build the binary.
2. Create a temporary workspace and local bare Git project remote.
3. Initialize workspace A.
4. Clone one project into workspace A and scan it.
5. Create a local bare manifest remote.
6. Push the manifest.
7. Initialize workspace B.
8. Pull the manifest into workspace B.
9. Run `plan` and `apply` to create placeholder structure.
10. Hydrate the placeholder Git project.
11. Store, list, and pull an encrypted env value.
12. Show final `devspace status`.
13. Show the remote-agent delivery workflow: `.claude/workflows/wave-ship.js`,
    `ops/wave-ship/devdrop-mvp.args.json`, and
    `ops/wave-ship/devdrop-capstone.args.json`.

## Safety Proof Points

- Path traversal is rejected by manifest validation and project add.
- Invalid remote manifest JSON is rejected before local replacement.
- Pull refuses to overwrite local unpushed manifest changes.
- Apply refuses stale saved plans when the manifest hash changes.
- Hydrate refuses non-empty destinations.
- Secret files are encrypted at rest and list output masks values.

These are covered by tests in `internal/devdrop/hardening_test.go`,
`internal/devdrop/workspace_sync_test.go`, and `internal/devdrop/devdrop_test.go`.

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
