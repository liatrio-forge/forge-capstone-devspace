import type { Plan, ProjectRow, ScanSummary, ServerEvent, Snapshot, SyncStatus, WorkspaceOverview } from "./protocol";

export const EVENT_LIMIT = 50;

export type Overlay =
  | { kind: "none" }
  | { kind: "help" }
  | { kind: "palette"; query: string; selected: number }
  | { kind: "plan"; plan: Plan; scroll: number }
  | { kind: "workspace"; overview: WorkspaceOverview }
  | { kind: "confirm-apply"; plan: Plan }
  | { kind: "confirm-remove"; row: ProjectRow };

export interface Toast {
  id: number;
  tone: "ok" | "error";
  text: string;
}

export interface DashboardState {
  rows: ProjectRow[];
  summary: ScanSummary;
  sync?: SyncStatus;
  selected: number;
  events: string[];
  busy?: string; // label of the in-flight action
  error?: string;
  watchAlive: boolean;
  overlay: Overlay;
  toasts: Toast[];
  lastPlan?: Plan;
}

export const initialState: DashboardState = {
  rows: [],
  summary: { foundProjects: 0, gitRepos: 0, untrackedFolders: 0, localOnlyProjects: 0, projectsWithEnv: 0 },
  selected: 0,
  events: [],
  watchAlive: true,
  overlay: { kind: "none" },
  toasts: [],
};

export type Action =
  | { type: "snapshot"; label: string; snapshot: Snapshot }
  | { type: "status"; status: SyncStatus }
  | { type: "server-event"; event: ServerEvent }
  | { type: "action-start"; label: string }
  | { type: "action-error"; label: string; message: string }
  | { type: "event"; message: string }
  | { type: "move"; delta: number }
  | { type: "select"; index: number }
  | { type: "overlay"; overlay: Overlay }
  | { type: "toast"; toast: Toast }
  | { type: "toast-expire"; id: number };

const clampSelected = (state: DashboardState): DashboardState => ({
  ...state,
  selected: state.rows.length === 0 ? 0 : Math.min(Math.max(state.selected, 0), state.rows.length - 1),
});

function pushEvent(events: string[], event: string): string[] {
  return [event, ...events].slice(0, EVENT_LIMIT);
}

function watchEventText(event: Extract<ServerEvent, { type: "watch-refresh" }>): string {
  const scope = event.refresh.fullScan ? "full" : "scoped";
  return `${scope} refresh at ${event.refresh.refreshStartedAt ?? "-"}: found=${event.summary.foundProjects} git=${event.summary.gitRepos} dirs=${event.refresh.watchedDirCount}`;
}

export function reduce(state: DashboardState, action: Action): DashboardState {
  switch (action.type) {
    case "snapshot": {
      const { snapshot, label } = action;
      const next: DashboardState = {
        ...state,
        rows: snapshot.rows ?? [],
        summary: snapshot.summary,
        busy: undefined,
        error: undefined,
        events: pushEvent(state.events, `${label} complete`),
        lastPlan: snapshot.plan ?? state.lastPlan,
      };
      return clampSelected(next);
    }
    case "status":
      return { ...state, sync: action.status };
    case "server-event": {
      const event = action.event;
      if (event.type === "watch-error") {
        return {
          ...state,
          watchAlive: false,
          events: pushEvent(state.events, `watch stopped: ${event.message}`),
        };
      }
      if (event.type === "watch-refresh") {
        return clampSelected({
          ...state,
          rows: event.rows ?? [],
          summary: event.summary,
          watchAlive: true,
          events: pushEvent(state.events, watchEventText(event)),
        });
      }
      return state;
    }
    case "action-start":
      return { ...state, busy: action.label, error: undefined };
    case "action-error":
      return {
        ...state,
        busy: undefined,
        error: action.message,
        events: pushEvent(state.events, `${action.label} failed`),
      };
    case "event":
      return { ...state, events: pushEvent(state.events, action.message) };
    case "move":
      return clampSelected({ ...state, selected: state.selected + action.delta });
    case "select":
      return clampSelected({ ...state, selected: action.index });
    case "overlay":
      return { ...state, overlay: action.overlay };
    case "toast":
      return { ...state, toasts: [...state.toasts, action.toast].slice(-3) };
    case "toast-expire":
      return { ...state, toasts: state.toasts.filter((t) => t.id !== action.id) };
  }
}
