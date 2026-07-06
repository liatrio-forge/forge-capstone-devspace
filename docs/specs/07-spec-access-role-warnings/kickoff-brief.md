# Kickoff Brief: Spec 07 — Access Role Warnings

## Status

Input package for the `/sdd-html` workflow. Not a spec. An SDD session should
be able to start from this document alone without re-deriving scope from
`docs/architecture/access-roles.md`, though that document remains the source
of truth for anything this brief paraphrases.

## Problem

DevSpace manifests already record access roles — `owner`, `maintainer`,
`developer`, and `viewer` — on `User`, `Team`, `TeamMember`, and
`ProjectAccess` records. These roles are advisory metadata only: the CLI
records intent but enforces nothing. No command today refuses, warns, or
changes behavior based on a role. The only real access boundary is
cryptographic — encrypted env profiles can be decrypted only by active `age`
recipients. Because roles look like permissions in the schema and docs, teams
may assume role-based enforcement exists when it does not. This is a
presentational risk, not a security gap: the fix is to make the CLI's
posture honest, and optionally add non-blocking feedback.

## Scope

Per the follow-up cards in `docs/architecture/access-roles.md`:

- Add a warning-only `effectiveRole` resolution helper (internal), covered by
  table-driven tests. It must not be wired to any command refusal.
- Add optional CLI warnings on the highest-risk shared mutations:
  `workspace push`, `hosted push`, `project remove`, and recipient changes
  (invite/revoke/rotate).
- Add a docs pass that replaces permission-implying language with the
  advisory-role wording the architecture doc already specifies.

## Out of Scope

- Hosted per-user token issuance, rotation, revocation, and audit logging —
  the architecture doc calls this out as its own later spec, gated on having
  a token-to-user model.
- Any hard enforcement (blocking, refusing, or altering command behavior
  based on role).
- Blocking any command on unknown or unmatched users.
- Revisiting direct-vs-team precedence (follow-up card 5 in access-roles.md)
  — deferred until real team workflows create conflicting grants; Q1's
  proposed default stands until then.

## Effective-Role Resolution Rules (from the architecture doc)

These rules govern the `effectiveRole` helper referenced in scope above,
quoted from `docs/architecture/access-roles.md`:

- Match identity by `User.AgeRecipient == localAgeRecipient`.
- Ignore users with `Status == "revoked"` or non-empty `RevokedAt`.
- Ignore `ProjectAccess` entries with non-empty `RevokedAt`.
- Ignore `TeamMember` entries with non-empty `RevokedAt`.
- Consider direct grants where `ProjectAccess.UserID` matches the user.
- Consider team grants where `ProjectAccess.TeamID` points to a team
  containing the user as an active member.
- Order roles from most to least privileged: `owner`, `maintainer`,
  `developer`, `viewer`.
- If several active grants apply, use the most privileged resulting role for
  backward compatibility and fewer surprising warnings.
- For team grants, cap the project access role by the member's team role. A
  `viewer` member of a team with `maintainer` access should resolve as
  `viewer`.
- If no user matches the local age recipient, continue and warn that no
  local manifest user was found.
- If a user exists but no active project grant applies, continue and warn
  that no project role was found.
- Unknown roles should continue and warn rather than fail.

Default recommendation for warning-only mode (from the doc): permissive-with-warning.

## Open Design Questions

These are the seven open questions from `docs/architecture/access-roles.md`
("Open Questions" section), quoted accurately. Each has a **Proposed
default** — this is a proposal from this brief's author, not a decision.
The SDD questions phase must confirm or veto each one.

**Q1 — Direct-vs-team grant precedence.** "Should direct grants override
team grants, or should the most privileged grant win?" The doc's own
recommendation: "most privileged active grant wins for backward
compatibility; revisit only for server-side enforcement."

- Proposed default: most-privileged grant wins for computation; warn when a
  direct grant and a team grant disagree, so the ambiguity is visible even
  though it isn't blocking.

**Q2 — Member-role capping.** "Should a team member's `Role` cap
project-level team access?" Doc recommendation: "yes. It makes team
membership meaningful and avoids a viewer inheriting broad mutating access
through a team grant."

- Proposed default: the cap applies inside the `effectiveRole` computation
  only (i.e., it's a resolution rule, not a separate enforcement layer).

**Q3 — Unknown-user blocking.** "Should unknown users be blocked once
warnings exist?" Doc recommendation: "no for the client-side path. Continue
with a warning so old single-user manifests keep working."

- Proposed default: no — never block; warnings only, in all cases.

**Q4 — Developer manifest push.** "Should `developer` be allowed to push
shared manifests?" Doc recommendation: "no by default. Developers can mutate
local workspace state; maintainers publish shared metadata."

- Proposed default: the command remains allowed (warning-only never
  blocks), but emits a warning naming the pusher's effective role as
  outside the recommended boundary for `workspace push`.

**Q5 — Viewer env pull.** "Should `env pull` be available to viewers?" Doc
recommendation: "yes. The real gate is cryptographic decryptability, and
viewer is useful for read-only secret consumers."

- Proposed default: age recipients (cryptographic) continue to govern env
  access; the role check only adds an informational warning, never a
  restriction.

**Q6 — Hosted enforcement before per-user tokens.** "Should hosted sync
enforce roles before per-user tokens exist?" Doc recommendation: "no. A
single bearer token cannot safely represent manifest users."

- Proposed default: no hosted enforcement of any kind until per-user tokens
  exist (tracked as its own later spec, explicitly out of scope here).

**Q7 — Role-aware conflict resolution.** "Should Plan 013 conflict
resolution understand access roles?" Doc recommendation: "not for initial
reconciliation. If server-side enforcement is pursued, combine it with Plan
013-style merge rules for users and access records."

- Proposed default: no role-awareness in reconcile for this spec; revisit
  only if/when server-side enforcement is designed.

## Suggested Verification Shape

- Table-driven unit tests for `effectiveRole` covering: direct vs. team
  grants, revoked users/grants/members, team-role capping, no-match cases,
  and unknown roles.
- Warning-output assertions (golden or substring) on the four mutation
  paths: `workspace push`, `hosted push`, `project remove`, recipient
  changes.
- `make verify` as the merge gate; no new command should refuse to run as a
  result of this spec.

## Source of Truth

`docs/architecture/access-roles.md` — read it directly for anything this
brief summarizes. Quotes above were checked against that document at the
time this brief was written.
