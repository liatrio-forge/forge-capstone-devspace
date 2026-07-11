import { useTerminalDimensions, useKeyboard } from "@opentui/react";
import { useEffect, useReducer, useRef, useState } from "react";
import type { DevspaceClient } from "./client";
import type { Hello, ProjectRow, Snapshot } from "./protocol";
import { initialState, reduce, type DashboardState } from "./state";
import { cell } from "./text";
import { themes, type Theme } from "./theme";
import { ConfirmApply, ConfirmRemove, HelpOverlay, Palette, PlanOverlay, WorkspaceOverlay, paletteCommands, planVisibleLines, runPaletteCommand } from "./overlays";

const SPINNER = ["⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"];

type ActionMethod = "scan" | "refresh" | "plan" | "apply" | "hydrate" | "remove";

export interface AppProps {
  client: DevspaceClient;
  hello: Hello;
  quit: (message?: string) => void;
}

export function App({ client, hello, quit }: AppProps) {
  const [state, dispatch] = useReducer(reduce, initialState);
  const [themeIndex, setThemeIndex] = useState(0);
  const { width, height } = useTerminalDimensions();
  const th = themes[themeIndex % themes.length]!;

  const stateRef = useRef(state);
  stateRef.current = state;
  const heightRef = useRef(height);
  heightRef.current = height;
  const toastSeq = useRef(1);
  const busyRef = useRef(false);

  function addToast(tone: "ok" | "error", text: string) {
    const id = toastSeq.current++;
    dispatch({ type: "toast", toast: { id, tone, text } });
    setTimeout(() => dispatch({ type: "toast-expire", id }), 3500);
  }

  function refreshStatus() {
    client.request("status").then(
      (status) => dispatch({ type: "status", status }),
      () => {},
    );
  }

  function openWorkspace() {
    client.request("workspace").then(
      (overview) => dispatch({ type: "overlay", overlay: { kind: "workspace", overview } }),
      (err: Error) => addToast("error", `workspace failed: ${err.message}`),
    );
  }

  function runAction(method: ActionMethod, ref?: string) {
    if (busyRef.current || stateRef.current.busy) {
      addToast("error", "busy; wait for the current operation");
      return;
    }
    const label = method === "apply" ? "apply-safe" : method;
    busyRef.current = true;
    dispatch({ type: "action-start", label });
    const req: Promise<Snapshot> =
      method === "hydrate" || method === "remove" ? client.request(method, { ref: ref ?? "" }) : client.request(method);
    req.then(
      (snapshot) => {
        busyRef.current = false;
        dispatch({ type: "snapshot", label, snapshot });
        for (const warning of snapshot.warnings ?? []) dispatch({ type: "event", message: warning });
        addToast("ok", `${label} complete`);
        if (method === "plan" && snapshot.plan) {
          dispatch({ type: "overlay", overlay: { kind: "plan", plan: snapshot.plan, scroll: 0 } });
        }
        refreshStatus();
      },
      (err: Error) => {
        busyRef.current = false;
        dispatch({ type: "action-error", label, message: err.message });
        addToast("error", `${label} failed`);
      },
    );
  }

  function openRemove(row?: ProjectRow) {
    const target = row ?? stateRef.current.rows[stateRef.current.selected];
    if (target) dispatch({ type: "overlay", overlay: { kind: "confirm-remove", row: target } });
  }

  useEffect(() => {
    runAction("scan");
    refreshStatus();
    const offEvent = client.onEvent((event) => {
      dispatch({ type: "server-event", event });
      if (event.type === "watch-refresh") refreshStatus();
    });
    const offClose = client.onClose((err) => quit(err ? err.message : "devspace ui-server exited"));
    return () => {
      offEvent();
      offClose();
    };
  }, []);

  useKeyboard((key) => {
    const s = stateRef.current;
    const overlay = s.overlay;

    if (key.ctrl && key.name === "c") return quit();

    if (overlay.kind !== "none") {
      if (key.name === "escape") return dispatch({ type: "overlay", overlay: { kind: "none" } });
      if (overlay.kind === "help" && (key.name === "q" || key.name === "?")) {
        return dispatch({ type: "overlay", overlay: { kind: "none" } });
      }
      if (overlay.kind === "workspace" && key.name === "q") {
        return dispatch({ type: "overlay", overlay: { kind: "none" } });
      }
      if (overlay.kind === "plan") {
        const totalLines = (overlay.plan.actions?.length ?? 0) + (overlay.plan.warnings?.length ?? 0);
        const max = Math.max(0, totalLines - planVisibleLines(heightRef.current));
        if (key.name === "j" || key.name === "down") {
          return dispatch({ type: "overlay", overlay: { ...overlay, scroll: Math.min(overlay.scroll + 1, max) } });
        }
        if (key.name === "k" || key.name === "up") {
          return dispatch({ type: "overlay", overlay: { ...overlay, scroll: Math.max(overlay.scroll - 1, 0) } });
        }
        if (key.name === "a") return dispatch({ type: "overlay", overlay: { kind: "confirm-apply", plan: overlay.plan } });
        if (key.name === "q") return dispatch({ type: "overlay", overlay: { kind: "none" } });
        return;
      }
      if (overlay.kind === "confirm-apply") {
        if (key.name === "return" || key.name === "y") {
          dispatch({ type: "overlay", overlay: { kind: "none" } });
          return runAction("apply");
        }
        if (key.name === "n" || key.name === "q") return dispatch({ type: "overlay", overlay: { kind: "none" } });
        return;
      }
      if (overlay.kind === "confirm-remove") {
        if (key.name === "return" || key.name === "y") {
          dispatch({ type: "overlay", overlay: { kind: "none" } });
          return runAction("remove", overlay.row.ref);
        }
        if (key.name === "n" || key.name === "q") return dispatch({ type: "overlay", overlay: { kind: "none" } });
        return;
      }
      if (overlay.kind === "palette") {
        const commands = paletteCommands(s, overlay.query);
        if (key.name === "up" || (key.ctrl && key.name === "p")) {
          return dispatch({ type: "overlay", overlay: { ...overlay, selected: Math.max(0, overlay.selected - 1) } });
        }
        if (key.name === "down" || (key.ctrl && key.name === "n")) {
          return dispatch({
            type: "overlay",
            overlay: { ...overlay, selected: Math.min(Math.max(0, commands.length - 1), overlay.selected + 1) },
          });
        }
        if (key.name === "return") {
          const command = commands[Math.min(overlay.selected, commands.length - 1)];
          dispatch({ type: "overlay", overlay: { kind: "none" } });
          if (command) {
            runPaletteCommand(command.id, {
              runAction,
              selectedRow: s.rows[s.selected],
              openWorkspace,
              openHelp: () => dispatch({ type: "overlay", overlay: { kind: "help" } }),
              openRemove,
              openPlan: () =>
                s.lastPlan && dispatch({ type: "overlay", overlay: { kind: "plan", plan: s.lastPlan, scroll: 0 } }),
              cycleTheme: () => setThemeIndex((i) => i + 1),
              quit,
            });
          }
          return;
        }
        return; // remaining keys go to the palette <input>
      }
      return;
    }

    if (key.ctrl && key.name === "k") {
      return dispatch({ type: "overlay", overlay: { kind: "palette", query: "", selected: 0 } });
    }
    switch (key.name) {
      case "q":
        return quit();
      case "j":
      case "down":
        return dispatch({ type: "move", delta: 1 });
      case "k":
      case "up":
        return dispatch({ type: "move", delta: -1 });
      case "g":
        return dispatch({ type: "select", index: key.shift ? s.rows.length - 1 : 0 });
      case "r":
        return runAction("refresh");
      case "w":
        return openWorkspace();
      case "s":
        return runAction("scan");
      case "p":
        return runAction("plan");
      case "a":
        if (s.lastPlan) return dispatch({ type: "overlay", overlay: { kind: "confirm-apply", plan: s.lastPlan } });
        return runAction("plan");
      case "h": {
        const row = s.rows[s.selected];
        if (row) runAction("hydrate", row.ref);
        return;
      }
      case "x":
        return openRemove();
      case "t":
        return setThemeIndex((i) => i + 1);
      case "?":
        return dispatch({ type: "overlay", overlay: { kind: "help" } });
    }
  });

  const overlayProps = { th, width, height };
  return (
    <box flexDirection="column" width="100%" height="100%" backgroundColor={th.bg}>
      <Header th={th} hello={hello} state={state} />
      {state.overlay.kind === "help" ? (
        <HelpOverlay {...overlayProps} />
      ) : state.overlay.kind === "workspace" ? (
        <WorkspaceOverlay {...overlayProps} overview={state.overlay.overview} />
      ) : state.overlay.kind === "plan" ? (
        <PlanOverlay {...overlayProps} overlay={state.overlay} />
      ) : state.overlay.kind === "confirm-apply" ? (
        <ConfirmApply {...overlayProps} plan={state.overlay.plan} />
      ) : state.overlay.kind === "confirm-remove" ? (
        <ConfirmRemove {...overlayProps} row={state.overlay.row} />
      ) : state.overlay.kind === "palette" ? (
        <Palette
          {...overlayProps}
          overlay={state.overlay}
          state={state}
          onQuery={(query) => dispatch({ type: "overlay", overlay: { kind: "palette", query, selected: 0 } })}
        />
      ) : (
        <box flexDirection="row" flexGrow={1} gap={1} padding={1}>
          <ProjectTable th={th} state={state} height={height} onSelect={(index) => dispatch({ type: "select", index })} />
          <box flexDirection="column" width={38} gap={1}>
            <SyncPanel th={th} state={state} />
            <EventsPanel th={th} state={state} height={height} />
          </box>
        </box>
      )}
      <Toasts th={th} state={state} />
      <StatusBar th={th} state={state} hello={hello} />
    </box>
  );
}

function Header({ th, hello, state }: { th: Theme; hello?: Hello; state: DashboardState }) {
  const s = state.summary;
  return (
    <box flexDirection="row" paddingLeft={1} paddingRight={1} backgroundColor={th.panel} justifyContent="space-between">
      <text>
        <strong fg={th.accent}>DevSpace</strong>
        <span fg={th.muted}>  {hello?.workspaceRoot ?? "…"}</span>
        <span fg={th.muted}>  ·  {hello ? `${hello.machineName} (${hello.machineId})` : ""}</span>
      </text>
      <text fg={th.muted}>
        projects {s.foundProjects} · git {s.gitRepos} · local {s.localOnlyProjects} · env {s.projectsWithEnv}
      </text>
    </box>
  );
}

const STATUS_COLOR: Record<string, keyof Theme> = {
  Hydrated: "ok",
  Placeholder: "warn",
  Missing: "fail",
};

function ProjectTable({
  th,
  state,
  height,
  onSelect,
}: {
  th: Theme;
  state: DashboardState;
  height: number;
  onSelect: (index: number) => void;
}) {
  // Manual windowing keeps the selected row visible without relying on
  // scrollbox internals. Rows are one line each.
  const visible = Math.max(3, height - 8);
  const start = Math.max(0, Math.min(state.selected - Math.floor(visible / 2), state.rows.length - visible));
  const rows = state.rows.slice(start, start + visible);

  return (
    <box
      flexDirection="column"
      flexGrow={1}
      border
      borderStyle="rounded"
      borderColor={th.border}
      title=" Projects "
      titleColor={th.accent}
      backgroundColor={th.panel}
      onMouseScroll={(event: { scroll?: { direction: string } }) => {
        if (event.scroll?.direction === "up") onSelect(Math.max(0, state.selected - 1));
        if (event.scroll?.direction === "down") onSelect(Math.min(state.rows.length - 1, state.selected + 1));
      }}
    >
      <box flexDirection="row" paddingLeft={1} paddingRight={1}>
        <text fg={th.muted}>{cell("Project", 28)}{cell("Type", 7)}{cell("Status", 13)}{cell("Dirty", 7)}{cell("Branch", 14)}{cell("Env", 4)}</text>
      </box>
      {rows.length === 0 ? (
        <box paddingLeft={1}>
          <text fg={th.muted}>no projects tracked — press s to scan</text>
        </box>
      ) : (
        rows.map((row, i) => (
          <Row key={row.ref} th={th} row={row} selected={start + i === state.selected} onMouseDown={() => onSelect(start + i)} />
        ))
      )}
      {state.rows.length > visible ? (
        <box paddingLeft={1}>
          <text fg={th.muted}>… {state.selected + 1}/{state.rows.length}</text>
        </box>
      ) : null}
    </box>
  );
}

function Row({ th, row, selected, onMouseDown }: { th: Theme; row: ProjectRow; selected: boolean; onMouseDown: () => void }) {
  const statusColor = th[STATUS_COLOR[row.status] ?? "muted"] as string;
  return (
    <box
      flexDirection="row"
      paddingLeft={1}
      paddingRight={1}
      backgroundColor={selected ? th.selectionBg : undefined}
      onMouseDown={onMouseDown}
    >
      <text>
        <span fg={selected ? th.selectionFg : th.text}>{cell(row.name, 28)}</span>
        <span fg={th.muted}>{cell(row.type, 7)}</span>
        <span fg={statusColor}>{cell(row.status, 13)}</span>
        <span fg={row.dirty ? th.warn : th.ok}>{cell(row.dirty ? "dirty" : "clean", 7)}</span>
        <span fg={th.muted}>{cell(row.branch ?? "-", 14)}</span>
        <span fg={row.env ? th.ok : th.muted}>{cell(row.env ? "✓" : "-", 4)}</span>
      </text>
    </box>
  );
}

function SyncPanel({ th, state }: { th: Theme; state: DashboardState }) {
  const sync = state.sync;
  return (
    <box
      flexDirection="column"
      border
      borderStyle="rounded"
      borderColor={th.border}
      title=" Sync "
      titleColor={th.accent}
      backgroundColor={th.panel}
      paddingLeft={1}
      paddingRight={1}
    >
      {!sync ? (
        <text fg={th.muted}>loading…</text>
      ) : sync.unavailableReason === "remote not configured" ? (
        <text fg={th.muted}>remote not configured</text>
      ) : sync.unavailableReason ? (
        <text fg={th.warn}>unavailable: {sync.unavailableReason}</text>
      ) : (
        <>
          <text fg={th.muted}>last sync {sync.lastSyncAt || "-"}</text>
          {sync.gitDiffUnavailable ? (
            <text fg={th.muted}>diff {sync.gitDiffUnavailable}</text>
          ) : (
            <text>
              <span fg={sync.localDiffers ? th.warn : th.ok}>{sync.localDiffers ? "local differs" : "in sync"}</span>
              <span fg={th.muted}>  +{sync.diffAdded} -{sync.diffRemoved} ~{sync.diffChanged}</span>
            </text>
          )}
          <text fg={sync.conflictCount > 0 ? th.fail : th.muted}>
            conflicts {sync.reconcileSaved ? sync.conflictCount : "-"}
          </text>
        </>
      )}
    </box>
  );
}

function EventsPanel({ th, state, height }: { th: Theme; state: DashboardState; height: number }) {
  const visible = Math.max(3, height - 14);
  return (
    <box
      flexDirection="column"
      flexGrow={1}
      border
      borderStyle="rounded"
      borderColor={th.border}
      title=" Events "
      titleColor={th.accent}
      backgroundColor={th.panel}
      paddingLeft={1}
      paddingRight={1}
    >
      {state.events.length === 0 ? (
        <text fg={th.muted}>none</text>
      ) : (
        state.events.slice(0, visible).map((event, i) => (
          <text key={i} fg={i === 0 ? th.text : th.muted}>
            {event}
          </text>
        ))
      )}
    </box>
  );
}

function Toasts({ th, state }: { th: Theme; state: DashboardState }) {
  if (state.toasts.length === 0) return null;
  return (
    <box flexDirection="column" paddingLeft={1} paddingRight={1}>
      {state.toasts.map((toast) => (
        <text key={toast.id} fg={toast.tone === "ok" ? th.ok : th.fail}>
          {toast.tone === "ok" ? "✓" : "✗"} {toast.text}
        </text>
      ))}
    </box>
  );
}

function StatusBar({ th, state, hello }: { th: Theme; state: DashboardState; hello?: Hello }) {
  const [frame, setFrame] = useState(0);
  useEffect(() => {
    if (!state.busy) return;
    const timer = setInterval(() => setFrame((f) => f + 1), 80);
    return () => clearInterval(timer);
  }, [state.busy]);

  const watch = hello?.watch === false ? "watch off" : state.watchAlive ? "● watching" : "○ watch stopped";
  return (
    <box flexDirection="row" paddingLeft={1} paddingRight={1} backgroundColor={th.panel} justifyContent="space-between">
      <text>
        {state.busy ? (
          <span fg={th.accent}>{SPINNER[frame % SPINNER.length]} {state.busy}…</span>
        ) : state.error ? (
          <span fg={th.fail}>✗ {state.error}</span>
        ) : (
          <span fg={th.ok}>ready</span>
        )}
        <span fg={state.watchAlive && hello?.watch !== false ? th.ok : th.muted}>  {watch}</span>
      </text>
      <text fg={th.muted}>j/k move · w workspace · s scan · p plan · a apply · h hydrate · x remove · ctrl+k palette · ? help · q quit</text>
    </box>
  );
}
