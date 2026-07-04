# DevDrop Capstone Packet

This packet maps DevDrop to the Liatrio Forge Module 5 capstone deliverables.
DevDrop is a local-first developer workspace recovery CLI. It helps a developer
recreate the shape of a working machine on a second machine without syncing
source code, overwriting local work, or storing secrets in plaintext.

For a browsable HTML version of the repo Markdown, open
[`index.html`](index.html).

## Capstone Thesis

Developers lose time rebuilding workspaces after machine changes, client
rotations, or repo sprawl. DevDrop proves a smaller, safer alternative to a
hosted "sync everything" service: track workspace metadata, push the manifest to
a user-owned Git remote, hydrate projects on demand, and keep env values
encrypted locally.

## Deliverable Map

| Forge deliverable | DevDrop artifact |
| --- | --- |
| Capstone product | Go CLI built as `devspace`, with local manifest sync, plan/apply, hydration, and encrypted env profiles |
| Case study write-up | [case-study.md](case-study.md) |
| Proof artifacts | [proof-artifacts.md](proof-artifacts.md), `docs/operations/release-readiness.md`, tests under `internal/devspace/` |
| Final demo recording | [demo-script.md](demo-script.md) and `../../scripts/demo-check.sh` |
| Playbook contribution | [playbook-contribution.md](playbook-contribution.md), `docs/playbook.html`, and the enablement notes in [case-study.md](case-study.md) |
| Remote-agent delivery case study | [remote-agent-case-study.md](remote-agent-case-study.md), `.claude/workflows/wave-ship.js`, and `.claude/workflows/ship-card.js` |
| Personal reflection | Add final reflection after the recorded demo and panel feedback |

## Current Product Surface

- `devspace init` creates local config, machine identity, age identity, and the
  workspace manifest.
- `devspace scan` discovers Git projects, dirty state, env presence, and setup
  hints.
- `devspace plan` and `devspace apply` separate review from mutation.
- `devspace workspace remote|push|pull` syncs only `manifest.json` through a
  user-owned Git repository.
- `devspace project hydrate` clones placeholder Git projects on demand.
- `devspace env set|list|pull` stores encrypted profiles and writes local `.env`
  files with `0600` permissions.

## Remaining Module 5 Work

1. Create a tagged release binary and attach it to a GitHub release.
2. Execute the capstone stretch cards documented in
   [remote-agent-case-study.md](remote-agent-case-study.md), whether via a
   wave-ship run or manually.
3. Push the frontier track: hosted sync, daemon/watch mode, FUSE lazy mount,
   managed team identity, and explicit dependency install.
4. Record the demo using [demo-script.md](demo-script.md).
5. Capture final proof links in [proof-artifacts.md](proof-artifacts.md).
6. Add the personal reflection after demo feedback is received.
7. Migrated on-disk paths and config directories to `.devspace`.
