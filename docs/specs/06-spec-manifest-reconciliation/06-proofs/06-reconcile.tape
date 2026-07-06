Output docs/specs/06-spec-manifest-reconciliation/06-proofs/06-reconcile.gif

Set Width 1000
Set Height 700
Set FontSize 16
Set Shell bash
Set TypingSpeed 40ms

# Render from the repo root:
#   vhs docs/specs/06-spec-manifest-reconciliation/06-proofs/06-reconcile.tape
#
# Two DEVSPACE_HOMEs (machine A and B) share a local bare manifest remote,
# diverge non-conflictingly, then hit the old push/pull dead end and recover
# via `devspace workspace reconcile`. A second divergence produces a genuine
# field conflict, resolved with --force-remote. Setup mechanics are lifted
# verbatim from the spec 06 task-04 demo script into the two hidden wrapper
# scripts alongside this tape (06-reconcile-setup-1.sh, -setup-2.sh). Only
# throwaway temp DEVSPACE_HOMEs are used; the real ~/.devspace is untouched.

Hide
Type "export CLICOLOR_FORCE=1"
Enter
Type "bash docs/specs/06-spec-manifest-reconciliation/06-proofs/06-reconcile-setup-1.sh"
Enter
Sleep 3s
Type "source /tmp/06-reconcile-demo-env.sh"
Enter
Type "clear"
Enter
Sleep 500ms
Show

# Machine B: the old dead end (diverged, non-conflicting changes)
Type "devspace workspace push"
Enter
Sleep 2s

Type "devspace workspace pull"
Enter
Sleep 2s

# Machine B: the new path - review-first reconcile, then apply
Type "devspace workspace reconcile"
Enter
Sleep 3s

Type "devspace workspace reconcile --apply"
Enter
Sleep 3s

Type "devspace workspace push"
Enter
Sleep 2s

Hide
Type "bash docs/specs/06-spec-manifest-reconciliation/06-proofs/06-reconcile-setup-2.sh"
Enter
Sleep 3s
Type "clear"
Enter
Sleep 500ms
Show

# Genuine conflict: A set apps/shared defaultBranch=develop, B set trunk
Type "devspace workspace reconcile"
Enter
Sleep 3s

Type "devspace workspace reconcile --apply"
Enter
Sleep 3s

Type "devspace workspace reconcile --force-remote --apply"
Enter
Sleep 3s
