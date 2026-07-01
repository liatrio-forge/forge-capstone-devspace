# Playbook Contribution: Client Enablement Lessons

## Principle

AI-native delivery works best when the team uses specs to narrow risk before
agents write code. The point is not to make the agent faster at every possible
task. The point is to make the desired behavior, forbidden behavior, and proof
standard clear enough that fast code generation does not create hidden risk.

## DevDrop Lessons

1. Name the dangerous default.

   For DevDrop, the dangerous default was "sync the whole workspace." Naming
   that risk forced the product toward metadata sync, explicit hydration, and
   local encrypted env profiles.

2. Put non-goals in the spec.

   Excluding hosted sync, source-code syncing, dependency auto-install, and team
   secret sharing kept the MVP small enough to finish and safe enough to demo.

3. Make mutation two-step when trust matters.

   `plan` and `apply` split review from write operations. This pattern is useful
   for client tools that touch files, infrastructure, database records, or
   production state.

4. Test the refusal paths.

   The most important tests are not the happy path. They prove that invalid
   manifests, path traversal, stale plans, non-empty hydrate destinations, and
   local unpushed changes are rejected.

5. Keep demos offline when possible.

   A local bare Git remote makes the capstone demo repeatable without network
   accounts, hosted infrastructure, or flaky external services.

6. Make agent orchestration inspectable.

   The `wave-ship` workflow is useful because each agent card has a title,
   scope, dependencies, PR, verification output, and merge status. That is the
   difference between "AI helped" and a delivery system a client can review.

## How To Introduce This Workflow To A Traditional Team

Start with a small, bounded product slice. Write the spec as a contract, not a
brainstorm. Include user stories, non-goals, safety boundaries, and acceptance
tests. Let agents implement against that contract, then review output through
tests, diffs, and a release-readiness note.

The adoption message is simple: AI can compress implementation time, but the
team still owns the product contract and the evidence bar.
