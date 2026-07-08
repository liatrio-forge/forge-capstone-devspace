# Plan 020: Runtime-validate every devspace-tui RPC response before state updates

> **Executor instructions**: Follow this plan step by step. Run every verification command and confirm the expected result before moving to the next step. If anything in the "STOP conditions" section occurs, stop and report — do not improvise. When done, update the status row for this plan in `plans/README.md`.
>
> **Drift check (run first)**: `git diff --stat cedcbc7..HEAD -- tui/src/client.ts tui/src/protocol.ts tui/test/client.test.ts tui/test/protocol.test.ts tui/test/startup.test.ts`
> If any in-scope file changed since this plan was written, compare the "Current state" excerpts against live code before proceeding; on mismatch, treat it as a STOP condition.

## Status

- **Priority**: P1
- **Effort**: M
- **Risk**: LOW
- **Depends on**: none
- **Category**: bug / tests
- **Planned at**: commit `cedcbc7`, 2026-07-08

## Why this matters

The TypeScript TUI already has runtime validators for every JSON-RPC DTO, but `DevspaceClient` does not use them when resolving responses. A malformed, stale, or version-skewed server response is currently treated as a typed result and can flow into React state before failing somewhere less obvious. This plan wires the existing validators into the single client dispatch point, so protocol drift fails at the boundary with one clear error.

## Current state

Relevant files:

- `tui/src/client.ts` — transport-agnostic NDJSON client. It resolves successful responses directly:

```ts
// tui/src/client.ts:124-130
if (typeof msg.id !== "number") return;
const pending = this.pending.get(msg.id);
if (!pending) return;
this.pending.delete(msg.id);
if (pending.timer) clearTimeout(pending.timer);
if (msg.error) pending.reject(new Error(msg.error.message));
else pending.resolve(msg.result);
```

- `tui/src/protocol.ts` — runtime validators already exist:

```ts
// tui/src/protocol.ts:312-349
export function isHello(v: unknown): v is Hello { ... }
export function isSnapshot(v: unknown): v is Snapshot { ... }
export function isSyncStatus(v: unknown): v is SyncStatus { ... }
```

- `tui/src/protocol.ts` also defines the full method/result map:

```ts
// tui/src/protocol.ts:381-393
export interface RequestMap {
  hello: { params?: undefined; result: Hello };
  projects: { params?: undefined; result: Snapshot };
  scan: { params?: undefined; result: Snapshot };
  refresh: { params?: undefined; result: Snapshot };
  plan: { params?: undefined; result: Snapshot };
  apply: { params?: undefined; result: Snapshot };
  hydrate: { params: { ref: string }; result: Snapshot };
  remove: { params: { ref: string }; result: Snapshot };
  status: { params?: undefined; result: SyncStatus };
  workspace: { params?: undefined; result: WorkspaceOverview };
  lastPlan: { params?: undefined; result: Plan };
}
```

- `tui/src/startup.ts` asks for `hello`, then calls `helloProblem(hello)`; it assumes the resolved value is actually a `Hello`:

```ts
// tui/src/startup.ts:30-38
hello = await raceTimeout(client.request("hello"), timeoutMs);
const problem = helloProblem(hello);
if (problem) throw new Error(problem);
return hello;
```

Repo conventions to match:

- TUI tests use Bun's built-in test runner under `tui/test/*.test.ts`.
- Protocol fixtures live under `tui/test/fixtures/` and are read by `tui/test/protocol.test.ts`.
- Keep validation code boring and explicit; this repo favors small switch statements over generic reflection.

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| TUI typecheck | `cd tui && bun run typecheck` | exit 0, no TypeScript errors |
| TUI tests | `cd tui && bun test` | all tests pass |
| Full TUI gate | `make tui-verify` | exit 0 |

## Scope

**In scope**:

- `tui/src/protocol.ts`
- `tui/src/client.ts`
- `tui/test/client.test.ts`
- `tui/test/protocol.test.ts`
- `tui/test/startup.test.ts` only if needed for a clearer startup error assertion

**Out of scope**:

- Go DTO shape changes in `internal/devspace/ui_server.go`.
- Regenerating protocol fixtures unless a validator test requires a fixture refresh.
- Changing request method names or wire format.
- Adding a schema library; existing handwritten validators are enough.

## Git workflow

- Branch: `advisor/020-tui-runtime-validation`
- Commit message style: conventional commit, e.g. `fix(tui): validate rpc responses at runtime`.
- Do not push or open a PR unless instructed.

## Steps

### Step 1: Add a per-method result parser in `tui/src/protocol.ts`

Add an exported function near `RequestMap`:

```ts
export function parseResult<M extends Method>(method: M, result: unknown): RequestMap[M]["result"] {
  const ok =
    method === "hello" ? isHello(result) :
    method === "projects" || method === "scan" || method === "refresh" || method === "plan" || method === "apply" || method === "hydrate" || method === "remove" ? isSnapshot(result) :
    method === "status" ? isSyncStatus(result) :
    method === "workspace" ? isWorkspaceOverview(result) :
    method === "lastPlan" ? isPlan(result) :
    false;
  if (!ok) throw new Error(`invalid ${method} response from devspace ui-server`);
  return result as RequestMap[M]["result"];
}
```

A `switch` is also fine if lint/typecheck prefer it. Keep `isPlan` unexported unless tests need it; `parseResult` can call it inside the same module.

**Verify**: `cd tui && bun run typecheck` → exit 0.

### Step 2: Store each pending request's method and validate before resolving

In `tui/src/client.ts`:

1. Import `parseResult` from `./protocol`.
2. Add `method: Method` to the `Pending` interface.
3. When creating a pending request, set `method`.
4. In `dispatch`, replace direct `pending.resolve(msg.result)` with:

```ts
try {
  pending.resolve(parseResult(pending.method, msg.result));
} catch (err) {
  pending.reject(err instanceof Error ? err : new Error(String(err)));
}
```

Keep the timer cleanup and pending deletion exactly where they are, so invalid responses do not leave hanging timers.

**Verify**: `cd tui && bun run typecheck` → exit 0.

### Step 3: Add regression tests for invalid successful responses

In `tui/test/client.test.ts`, add tests using the existing `pair()` helper:

- `hello` response missing `workspaceRoot` rejects with `invalid hello response`.
- `status` response missing `conflictCount` rejects with `invalid status response`.
- `projects` response with `rows` not an array rejects with `invalid projects response`.
- A valid response still resolves normally; keep or reuse existing tests.

In `tui/test/protocol.test.ts`, add a direct `parseResult` test if useful:

- valid `hello.json` parses for method `hello`.
- the same object with `protocol` removed throws.

**Verify**: `cd tui && bun test` → all tests pass.

### Step 4: Run the full TUI gate

Run the same TUI verification CI uses.

**Verify**: `make tui-verify` → exit 0.

## Test plan

- Unit tests in `tui/test/client.test.ts` prove malformed successful responses reject at the transport boundary.
- Existing fixture tests in `tui/test/protocol.test.ts` continue proving the Go-generated fixtures match TypeScript validators.
- `make tui-verify` proves typecheck and all Bun tests pass together.

## Done criteria

- [ ] `cd tui && bun run typecheck` exits 0.
- [ ] `cd tui && bun test` exits 0 with the new invalid-response tests passing.
- [ ] `make tui-verify` exits 0.
- [ ] `DevspaceClient` no longer resolves `msg.result` without `parseResult` or equivalent validation.
- [ ] No Go files are modified.
- [ ] No files outside the in-scope list are modified, except `plans/README.md` status.
- [ ] `plans/README.md` row 020 is updated when complete.

## STOP conditions

Stop and report back if:

- The live `RequestMap` differs from the method list in this plan.
- Adding validation requires changing Go DTOs or fixture generation.
- The TUI has intentionally started accepting partial DTOs that fail the existing validators.
- `make tui-verify` fails for unrelated dependency/toolchain reasons after the code-level tests pass.

## Maintenance notes

Future UI-server methods must update `RequestMap`, add or reuse a validator, and update `parseResult`. Reviewers should reject new client methods that bypass runtime validation; otherwise the Go/TS golden fixtures only prove static examples, not live response handling.
