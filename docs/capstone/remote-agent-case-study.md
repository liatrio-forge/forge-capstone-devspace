# Remote Agent Case Study

DevDrop is also a case study in agent-orchestrated delivery. The MVP was planned
as a set of independently shippable cards and prepared for execution through the
checked-in Claude workflow files:

- `.claude/workflows/wave-ship.js`
- `.claude/workflows/ship-card.js`
- `ops/wave-ship/devdrop-mvp.args.json`

## Why This Matters

The capstone is not only "we built a CLI." The stronger claim is that a small
team can use AI agents as a delivery system when the work is decomposed into
clear, testable, file-scoped cards.

For DevDrop, the workflow turns a product goal into:

1. Linear cards with explicit dependencies.
2. Isolated worker execution.
3. Pull requests per card.
4. CI and review loops.
5. Serialized merge back to the base branch.
6. Final reconciliation against the original goal.

That makes the agent system inspectable. Each worker either produces a PR with
verification evidence or reports a blocker.

## Workflow Shape

`wave-ship.js` owns the portfolio-level loop:

- Accepts a goal, plan, cards, or waves.
- Builds a dependency graph from card `dependsOn` values.
- Dispatches cards as soon as dependencies are merged.
- Caps global concurrency with `maxConcurrent`.
- Serializes migration cards.
- Optionally uses the `orca` backend for isolated remote worker execution.
- Owns serialized merges when `serializedMerge` is true.
- Reconciles blocked or incomplete cards into remediation work.

`ship-card.js` owns one unit of work:

- Resolves or creates the Linear ticket.
- Creates the implementation branch/worktree.
- Runs the build agent.
- Opens the PR.
- Watches CI and review feedback.
- Pushes fixes until the PR is clean or blocked.
- Lands the work or returns `merge-ready` to the coordinator.

## DevDrop MVP Wave

The initial prepared wave is stored in
`ops/wave-ship/devdrop-mvp.args.json`.

It uses:

```json
{
  "backend": "orca",
  "serializedMerge": true,
  "autoContinue": false,
  "maxConcurrent": 3,
  "engine": "codex"
}
```

The five cards map to the core MVP:

| Card | Purpose |
| --- | --- |
| `CIL-217` | Scaffold the Go CLI and init command |
| `CIL-218` | Manifest model, workspace scan, and status |
| `CIL-219` | Project add, workspace sync, and Git hydration |
| `CIL-220` | Encrypted env profile commands |
| `CIL-221` | README, examples, and end-to-end MVP verification |

## Capstone Stretch Wave

The next wave is stored in
`ops/wave-ship/devdrop-capstone.args.json`.

It pushes the MVP toward a stronger final demo without changing the product into
a hosted platform:

1. Release packaging and install docs.
2. Executable demo-check harness.
3. `devspace doctor` diagnostics.
4. Manifest remote diff/reconcile preview.
5. Final capstone evidence, reflection, and demo readiness.

## Guardrails

The workflow is powerful enough to create risk if the cards are vague. DevDrop
keeps the risk controlled with these rules:

- Every card declares scope and explicit non-goals.
- Workers use isolated worktrees.
- The coordinator owns serialized merge.
- Product cards must include local verification commands.
- Demo and release cards must avoid real secrets.
- Cloud-agent evidence belongs in docs and PR links, not in generated secrets,
  logs, or hidden local state.

## What To Show In The Final Demo

Use the remote-agent system as the "how it was built" segment:

1. Open `ops/wave-ship/devdrop-mvp.args.json`.
2. Explain the dependency graph: init first, scan/hydrate/env in parallel, docs
   and verification last.
3. Open `.claude/workflows/wave-ship.js`.
4. Point to `backend: "orca"`, `maxConcurrent`, and `serializedMerge`.
5. Show the Linear/PR evidence for each card if available.
6. Explain what failed or required human judgment.
7. Tie it back to client enablement: agents work when the work is decomposed
   into explicit contracts with observable gates.

