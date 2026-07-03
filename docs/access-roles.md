# Access Role Posture

DevSpace manifests already contain users, teams, and project access roles, but
the current CLI does not enforce those roles. This document records the intended
posture so the product language stays honest and future enforcement work has a
stable target.

## Problem

The manifest schema has `User`, `Team`, `TeamMember`, and `ProjectAccess`
records with `owner`, `maintainer`, `developer`, and `viewer` roles. Manifest
validation checks that those records refer to existing users, teams, and
projects.

No command currently refuses, warns, or changes behavior based on those roles.
The hosted sync prototype also uses one bearer token for every workspace ID, so
it cannot map a request to a manifest user. The only hard access boundary today
is cryptographic: encrypted env profiles can be decrypted only by active age
recipients.

The immediate risk is presentational. If docs say or imply that roles control
what a user can do, teams may assume enforcement exists when it does not.

## Candidate Postures

### Document-as-advisory

Roles remain bookkeeping metadata for humans and future automation. Docs say
explicitly that the CLI records intended responsibility but does not enforce
role-based command permissions.

This is the lowest-risk posture for the current codebase because it matches
actual behavior and does not create a weak local-only security claim.

### Client-Side Advisory Enforcement

The CLI resolves the local age recipient to a manifest user, derives an
effective role for a project, and prints warnings when a mutating command is
outside the recommended role boundary. Commands still continue.

This gives teams useful feedback without pretending that local checks are a
security boundary. The manifest is user-editable, and the CLI is open source, so
client-side enforcement can only be advisory unless paired with cryptographic or
server-side controls.

### Server-Side Enforcement

Hosted sync issues per-user tokens, maps each token to `User.ID`, and enforces
project access on every read and write request.

This is the only posture that can become a real hosted authorization boundary,
but it requires token issuance, rotation, revocation, audit logging, workspace
membership management, and migration from the current single-token server.

## Recommendation

Use document-as-advisory now. Design client-side warning-only checks as the next
increment if role feedback becomes useful in daily workflows. Defer server-side
enforcement until hosted sync has a token-to-user model.

Rationale:

- It matches the current implementation: roles are recorded and validated, not
  enforced.
- It avoids a false security claim around local checks.
- It keeps the real current boundary clear: age recipients control who can
  decrypt shared secrets.
- It leaves a clean path to warning-only UX without changing command semantics.
- It avoids blocking hosted auth design on a spike whose immediate deliverable
  is documentation accuracy.

## Effective-Role Rules

Use these rules for future warning-only checks or server-side enforcement.

- Match identity by `User.AgeRecipient == localAgeRecipient`.
- Ignore users with `Status == "revoked"` or non-empty `RevokedAt`.
- Ignore `ProjectAccess` entries with non-empty `RevokedAt`.
- Ignore `TeamMember` entries with non-empty `RevokedAt`.
- Consider direct grants where `ProjectAccess.UserID` matches the user.
- Consider team grants where `ProjectAccess.TeamID` points to a team containing
  the user as an active member.
- Order roles from most to least privileged: `owner`, `maintainer`,
  `developer`, `viewer`.
- If several active grants apply, use the most privileged resulting role for
  backward compatibility and fewer surprising warnings.
- For team grants, cap the project access role by the member's team role. A
  `viewer` member of a team with `maintainer` access should resolve as `viewer`.
- If no user matches the local age recipient, continue and warn that no local
  manifest user was found.
- If a user exists but no active project grant applies, continue and warn that
  no project role was found.
- Unknown roles should continue and warn rather than fail.

Default recommendation for warning-only mode: permissive-with-warning. That
preserves existing single-user and partially migrated manifests while making the
missing access metadata visible.

## Mutating CLI Inventory

The table lists recommended role boundaries for future advisory or enforced
behavior. It does not describe behavior that exists today.

| CLI surface | Mutation | Recommended allowed roles | Notes |
| --- | --- | --- | --- |
| `devspace init` | Creates local config, identity, state, and manifest files. | No manifest role required | Bootstrap runs before access metadata exists. |
| `devspace scan` / `devspace workspace scan` | Refreshes local manifest and state from the filesystem. | owner, maintainer, developer | Viewer can use read-only status commands instead. |
| `devspace watch --sync off` | Continuously refreshes local manifest and state. | owner, maintainer, developer | Same boundary as scan. |
| `devspace watch --sync git` / `--sync hosted` | Refreshes metadata and pushes it to shared sync. | owner, maintainer | Shared publication should be narrower than local scan. |
| `devspace plan` | Saves the last plan under local DevSpace state. | owner, maintainer, developer, viewer | Treat as inspect-only despite the cache write. |
| `devspace apply` | Applies the last safe workspace plan locally. | owner, maintainer, developer | Can hydrate or alter local workspace files. |
| `devspace workspace push` | Publishes the manifest to the configured Git remote. | owner, maintainer | Shared manifest write. |
| `devspace workspace pull` | Replaces local manifest state from the Git remote. | owner, maintainer, developer, viewer | Local write, but read-oriented from the shared source. |
| `devspace workspace sync` | Saves a plan and applies it locally. | owner, maintainer, developer | Compatibility alias for plan/apply. |
| `devspace workspace remote set` | Changes the configured manifest Git remote. | owner, maintainer | Workspace-level sync configuration. |
| `devspace workspace remote create local` | Creates and configures a local bare manifest remote. | owner, maintainer | Workspace-level sync configuration. |
| `devspace workspace remote create github` | Creates and configures a GitHub manifest remote. | owner, maintainer | External shared infrastructure. |
| `devspace hosted config set` | Stores hosted endpoint, token, and workspace ID locally. | owner, maintainer | Token-bearing sync configuration. |
| `devspace hosted push` | Publishes the manifest to hosted sync. | owner, maintainer | Shared manifest write. |
| `devspace hosted pull` | Replaces local manifest state from hosted sync. | owner, maintainer, developer, viewer | Local write, but read-oriented from the shared source. |
| `devspace hosted serve` | Runs a hosted sync server that writes manifest payloads. | owner | Server operator path until per-user auth exists. |
| `devspace project add` | Adds a project to the manifest. | owner, maintainer | Shared inventory change. |
| `devspace project hydrate` | Clones a placeholder Git project locally. | owner, maintainer, developer | Local workspace materialization. |
| `devspace project remove` | Removes a project from manifest tracking. | owner, maintainer | Shared inventory change; files and secrets remain on disk. |
| `devspace mount <mountpoint>` | Mounts a view that may hydrate projects on lookup. | owner, maintainer, developer | `--preview` should remain available to viewers. |
| `devspace env set` | Rewrites an encrypted profile value. | owner, maintainer, developer | Age recipients still determine who can decrypt. |
| `devspace env pull` | Writes a local `.env` from an encrypted profile. | owner, maintainer, developer, viewer | Only works for users with decryptable ciphertext. |
| `devspace env recipient invite` | Adds a recipient and access metadata. | owner, maintainer | Changes future decryptability. |
| `devspace env recipient revoke` | Revokes a recipient from future encrypted writes. | owner, maintainer | Cannot claw back copied or decrypted values. |
| `devspace env recipient rotate` | Rewraps a profile for active recipients. | owner, maintainer | Changes ciphertext recipient set. |
| `devspace setup run` | Runs a project setup command locally. | owner, maintainer, developer | Executes project-defined commands. |
| `devspace setup apply` | Runs install commands across runnable projects. | owner, maintainer, developer | Executes project-defined commands. |

## Documentation Wording

Use this exact wording when describing current access roles:

```text
Access roles are advisory metadata today. DevSpace records owners,
maintainers, developers, and viewers to help teams describe intended
responsibility, but the CLI does not currently refuse commands based on these
roles. Encrypted env access is controlled by age recipients, not by the role
field.
```

Avoid wording like:

```text
Viewers cannot edit env values.
Maintainers can push workspace changes.
Owners control project access.
```

Until enforcement exists, prefer:

```text
Recommended future policy: viewers should inspect, developers should mutate
local project state, maintainers should mutate shared workspace metadata, and
owners should administer hosted or organization-level access.
```

For recipient commands, use:

```text
Inviting a recipient records access metadata and includes that recipient in
future encrypted writes. It does not enable role-based command enforcement.
```

For revocation, use:

```text
Revocation removes a recipient from future encrypted writes. It does not delete
previous ciphertext, copied `.env` files, or values a user already decrypted.
```

## Open Questions

- Should direct grants override team grants, or should the most privileged grant
  win? Recommendation: most privileged active grant wins for backward
  compatibility; revisit only for server-side enforcement.
- Should a team member's `Role` cap project-level team access? Recommendation:
  yes. It makes team membership meaningful and avoids a viewer inheriting broad
  mutating access through a team grant.
- Should unknown users be blocked once warnings exist? Recommendation: no for
  the client-side path. Continue with a warning so old single-user manifests
  keep working.
- Should `developer` be allowed to push shared manifests? Recommendation: no by
  default. Developers can mutate local workspace state; maintainers publish
  shared metadata.
- Should `env pull` be available to viewers? Recommendation: yes. The real gate
  is cryptographic decryptability, and viewer is useful for read-only secret
  consumers.
- Should hosted sync enforce roles before per-user tokens exist? Recommendation:
  no. A single bearer token cannot safely represent manifest users.
- Should Plan 013 conflict resolution understand access roles? Recommendation:
  not for initial reconciliation. If server-side enforcement is pursued, combine
  it with Plan 013-style merge rules for users and access records.

## Follow-Up Cards

- Add warning-only `effectiveRole` resolution behind an internal helper and
  table-driven tests; do not wire it to command refusal.
- Add a docs pass replacing permission language with advisory-role wording.
- Add optional CLI warnings for the highest-risk shared mutations:
  `workspace push`, `hosted push`, `project remove`, and recipient changes.
- Design hosted per-user token issuance, rotation, revocation, and audit logs
  before attempting server-side role enforcement.
- Revisit direct-versus-team precedence once real team workflows create
  conflicting grants.
