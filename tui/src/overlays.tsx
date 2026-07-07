import type { ReactNode } from "react";
import type { Plan, PlanAction, ProjectRow } from "./protocol";
import type { DashboardState, Overlay } from "./state";
import type { Theme } from "./theme";

interface OverlayFrameProps {
  th: Theme;
  title: string;
  children: ReactNode;
}

function OverlayFrame({ th, title, children }: OverlayFrameProps) {
  return (
    <box flexGrow={1} padding={1} alignItems="center" justifyContent="center">
      <box
        flexDirection="column"
        border
        borderStyle="rounded"
        borderColor={th.borderFocus}
        title={` ${title} `}
        titleColor={th.accent}
        backgroundColor={th.panel}
        width="80%"
        paddingLeft={2}
        paddingRight={2}
        paddingTop={1}
        paddingBottom={1}
      >
        {children}
      </box>
    </box>
  );
}

const KEYMAP: Array<[string, string]> = [
  ["j / k / ↑ / ↓", "move selection"],
  ["g / G", "first / last project"],
  ["r", "refresh workspace state"],
  ["s", "scan workspace"],
  ["p", "build plan (opens plan view)"],
  ["a", "apply safe actions (confirms first)"],
  ["h", "hydrate selected project (git clone)"],
  ["ctrl+k", "command palette"],
  ["t", "cycle theme"],
  ["mouse", "click selects · wheel scrolls"],
  ["?", "toggle this help"],
  ["esc", "close overlay"],
  ["q / ctrl+c", "quit"],
];

export function HelpOverlay({ th }: { th: Theme; width: number; height: number }) {
  return (
    <OverlayFrame th={th} title="Help">
      {KEYMAP.map(([keys, description]) => (
        <text key={keys}>
          <span fg={th.accent}>{keys.padEnd(18)}</span>
          <span fg={th.text}>{description}</span>
        </text>
      ))}
      <text> </text>
      <text fg={th.muted}>Apply only creates safe placeholder folders — it never deletes or overwrites.</text>
    </OverlayFrame>
  );
}

const SAFETY_ORDER = ["safe", "skipped"];

/** Number of plan lines visible in the overlay for a given terminal height. */
export function planVisibleLines(height: number): number {
  return Math.max(5, height - 12);
}

export function PlanOverlay({ th, overlay, height }: { th: Theme; overlay: Extract<Overlay, { kind: "plan" }>; width: number; height: number }) {
  const plan = overlay.plan;
  const actions = plan.actions ?? [];
  const warnings = plan.warnings ?? [];
  const grouped = SAFETY_ORDER.flatMap((safety) => actions.filter((a) => a.safety === safety)).concat(
    actions.filter((a) => !SAFETY_ORDER.includes(a.safety)),
  );
  const safeCount = actions.filter((a) => a.safety === "safe").length;
  const visible = planVisibleLines(height);
  const lines: Array<{ text: string; color: string }> = [
    ...grouped.map((action) => ({ text: planActionLine(action), color: actionColor(th, action) })),
    ...warnings.map((warning) => ({ text: `⚠ ${warning}`, color: th.warn })),
  ];
  const window = lines.slice(overlay.scroll, overlay.scroll + visible);

  return (
    <OverlayFrame th={th} title={`Plan · ${plan.generatedAt}`}>
      <text>
        <span fg={th.ok}>{`${safeCount} safe`}</span>
        <span fg={th.muted}>{` · ${actions.length - safeCount} skipped · ${warnings.length} warnings`}</span>
      </text>
      <text> </text>
      {window.length === 0 ? (
        <text fg={th.muted}>plan is empty — workspace matches the manifest</text>
      ) : (
        window.map((line, i) => (
          <text key={i} fg={line.color}>
            {line.text}
          </text>
        ))
      )}
      {lines.length > visible ? <text fg={th.muted}>… {overlay.scroll + window.length}/{lines.length} (j/k to scroll)</text> : null}
      <text> </text>
      <text fg={th.muted}>a apply safe actions · esc close</text>
    </OverlayFrame>
  );
}

function planActionLine(action: PlanAction): string {
  const marker = action.safety === "safe" ? "+" : "·";
  const reason = action.reason ? `  (${action.reason})` : "";
  return `${marker} ${action.kind.padEnd(14)} ${action.path}${reason}`;
}

function actionColor(th: Theme, action: PlanAction): string {
  if (action.safety === "safe") return th.ok;
  if (action.reason?.includes("dirty")) return th.warn;
  return th.muted;
}

export function ConfirmApply({ th, plan }: { th: Theme; plan: Plan; width: number; height: number }) {
  const safeCount = (plan.actions ?? []).filter((a) => a.safety === "safe").length;
  return (
    <OverlayFrame th={th} title="Apply plan?">
      <text fg={th.text}>
        Apply <strong fg={th.ok}>{`${safeCount}`}</strong> safe action{safeCount === 1 ? "" : "s"} (create placeholder folders only)?
      </text>
      <text fg={th.muted}>Nothing is ever deleted or overwritten.</text>
      <text> </text>
      <text>
        <span fg={th.ok}>enter/y</span>
        <span fg={th.muted}> apply · </span>
        <span fg={th.fail}>esc/n</span>
        <span fg={th.muted}> cancel</span>
      </text>
    </OverlayFrame>
  );
}

export interface PaletteCommand {
  id: string;
  label: string;
  hint?: string;
}

export function paletteCommands(state: DashboardState, query: string): PaletteCommand[] {
  const selected = state.rows[state.selected];
  const commands: PaletteCommand[] = [
    { id: "refresh", label: "Refresh workspace", hint: "r" },
    { id: "scan", label: "Scan workspace", hint: "s" },
    { id: "plan", label: "Build plan", hint: "p" },
    { id: "apply", label: "Apply safe actions", hint: "a" },
    ...(selected ? [{ id: "hydrate", label: `Hydrate ${selected.name}`, hint: "h" }] : []),
    ...(state.lastPlan ? [{ id: "show-plan", label: "Show last plan" }] : []),
    { id: "theme", label: "Cycle theme", hint: "t" },
    { id: "help", label: "Help", hint: "?" },
    { id: "quit", label: "Quit", hint: "q" },
  ];
  const q = query.trim().toLowerCase();
  if (!q) return commands;
  // ponytail: subsequence match is fuzzy enough for ~9 commands
  return commands.filter((command) => {
    const label = command.label.toLowerCase();
    let at = 0;
    for (const ch of q) {
      at = label.indexOf(ch, at);
      if (at < 0) return false;
      at++;
    }
    return true;
  });
}

export function runPaletteCommand(
  id: string,
  ctx: {
    runAction: (method: "scan" | "refresh" | "plan" | "apply" | "hydrate", ref?: string) => void;
    selectedRow?: ProjectRow;
    openHelp: () => void;
    openPlan: () => void;
    cycleTheme: () => void;
    quit: () => void;
  },
): void {
  switch (id) {
    case "refresh":
    case "scan":
    case "plan":
    case "apply":
      return ctx.runAction(id);
    case "hydrate":
      if (ctx.selectedRow) ctx.runAction("hydrate", ctx.selectedRow.ref);
      return;
    case "show-plan":
      return ctx.openPlan();
    case "theme":
      return ctx.cycleTheme();
    case "help":
      return ctx.openHelp();
    case "quit":
      return ctx.quit();
  }
}

export function Palette({
  th,
  overlay,
  state,
  onQuery,
}: {
  th: Theme;
  overlay: Extract<Overlay, { kind: "palette" }>;
  state: DashboardState;
  onQuery: (query: string) => void;
  width: number;
  height: number;
}) {
  const commands = paletteCommands(state, overlay.query);
  const selected = Math.min(overlay.selected, Math.max(0, commands.length - 1));
  return (
    <box flexGrow={1} padding={1} alignItems="center">
      <box
        flexDirection="column"
        border
        borderStyle="rounded"
        borderColor={th.borderFocus}
        title=" Command Palette "
        titleColor={th.accent}
        backgroundColor={th.panel}
        width="60%"
      >
        <input focused placeholder="type a command…" onInput={onQuery} />
        {commands.length === 0 ? (
          <box paddingLeft={1}>
            <text fg={th.muted}>no matching commands</text>
          </box>
        ) : (
          commands.map((command, i) => (
            <box key={command.id} paddingLeft={1} paddingRight={1} backgroundColor={i === selected ? th.selectionBg : undefined}>
              <text>
                <span fg={i === selected ? th.accent : th.text}>{command.label.padEnd(32)}</span>
                <span fg={th.muted}>{command.hint ?? ""}</span>
              </text>
            </box>
          ))
        )}
        <box paddingLeft={1}>
          <text fg={th.muted}>↑/↓ choose · enter run · esc close</text>
        </box>
      </box>
    </box>
  );
}
