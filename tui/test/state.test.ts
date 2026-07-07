import { describe, expect, test } from "bun:test";
import { EVENT_LIMIT, initialState, reduce, type DashboardState } from "../src/state";
import type { ProjectRow, ScanSummary, Snapshot } from "../src/protocol";

const summary: ScanSummary = { foundProjects: 2, gitRepos: 1, untrackedFolders: 0, localOnlyProjects: 1, projectsWithEnv: 1 };

const rows: ProjectRow[] = [
  { ref: "apps/api", name: "api", path: "apps/api", type: "git", status: "Hydrated", dirty: true, branch: "main", env: true },
  { ref: "services/worker", name: "worker", path: "services/worker", type: "local", status: "Hydrated", dirty: false, env: false },
];

const snapshot: Snapshot = { rows, summary };

describe("reduce", () => {
  test("snapshot clears busy/error, logs event, keeps lastPlan", () => {
    const busyState: DashboardState = { ...initialState, busy: "scan", error: "old" };
    const next = reduce(busyState, { type: "snapshot", label: "scan", snapshot });
    expect(next.rows).toHaveLength(2);
    expect(next.busy).toBeUndefined();
    expect(next.error).toBeUndefined();
    expect(next.events[0]).toBe("scan complete");
    expect(next.lastPlan).toBeUndefined();

    const plan = { version: 1, workspaceRoot: "/w", manifestHash: "h", generatedAt: "t", actions: [], warnings: [] };
    const withPlan = reduce(next, { type: "snapshot", label: "plan", snapshot: { ...snapshot, plan } });
    expect(withPlan.lastPlan).toEqual(plan);
    // Later snapshots without a plan keep the last one.
    expect(reduce(withPlan, { type: "snapshot", label: "scan", snapshot }).lastPlan).toEqual(plan);
  });

  test("selection moves and clamps", () => {
    let state = reduce(initialState, { type: "snapshot", label: "scan", snapshot });
    state = reduce(state, { type: "move", delta: 1 });
    expect(state.selected).toBe(1);
    state = reduce(state, { type: "move", delta: 5 });
    expect(state.selected).toBe(1);
    state = reduce(state, { type: "move", delta: -9 });
    expect(state.selected).toBe(0);
    // Shrinking rows clamps selection.
    state = reduce(state, { type: "move", delta: 1 });
    const shrunk = reduce(state, { type: "snapshot", label: "scan", snapshot: { rows: [rows[0]!], summary } });
    expect(shrunk.selected).toBe(0);
  });

  test("watch refresh updates rows and logs; watch error marks watcher dead", () => {
    const refreshed = reduce(initialState, {
      type: "server-event",
      event: { type: "watch-refresh", rows, summary, refresh: { fullScan: true, watchedDirCount: 4, syncChanged: false, refreshStartedAt: "t1" } },
    });
    expect(refreshed.rows).toHaveLength(2);
    expect(refreshed.events[0]).toContain("full refresh at t1");
    expect(refreshed.watchAlive).toBe(true);

    const dead = reduce(refreshed, { type: "server-event", event: { type: "watch-error", message: "boom" } });
    expect(dead.watchAlive).toBe(false);
    expect(dead.events[0]).toBe("watch stopped: boom");
  });

  test("action error records message and event", () => {
    const state = reduce({ ...initialState, busy: "hydrate" }, { type: "action-error", label: "hydrate", message: "no remote" });
    expect(state.busy).toBeUndefined();
    expect(state.error).toBe("no remote");
    expect(state.events[0]).toBe("hydrate failed");
  });

  test("event log is capped", () => {
    let state = initialState;
    for (let i = 0; i < EVENT_LIMIT + 10; i++) {
      state = reduce(state, { type: "snapshot", label: `a${i}`, snapshot });
    }
    expect(state.events).toHaveLength(EVENT_LIMIT);
    expect(state.events[0]).toBe(`a${EVENT_LIMIT + 9} complete`);
  });

  test("toasts append, cap at 3, and expire by id", () => {
    let state = initialState;
    for (let id = 1; id <= 4; id++) {
      state = reduce(state, { type: "toast", toast: { id, tone: "ok", text: `t${id}` } });
    }
    expect(state.toasts.map((t) => t.id)).toEqual([2, 3, 4]);
    state = reduce(state, { type: "toast-expire", id: 3 });
    expect(state.toasts.map((t) => t.id)).toEqual([2, 4]);
  });
});
