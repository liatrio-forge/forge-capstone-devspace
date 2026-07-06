# Follow-ups

Tracked work plan: [docs/superpowers/plans/2026-07-05-followups.md](docs/superpowers/plans/2026-07-05-followups.md)

| Card | Item | Priority | Status |
| ---- | ---- | -------- | ------ |
| F1 | macOS FUSE local proof (closes spec 03 PENDING marker) | P1 | Done — real mount smoke test passed 2026-07-06; proof at docs/specs/03-spec-fuse-lazy-mount/03-proofs/03-macos-local-mount-proof.md. Spec 02 Plan 015 stays open separately — it needs GitHub Actions FUSE probe evidence, not a local run. |
| F2 | Spec-01 attestation M-1: make repo public + `gh attestation verify`, or accept | P2 | Done — M-1 accepted (repo stays private), recorded in spec-01 validation |
| F3 | Capstone wave-5: finalize case studies, demo recording, reflection, proof links | P1 (deadline) | Partially done — case studies marked Done; demo recording + reflection remain manual; stretch-card decision open |
| F4 | Spec 07: access-role warning tier (`effectiveRole`, CLI warnings) | P2 | Implemented (Codex delegation, reviewed + accepted) — PR #34 |
| F5 | Spec 08: reconcile in `devspace ui`, per-project `--force` | P3 | Implemented (Codex delegation, reviewed + accepted) — PR #35 |
| F6 | `ko` base-image digest automation; advisory-language doc pass | P3 | Opportunistic (advisory-language pass shipped with F4) |

Closed 2026-07-05: doc-sync PR (stale README/roadmap/release docs, spec-01
FLAGs, plans/README backlog) and the `workspace diff` → `workspace pull`
false-refusal fix (merged in PR #31).
