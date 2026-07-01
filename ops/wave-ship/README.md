# DevDrop MVP Wave-Ship Prep

## Workflow

Use the workflow files checked into this repo:

```text
.claude/workflows/wave-ship.js
.claude/workflows/ship-card.js
```

`wave-ship.js` is the top-level coordinator. It decomposes or accepts cards,
dispatches them through the configured backend, and owns serialized merge when
`serializedMerge` is `true`.

`ship-card.js` is the per-card runner. `wave-ship.js` calls it for each card, so
keep both files together when copying or updating the workflow.

The old Inkwell workflow path is only an upstream reference for comparison:

```text
/Users/lecoqjacob/Projects/personal/inkwell/.claude/workflows/wave-ship.js
```

## Args

Pass `devdrop-mvp.args.json` as the workflow args JSON for the original MVP wave.
Pass `devdrop-capstone.args.json` for the stretch wave that pushes the project to
final capstone readiness.

Example:

```text
Run .claude/workflows/wave-ship.js with ops/wave-ship/devdrop-mvp.args.json
```

The prepared run uses:

- `repo`: `/Users/lecoqjacob/Projects/personal/devdrop`
- `project`: `DevDrop MVP`
- `team`: `Cypress Ink Labs`
- `base`: `main`
- `backend`: `orca`
- `serializedMerge`: `true`
- `autoContinue`: `false`
- `maxConcurrent`: `3`
- `engine`: `codex`

## Linear Cards

Wave 1:

- `CIL-217` — DevDrop: scaffold Go CLI and init command

Wave 2:

- `CIL-218` — DevDrop: manifest model, workspace scan, and status
- `CIL-219` — DevDrop: project add, workspace sync, and Git hydration
- `CIL-220` — DevDrop: encrypted env profile commands

Wave 3:

- `CIL-221` — DevDrop: README, examples, and end-to-end MVP verification

## Capstone Stretch Cards

`devdrop-capstone.args.json` adds the next wave:

- Release packaging and install docs.
- Executable local demo-check harness.
- `devspace doctor` diagnostics.
- Manifest remote diff preview.
- Final capstone evidence and reflection.

## Boundaries

- Workers open PRs and do not merge.
- `wave-ship` owns serialized merge and ticket closeout.
- Do not build hosted sync, FUSE, a daemon, team secrets, editor settings sync,
  or dependency auto-install in this run.
- Do not put real secrets in docs, fixtures, logs, or tests.
