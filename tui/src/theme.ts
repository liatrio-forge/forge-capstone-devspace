// Named themes. The default ports the Liatrio brand palette from
// internal/devspace/styles.go so both frontends share one visual identity.

export interface Theme {
  name: string;
  bg: string;
  panel: string;
  border: string;
  borderFocus: string;
  text: string;
  muted: string;
  accent: string;
  ok: string;
  warn: string;
  fail: string;
  selectionBg: string;
  selectionFg: string;
}

export const themes: Theme[] = [
  {
    name: "liatrio-dark",
    bg: "#101418",
    panel: "#161b22",
    border: "#30363d",
    borderFocus: "#89DF00",
    text: "#e6edf3",
    muted: "#9CA3AF",
    accent: "#89DF00",
    ok: "#89DF00",
    warn: "#FBBF24",
    fail: "#EF4343",
    selectionBg: "#233043",
    selectionFg: "#e6edf3",
  },
  {
    name: "liatrio-light",
    bg: "#f6f8fa",
    panel: "#ffffff",
    border: "#d0d7de",
    borderFocus: "#24AE1D",
    text: "#1f2328",
    muted: "#6B7280",
    accent: "#24AE1D",
    ok: "#24AE1D",
    warn: "#D97706",
    fail: "#EF4343",
    selectionBg: "#ddf4ff",
    selectionFg: "#1f2328",
  },
  {
    name: "tokyonight",
    bg: "#1a1b26",
    panel: "#1f2335",
    border: "#3b4261",
    borderFocus: "#7aa2f7",
    text: "#c0caf5",
    muted: "#565f89",
    accent: "#7aa2f7",
    ok: "#9ece6a",
    warn: "#e0af68",
    fail: "#f7768e",
    selectionBg: "#283457",
    selectionFg: "#c0caf5",
  },
];
