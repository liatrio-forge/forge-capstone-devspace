# Case Study: DevDrop

## Client-Friendly Summary

DevDrop is a local-first workspace recovery CLI for developers who move across
machines, clients, and project sets. Instead of syncing entire folders, it tracks
safe workspace metadata: project paths, Git remotes, setup hints, dirty state,
placeholder folders, and encrypted env profiles.

The result is a repeatable way to rebuild a developer workspace on a second
machine without copying source code, dependency folders, generated files, or
plaintext secrets.

## Problem

Developer onboarding and machine recovery often depend on tribal knowledge:
which repos matter, where they live, which branch was active, which projects
have env files, and which setup command should run first. Copying a whole
workspace is risky because it can move secrets, client code, large generated
artifacts, and stale dependency folders.

## Solution

DevDrop separates workspace knowledge from workspace contents.

- A local manifest records project metadata.
- A Git-backed sync flow moves only `manifest.json` through a user-owned remote.
- A plan/apply workflow shows filesystem changes before creating placeholders.
- Git projects hydrate on demand with normal `git clone`.
- Env values are encrypted locally with age and only materialized into `.env`
  files through an explicit command.

## Outcome

The current MVP proves the core workflow:

- Initialize a workspace.
- Scan projects.
- Push workspace metadata to a Git remote.
- Pull that metadata on a second machine.
- Recreate placeholder structure safely.
- Hydrate a project only when needed.
- Keep env values encrypted at rest.

## Why AI-Native Delivery Mattered

The capstone demonstrates an AI-assisted engineering workflow in a realistic
product slice:

- The product was defined around explicit safety boundaries before expanding
  features.
- Tests captured destructive edge cases, not only the happy path.
- The implementation stayed local-first, making demos and verification possible
  without hosted infrastructure.
- Documentation records what was tested, what passed, what failed, and what
  remains risky.
- The delivery plan was shaped as remote-agent cards through
  `.claude/workflows/wave-ship.js`, giving the team a concrete example of
  agent orchestration rather than a generic claim about AI usage.

The main lesson for client enablement is that AI acceleration works best when it
is paired with small, explicit contracts: specs, acceptance criteria, safety
tests, and release gates.

## Delivery Case Study

DevDrop uses the checked-in `wave-ship` workflow as a second capstone artifact.
The workflow decomposes a product goal into Linear-backed cards, dispatches work
to isolated agent workers, opens PRs, watches CI/review feedback, and merges
serially through the coordinator.

That process is valuable to a client because it makes agent delivery auditable:
scope, dependencies, PR evidence, verification commands, and blockers are all
visible. The capstone demo should show both the product and the delivery system
that produced it.

## Trade-Offs

- Git-backed manifest sync is easy to inspect and host, but it does not provide
  real-time collaboration or conflict resolution UI.
- Placeholder directories are safe and simple, but they do not provide lazy file
  access like FUSE or virtual filesystem designs.
- Env profiles are encrypted locally, but there is no team sharing, cloud
  backup, or rotation flow.
- Setup commands are detected as hints, but DevDrop does not install
  dependencies because automatic execution would cross a safety boundary.

## Client Enablement Notes

Use DevDrop as a teaching example for AI-native delivery:

1. Start with the unsafe default behavior and name it plainly.
2. Define what the tool will not do without permission.
3. Write acceptance criteria for safety cases before demo polish.
4. Keep proof artifacts close to the repo: tests, release notes, demo scripts,
   and case study.
5. Show how AI helped compress delivery time while the specification preserved
   control.

## Current Limitations

- No hosted sync service.
- No background daemon or filesystem watcher.
- No source-code syncing.
- No team secret sharing.
- The intended binary name is `devspace`, while some on-disk paths still use
  `.devdrop`.
