## Relevant Files

| File | Why It Is Relevant |
| --- | --- |
| `internal/devspace/ui_server.go` | Owns the JSON-RPC methods, request validation, single-flight action guard, response DTOs, and ui-server command wiring. |
| `internal/devspace/ui_actions.go` | Owns dashboard action commands that call domain functions under `runLocked`; add the remove command here to reuse existing locking and snapshot behavior. |
| `internal/devspace/ui_model.go` | Defines shared dashboard result messages; may need warning/result fields for remove feedback. |
| `internal/devspace/workspace.go` | Contains `RemoveProject`, the existing domain behavior that must be reused without changing file deletion semantics. |
| `internal/devspace/access_roles.go` | Contains existing advisory warning helpers for `devspace project remove`. |
| `internal/devspace/ui_server_test.go` | Add ui-server remove request, error, warning, snapshot, and on-disk file safety coverage. |
| `internal/devspace/access_roles_test.go` | Existing CLI advisory tests; use as reference and keep passing. |
| `internal/devspace/devspace_test.go` | Existing `RemoveProject` domain tests; use as regression coverage for manifest/access/state behavior. |
| `tui/src/protocol.ts` | Add the `remove` request type and any optional remove warning/result fields returned by ui-server. |
| `tui/src/client.ts` | Transport stays generic, but type changes in `protocol.ts` flow through this client. |
| `tui/src/state.ts` | Add or reuse overlay state for remove confirmation. |
| `tui/src/app.tsx` | Wire the `x` keybinding, confirmation handling, remove request, success/error toast, row refresh, and sync refresh. |
| `tui/src/overlays.tsx` | Add the remove confirmation overlay and command palette entry. |
| `tui/test/protocol.test.ts` | Extend protocol fixture validation if the snapshot/remove response shape gains optional warnings. |
| `tui/test/fixtures/snapshot.json` | Update only if the snapshot fixture intentionally includes remove warnings. |
| `README.md` | Update `devspace ui` docs so the documented TUI action set includes safe project untracking. |
| `docs/specs/11-spec-tui-project-remove/11-tui-project-remove.tape` | VHS script for the sandbox TUI remove proof. |
| `docs/specs/11-spec-tui-project-remove/11-tui-project-remove.gif` | Rendered VHS proof artifact for validation. |

### Notes

- Reuse `RemoveProject`; do not add a second manifest mutation path.
- Keep removal local-only; do not push, pull, reconcile, or call hosted sync.
- Use sandbox data in proofs; do not record real `.env` values, tokens, age identities, or private remotes.
- For Go checks in this sandbox, prefer a temporary `GOCACHE` and exact test names before the broader gates.

## Standards Evidence

| Source File | Read | Standards Extracted | Conflicts |
| --- | --- | --- | --- |
| `AGENTS.md` | yes | Go CLI entrypoint is `cmd/devspace/main.go`; implementation belongs in `internal/devspace/`; tests sit beside code; run `make verify`; do not commit secrets or generated workspace state. | none |
| `README.md` | yes | `devspace ui` is local-first; existing TUI actions are scan, plan, apply-safe, and hydrate; JSON output must be clean; scan refreshes saved project metadata. | none |
| `CONTRIBUTING.md` | not found | n/a | none |
| `.github/pull_request_template.md` | not found | n/a | none |
| `Makefile` | yes | `make verify` runs Go tests, vet, lint, vulncheck, build; `make tui-verify` runs Bun typecheck and tests; `make ci` runs both Go and TUI gates. | none |
| `tui/package.json` | yes | TUI scripts are `bun run typecheck`, `bun test`, and `bun run build`; implementation uses TypeScript/OpenTUI/React. | none |
| `.github/workflows/ci.yml` | yes | CI runs `make verify`, `make tui-verify`, and FUSE integration separately. | none |

## Tasks

### [x] 1.0 Add ui-server Project Remove Request

#### 1.0 Proof Artifact(s)

- Test: `GOCACHE=$(mktemp -d) go test ./internal/devspace -run 'TestUIServer.*Remove|TestRemoveProject' -v` passes and demonstrates TUI removal reuses manifest/access/state removal semantics.
- Test: `GOCACHE=$(mktemp -d) go test ./internal/devspace -run 'TestUIServerErrorPaths' -v` passes and demonstrates blank or missing remove refs return user-facing errors.

#### 1.0 Tasks

- [x] 1.1 Add a `dashboardRemoveCmd(ref string)` in `internal/devspace/ui_actions.go` that runs under `runLocked`, trims/validates the ref, calls `RemoveProject(ref)`, then loads rows and summary with the existing dashboard snapshot helpers.
- [x] 1.2 Extend `uiServerOptions` in `internal/devspace/ui_server.go` with an optional `removeCmd func(string) tea.Cmd` test seam, defaulting to `dashboardRemoveCmd`.
- [x] 1.3 Add a `remove` case to `uiServer.handle` that uses `beginAction("remove")`, invalidates sync status cache, validates `params.ref`, calls the remove command, and returns the standard `uiSnapshot`.
- [x] 1.4 Include the removed `Project` in the response by reusing the existing `Project` field on `uiSnapshot`.
- [x] 1.5 Add ui-server tests proving a tracked project is removed from response rows, its manifest/access/state entries are gone, and the project directory still exists on disk.
- [x] 1.6 Add ui-server error-path coverage for missing, blank, malformed, and unknown project refs.

### [ ] 2.0 Add TUI Remove Entry Points And Confirmation

#### 2.0 Proof Artifact(s)

- Verification: `cd tui && bun run typecheck` passes and demonstrates the protocol/client/TUI remove action is type-valid.
- VHS: `docs/specs/11-spec-tui-project-remove/11-tui-project-remove.gif` shows `x` or command palette opening the remove confirmation overlay for the selected project.

#### 2.0 Tasks

- [ ] 2.1 Add `remove: { params: { ref: string }; result: Snapshot }` to `RequestMap` in `tui/src/protocol.ts`.
- [ ] 2.2 Extend the TUI action method union in `tui/src/app.tsx` to include `remove`, and route it through `client.request("remove", { ref })`.
- [ ] 2.3 Add a `confirm-remove` overlay state carrying the selected `ProjectRow` in `tui/src/state.ts`.
- [ ] 2.4 Add a `ConfirmRemove` overlay in `tui/src/overlays.tsx` that shows project name/path and says files on disk are not touched.
- [ ] 2.5 Add `x` handling in the main app key switch to open `confirm-remove` for the selected row when one exists.
- [ ] 2.6 Add `enter`/`y` handling in the confirmation overlay to call remove for the selected ref, and `esc`/`n`/`q` handling to cancel.
- [ ] 2.7 Add `Remove selected project` to the command palette when a row is selected and route it to the same confirmation overlay.
- [ ] 2.8 Update help overlay and status bar text so `x remove` is visible without changing the no-project empty state.

### [ ] 3.0 Surface Advisory, Result, And Refresh Feedback

#### 3.0 Proof Artifact(s)

- Test: `GOCACHE=$(mktemp -d) go test ./internal/devspace -run 'TestUIServer.*Remove|TestAccessRoleAdvisory' -v` passes and demonstrates access advisory behavior remains visible for project removal.
- VHS: `docs/specs/11-spec-tui-project-remove/11-tui-project-remove.gif` shows the removed project leaving the table, success feedback appearing, and files remaining untouched.

#### 3.0 Tasks

- [ ] 3.1 In the ui-server remove flow, collect `accessRoleAdvisoryWarnings("devspace project remove", ref, AccessRoleOwner, AccessRoleMaintainer)` before calling `RemoveProject`.
- [ ] 3.2 Add an optional `warnings` field to the remove snapshot response only if warnings exist; keep it free of ANSI styling.
- [ ] 3.3 Update `tui/src/protocol.ts` snapshot validation to accept optional `warnings?: string[]`.
- [ ] 3.4 Show advisory warnings in the TUI result flow, either as a warning event line or a compact toast that does not hide success/failure state.
- [ ] 3.5 On successful remove, dispatch the returned snapshot so the removed row disappears and selection is clamped by the existing reducer.
- [ ] 3.6 On successful remove, call `refreshStatus()` and do not call scan, push, pull, reconcile, hosted sync, or watch sync.
- [ ] 3.7 On failed remove, leave rows unchanged and show the existing action error + error toast path.
- [ ] 3.8 Add focused tests or fixture validation for optional snapshot warnings if the response DTO changes.

### [ ] 4.0 Capture Demo And Verification Evidence

#### 4.0 Proof Artifact(s)

- VHS: `docs/specs/11-spec-tui-project-remove/11-tui-project-remove.tape` and `docs/specs/11-spec-tui-project-remove/11-tui-project-remove.gif` demonstrate the end-to-end TUI remove flow with sandbox data.
- Verification: `make tui-verify` passes and demonstrates the TUI typecheck/test gate remains green.
- Verification: `make verify` passes and demonstrates the Go repository gate remains green.

#### 4.0 Tasks

- [ ] 4.1 Update `README.md` so the `devspace ui` section lists project untracking and clearly says files on disk are not touched.
- [ ] 4.2 Create a VHS tape that initializes a temporary `DEVSPACE_HOME` and workspace, adds a sandbox project, starts the TUI with `--no-watch`, removes the project, and verifies the project directory still exists.
- [ ] 4.3 Render the VHS GIF under `docs/specs/11-spec-tui-project-remove/`.
- [ ] 4.4 Save proof notes or command transcripts under `docs/specs/11-spec-tui-project-remove/11-proofs/` with exact commands and sanitized output.
- [ ] 4.5 Run focused Go tests for remove and advisory behavior with a temporary `GOCACHE`.
- [ ] 4.6 Run `make tui-verify`.
- [ ] 4.7 Run `make verify`, or record the exact blocker if the broader sandbox gate cannot complete.
