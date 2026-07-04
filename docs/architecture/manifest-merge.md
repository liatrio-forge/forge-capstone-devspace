# Manifest Conflict Reconciliation Spike

Manifest pull currently refuses when local and remote manifests both changed since the last sync point. That is the right default safety backstop, but it leaves multi-machine users with a manual JSON merge when two laptops scan or edit different projects before syncing.

This spike proposes an opt-in three-way merge:

- `base`: last-synced manifest copy
- `ours`: current local manifest
- `theirs`: pulled remote manifest

The prototype is intentionally unwired. Default pull behavior should continue to refuse until the merge UX and unresolved policy questions are answered.

## Problem

Git-backed sync already has a prior remote copy before pull, and hosted sync tracks the last synced version/hash. Both paths can provide the third point needed for safe reconciliation. Without using that base, DevSpace cannot distinguish independent additions from conflicting edits, so it refuses all divergence.

The merge must remain non-destructive. A pull should not silently delete a project or access rule that exists on only one side. Merged manifests must still satisfy `ValidateManifest`, including safe project IDs, safe paths, unique project names/paths, valid remotes, and access referential integrity.

Plan 006 matters here: post-Plan-006 `mergeProject` preserves user-set `ignore`, non-default `hydrateMode`, and existing `envProfiles` across rescans. Manifest reconciliation should not undo those local override-preservation semantics. If one side is only a rescan-derived update and the other side carries preserved user overrides, the eventual wired merge should prefer the preserved values rather than treating generated defaults as authoritative user intent.

## Options Considered

### Keep Refuse-Only

Safest and simplest, but it does not solve the named roadmap gap. Users still hand-edit JSON for independent additions.

### Two-Way Merge

Union local and remote manifests directly. This is rejected because it cannot tell delete from absence, or generated defaults from intentional edits.

### Three-Way Merge With Refusal On True Conflicts

Use the last-synced copy as base. Independent additions and one-sided modifications merge automatically. Both-sided edits to the same record conflict unless field-level policy can prove they are compatible. This is the chosen behavior.

## Chosen Behavior

The merge unit is a stable identity:

- `Projects`: `Project.ID`
- `Access`: `ProjectID + UserID + TeamID`
- `Users`, `Teams`, and `Machines`: follow-up scope

The prototype merges Projects and Access only. It leaves Users, Teams, Machines, `Version`, and `WorkspaceRoot` from `ours`, then validates successful non-conflict output.

| Entity | Added on both sides | Modified ours only | Modified theirs only | Modified both | Deleted vs modified |
| --- | --- | --- | --- | --- | --- |
| Project | Different IDs: union if manifest validation passes. Same ID and identical content: keep one. Same ID with different content: conflict. Same path with different IDs: open question. | Take ours. | Take theirs. | Conflict unless the records are identical. Future field-level merge can preserve Plan-006 `mergeProject` user overrides for `ignore`, `hydrateMode`, and `envProfiles`. | Keep the modified/present project. Do not delete automatically. |
| Access | Different composite keys: union if references remain valid. Same key and identical content: keep one. Same key with different role/profile/revocation data: conflict. | Take ours. | Take theirs. | Conflict for role, env profile, or revocation differences. | Conflict. Access delete-vs-modify is security-sensitive and must block for explicit operator review. |
| User | Follow-up. | Follow-up. | Follow-up. | Follow-up. | Follow-up. |
| Team | Follow-up. | Follow-up. | Follow-up. | Follow-up. | Follow-up. |
| Machine | Prefer local machine localization after sync. | Follow-up. | Follow-up. | Follow-up. | Follow-up. |

## UX Proposal

Add opt-in flags after the policy questions are closed:

- `devspace workspace pull --merge`
- `devspace hosted pull --merge`

Default pull remains refuse-and-explain. With `--merge`, pull should print a summary before saving:

```text
Merged manifest changes:
  projects: +2, modified 1, kept 1 delete-vs-modify
  access: +1, modified 0
  conflicts: 0
```

When true field-level conflicts exist, the command refuses and prints conflict records with entity, key, field, ours, and theirs. Future `--force-theirs` and `--force-mine` can resolve conflicts explicitly, but they should be separate from non-destructive merge.

## Rollout

1. Land the unwired prototype and tests.
2. Resolve open questions below.
3. Add a narrow internal merge proof path for both sync backends without changing default behavior.
4. Add `--merge` behind explicit CLI flags.
5. Keep default pull refusal until the merge path has real-world proof and readable diagnostics.

## Follow-Up Cards

- Implement field-level Project merging that composes with post-Plan-006 `mergeProject` preservation semantics.
- Extend reconciliation to Users and Teams so Access rules from remote machines can validate when identities are added independently.
- Add CLI conflict rendering and machine-readable conflict output for future editor tooling.
- Design `--force-theirs` and `--force-mine` separately from `--merge`.
- Add sync-backend tests proving git `previousRemote` and hosted last-synced state provide the expected base.

## Open Questions

### What wins when the same path has different project IDs?

Recommendation: refuse with a conflict. Path is user-facing and `ValidateManifest` requires uniqueness. Auto-renaming IDs would make secret/profile state ambiguous.

### What wins when the same Access key has different roles?

Recommendation: refuse with a conflict. Access role changes are security-sensitive, and choosing the stronger or weaker role silently is wrong.

### Should `--force` exist independently of merge?

Recommendation: yes, but only as explicit directional flags such as `--force-theirs` and `--force-mine`. Plain `--force` is too ambiguous for sync.

### What if a backend lacks a recoverable base?

Recommendation: refuse merge and explain that a successful sync baseline is required. Do not fall back to two-way merge.

### Should delete-vs-modify be reported as a conflict?

Recommendation: keep non-destructively and report it as a warning in the summary, not as a blocking conflict.
