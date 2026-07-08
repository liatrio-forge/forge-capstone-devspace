# 11-spec-tui-project-remove.md

## Introduction/Overview

DevSpace already supports `devspace project remove` in the CLI, but the TUI only exposes project viewing, scan, plan, apply, hydrate, sync, and workspace actions. This feature adds a safe TUI path for removing the selected project from manifest tracking while preserving the existing CLI semantics: metadata is updated, related access/state entries are removed, and files on disk are not touched.

## Goals

- Add a TUI remove action for the selected project.
- Require confirmation before the TUI performs the removal.
- Reuse the existing `RemoveProject` domain behavior and project-remove access advisory model.
- Refresh the TUI rows and sync status after a successful removal.
- Capture proof with ui-server tests and a VHS demo.

## User Stories

- **As a DevSpace TUI user**, I want to remove the selected project from tracking without leaving the dashboard so that I can manage the workspace manifest from the same surface where I inspect projects.
- **As a user sharing workspace metadata**, I want the TUI to show access advisory warnings before or during removal so that I understand when the action may affect shared inventory.
- **As a user with local project files**, I want removal to leave files on disk untouched so that untracking a project does not destroy work.

## Demoable Units of Work

### Unit 1: Backend Remove Request

**Purpose:** Give the TUI client a JSON-RPC action that performs the same operation as `devspace project remove`.

**Functional Requirements:**

- The system shall add a `remove` request to `devspace ui-server`.
- The request shall require a non-empty project reference.
- The request shall call the existing project removal path so manifest, access, and state behavior matches `devspace project remove`.
- The request shall not delete project files or directories from disk.
- The response shall include an updated project snapshot after successful removal.

**Proof Artifacts:**

- Test: Go ui-server test for `remove` with a tracked project demonstrates the project disappears from rows and remains on disk.
- Test: Go ui-server test for missing or blank refs demonstrates validation errors.

### Unit 2: TUI Remove Action And Confirmation

**Purpose:** Let users invoke removal from the selected project row with a keyboard shortcut and command palette action.

**Functional Requirements:**

- The TUI shall expose a visible `x` keybinding for removing the selected project.
- The TUI shall expose a command palette action named `Remove selected project` when a project is selected.
- The TUI shall show a confirmation overlay with the selected project name and path before sending the remove request.
- The confirmation overlay shall accept `enter` or `y` to remove, and `esc`, `n`, or `q` to cancel.
- The TUI shall do nothing when remove is invoked without a selected row.

**Proof Artifacts:**

- VHS: recorded demo demonstrates the `x` keybinding or command palette opens the confirmation overlay.
- Verification: `make tui-verify` passes, demonstrating the TypeScript protocol and TUI code remain valid.

### Unit 3: Advisory And Refresh Feedback

**Purpose:** Make the mutating metadata action visible and consistent with existing local-first sync behavior.

**Functional Requirements:**

- The TUI shall show any existing project-remove access advisory warning in the confirmation or result flow.
- The TUI shall show a success toast after removal and an error toast if removal fails.
- The TUI shall refresh project rows from the updated snapshot after removal.
- The TUI shall refresh sync status after successful removal.
- The TUI shall not automatically push, pull, reconcile, or contact hosted sync after removal.

**Proof Artifacts:**

- Test: Go ui-server test demonstrates warnings, when present, are available to the TUI remove flow.
- VHS: recorded demo demonstrates success feedback and refreshed project rows after removal.

### Unit 4: VHS Demo

**Purpose:** Provide a human-readable proof artifact for the actual terminal workflow.

**Functional Requirements:**

- The demo shall show a workspace with at least one tracked project in the TUI.
- The demo shall show removal started with `x` or the command palette.
- The demo shall show the confirmation overlay.
- The demo shall show the project removed from the table and a success indication.
- The demo shall avoid real credentials, private remotes, and real `.env` values.

**Proof Artifacts:**

- VHS: recorded demo demonstrates confirmation and successful TUI project removal.

## Non-Goals (Out of Scope)

1. **No file deletion:** this feature will not delete project directories, Git repos, files, or env files.
2. **No automatic sync:** this feature will not push, pull, reconcile, or contact hosted sync after removal.
3. **No access enforcement change:** this feature will keep the current advisory-only access model and will not block removal based on role.
4. **No row-level mouse controls:** this feature will not add clickable per-row action buttons or menus.
5. **No undo system:** users can re-add or rescan projects through existing commands, but this feature will not add a dedicated undo flow.

## Design Considerations

The remove action should follow existing TUI interaction patterns: single-key actions in the main view, command palette discoverability, and confirmation overlays for mutating actions. The confirmation copy must clearly state that files on disk are not touched. Help text and the status bar should include the `x` shortcut without crowding the existing compact layout.

## Repository Standards

- DevSpace is a Go CLI with command wiring and TUI server behavior in `internal/devspace/`.
- The external TUI lives in `tui/` and uses OpenTUI React with TypeScript.
- Reuse existing dashboard command and snapshot patterns instead of adding a separate removal abstraction.
- Keep tests next to implementation: Go tests in `internal/devspace/*_test.go`, TUI tests in `tui/test/*.test.ts`.
- Use existing local-first command behavior: mutating local metadata should not imply remote sync.
- Run focused Go tests, TUI tests, and the repository verification gate before completion.

## Technical Considerations

The implementation should extend the existing JSON-RPC protocol with the smallest useful shape: a `remove` method that accepts `{ "ref": "<project>" }` and returns the same snapshot shape used by scan, refresh, plan, apply, and hydrate. OpenTUI keyboard handling should stay in the focused app-level handler and command palette flow already used by the dashboard. Current OpenTUI guidance emphasizes focused components for keyboard input; this spec follows the current app pattern rather than adding a new focus model.

## Security Considerations

- The action mutates workspace metadata and should surface existing advisory warnings for `devspace project remove`.
- Removal must not print, record, or commit plaintext env values, hosted sync tokens, age identities, or private credentials.
- VHS and test fixtures must use sandbox projects and placeholder remotes.
- The TUI must not perform remote sync automatically, because that would turn a local metadata action into a remote state change.

## Success Metrics

1. **TUI action works:** selecting a project and confirming remove untracks it from the manifest and removes it from the table.
2. **Files remain safe:** the removed project's directory remains on disk after TUI removal.
3. **Existing semantics are preserved:** CLI and TUI removal share the same manifest/access/state behavior.
4. **Feedback is visible:** confirmation, advisory warning when applicable, success/error toast, and refreshed sync status are visible.
5. **Proof passes:** Go ui-server tests and the VHS demo artifact demonstrate the workflow.

## Open Questions

No open questions at this time.
