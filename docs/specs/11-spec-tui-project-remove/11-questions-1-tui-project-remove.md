# 11 Questions Round 1 - TUI Project Remove

Please answer each question below (select one or more options, or add your own notes). Feel free to add additional context under any question.

## 1. TUI Entry Point

How should users start removing the selected project from the TUI?

- [ ] (A) Add the action only to the command palette as `Remove selected project`
- [x] (B) Add a visible keybinding, such as `x`, plus the command palette action
- [ ] (C) Add mouse or row-level affordances in addition to keyboard and palette access
- [ ] (D) Other (describe)

**Current best-practice context:** OpenTUI routes keyboard input through focused components, and the current TUI already centralizes global actions through key handlers plus the command palette. Staying in that pattern keeps focus behavior predictable.

**Recommended answer(s):** [(B)]

**Why these are recommended:**

- `(B)` matches existing TUI actions like scan, plan, apply, and hydrate: a fast key for common use plus palette discoverability.
- `(A)` is safer but makes a core project-management action harder to discover from the main view.
- `(C)` adds more UI surface than this feature needs and should wait until the TUI has broader row-level action patterns.

## 2. Confirmation Behavior

What confirmation should be required before the TUI removes a project?

- [x] (A) Require a confirmation overlay showing project name/path and accept `enter` or `y`
- [ ] (B) Require typing the project name or path exactly before removal
- [ ] (C) No confirmation; remove immediately
- [ ] (D) Other (describe)

**Recommended answer(s):** [(A)]

**Why these are recommended:**

- `(A)` is consistent with the existing apply confirmation overlay and is enough because `project remove` untracks metadata only.
- `(B)` is heavier and better reserved for destructive file deletion, which is not in scope here.
- `(C)` is too easy to trigger accidentally from a keyboard-driven TUI.

## 3. Remove Semantics

What should the TUI remove action do after confirmation?

- [x] (A) Match `devspace project remove`: untrack the project from the manifest, remove related access/state entries, and leave files on disk untouched
- [ ] (B) Also delete the project directory from disk
- [ ] (C) Only remove the project from the current TUI view until the next scan
- [ ] (D) Other (describe)

**Recommended answer(s):** [(A)]

**Why these are recommended:**

- `(A)` reuses the existing domain function and CLI contract, which keeps the TUI a second surface for the same safe operation.
- `(B)` is destructive and would need a separate spec with stronger safeguards.
- `(C)` would make the TUI misleading because the manifest would still track the project.

## 4. Access And Sync Feedback

What feedback should the TUI show around role warnings and sync state after removal?

- [x] (A) Show any existing project-remove access advisory warning in the confirmation or result, then refresh rows and sync status
- [ ] (B) Block removal unless the active user has owner or maintainer access
- [ ] (C) Skip role warning display and only show success/error toast plus refreshed rows
- [ ] (D) Automatically push or reconcile the manifest after removal when sync is configured
- [ ] (E) Other (describe)

**Recommended answer(s):** [(A)]

**Why these are recommended:**

- `(A)` preserves the repository's current advisory-only access model while making shared-metadata risk visible.
- `(B)` would change access warnings into enforcement, which is larger than adding the CLI operation to the TUI.
- `(C)` is simpler but hides an existing safety signal for a mutating metadata operation.
- `(D)` mixes local TUI mutation with remote sync behavior and should remain an explicit user action.

## 5. Proof Artifacts

Which proof artifacts should the SDD workflow require for this feature?

- [x] (A) Go ui-server tests for a `remove` request and refreshed snapshot
- [ ] (B) Bun/TypeScript tests for protocol typing, palette command, and confirmation behavior
- [ ] (C) CLI/TUI transcript or screenshot showing confirmation and successful removal
- [ ] (D) Full manual remote-sync demo after removal
- [x] (E) Other (describe) - also add vhs demo

**Recommended answer(s):** [(A), (B), (C)]

**Why these are recommended:**

- `(A)` proves the mutating backend path reuses existing manifest/state behavior.
- `(B)` proves the visible TUI action, confirmation guard, and protocol contract.
- `(C)` gives a human-readable proof artifact for the actual terminal workflow.
- `(D)` is outside the recommended local-only scope unless automatic sync becomes part of the answer to Question 4.
