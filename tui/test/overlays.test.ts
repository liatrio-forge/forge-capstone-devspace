import { describe, expect, test } from "bun:test";
import { initialState } from "../src/state";
import { paletteCommands, runPaletteCommand } from "../src/overlays";
import type { DashboardState } from "../src/state";

const stateWithRow: DashboardState = {
  ...initialState,
  rows: [{ ref: "apps/api", name: "api", path: "apps/api", type: "local", status: "Hydrated", dirty: false, env: false }],
};

describe("remove command palette action", () => {
  test("offers remove for the selected project", () => {
    const commands = paletteCommands(stateWithRow, "");
    expect(commands).toContainEqual({ id: "remove", label: "Remove selected project", hint: "x" });
  });

  test("opens remove confirmation instead of removing immediately", () => {
    let opened = false;
    let ran = false;
    runPaletteCommand("remove", {
      runAction: () => {
        ran = true;
      },
      selectedRow: stateWithRow.rows[0],
      openRemove: () => {
        opened = true;
      },
      openWorkspace: () => {},
      openHelp: () => {},
      openPlan: () => {},
      cycleTheme: () => {},
      quit: () => {},
    });
    expect(opened).toBe(true);
    expect(ran).toBe(false);
  });
});
