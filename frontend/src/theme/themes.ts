export type Appearance = "light" | "dark" | "auto" | "hacker";
export type ResolvedMode = "light" | "dark" | "hacker";

export type AccentId =
  | "blue"
  | "purple"
  | "pink"
  | "red"
  | "orange"
  | "yellow"
  | "green"
  | "graphite";

export type AccentDef = {
  id: AccentId;
  /** i18n key under theme.accent.* */
  nameKey: string;
  swatch: string;
  vars: Record<string, string>;
};

const APPEARANCE_KEY = "bg.appearance";
const ACCENT_KEY = "bg.accent";

export const ACCENTS: AccentDef[] = [
  {
    id: "blue",
    nameKey: "theme.accent.blue",
    swatch: "#0a84ff",
    vars: {
      "--color-signal": "#0a84ff",
      "--color-signal-2": "#0a84ff",
      "--color-signal-dim": "rgba(10, 132, 255, 0.13)",
    },
  },
  {
    id: "purple",
    nameKey: "theme.accent.purple",
    swatch: "#bf5af2",
    vars: {
      "--color-signal": "#bf5af2",
      "--color-signal-2": "#bf5af2",
      "--color-signal-dim": "rgba(191, 90, 242, 0.14)",
    },
  },
  {
    id: "pink",
    nameKey: "theme.accent.pink",
    swatch: "#ff375f",
    vars: {
      "--color-signal": "#ff375f",
      "--color-signal-2": "#ff375f",
      "--color-signal-dim": "rgba(255, 55, 95, 0.13)",
    },
  },
  {
    id: "red",
    nameKey: "theme.accent.red",
    swatch: "#ff453a",
    vars: {
      "--color-signal": "#ff453a",
      "--color-signal-2": "#ff453a",
      "--color-signal-dim": "rgba(255, 69, 58, 0.13)",
    },
  },
  {
    id: "orange",
    nameKey: "theme.accent.orange",
    swatch: "#ff9500",
    vars: {
      "--color-signal": "#ff9500",
      "--color-signal-2": "#ff9500",
      "--color-signal-dim": "rgba(255, 149, 0, 0.15)",
    },
  },
  {
    id: "yellow",
    nameKey: "theme.accent.yellow",
    swatch: "#ffcc00",
    vars: {
      "--color-signal": "#ffcc00",
      // Pure yellow fails as label/icon text on both light and dark — darken for legibility.
      "--color-signal-2": "#a17400",
      "--color-signal-dim": "rgba(255, 204, 0, 0.18)",
    },
  },
  {
    id: "green",
    nameKey: "theme.accent.green",
    swatch: "#30d158",
    vars: {
      "--color-signal": "#30d158",
      "--color-signal-2": "#1f8a3e",
      "--color-signal-dim": "rgba(48, 209, 88, 0.14)",
    },
  },
  {
    id: "graphite",
    nameKey: "theme.accent.graphite",
    swatch: "#8e8e93",
    vars: {
      "--color-signal": "#8e8e93",
      "--color-signal-2": "#6e6e73",
      "--color-signal-dim": "rgba(142, 142, 147, 0.16)",
    },
  },
];

const LIGHT_VARS: Record<string, string> = {
  "--color-bg": "#f5f5f7",
  "--color-bg-elevated": "#ffffff",
  "--color-ink": "#ffffff",
  "--color-panel": "#ffffff",
  "--color-panel-2": "#f5f5f7",
  "--color-panel-3": "#ebebf0",
  "--color-surface": "#ffffff",
  "--color-surface-hover": "#f0f0f3",
  "--color-line": "rgba(0, 0, 0, 0.09)",
  "--color-line-strong": "rgba(0, 0, 0, 0.16)",
  "--color-fog": "#6e6e73",
  "--color-muted": "#8e8e93",
  "--color-snow": "#1d1d1f",
  "--color-white": "#000000",
  "--color-success": "#248a3d",
  "--color-success-dim": "rgba(52, 199, 89, 0.14)",
  "--color-warn": "#b25000",
  "--color-warn-dim": "rgba(255, 149, 0, 0.14)",
  "--color-danger": "#d70015",
  "--color-danger-dim": "rgba(255, 59, 48, 0.13)",
  "--color-info": "#0a84ff",
  "--color-info-dim": "rgba(10, 132, 255, 0.13)",
  "--shadow-sm": "0 1px 2px rgba(0, 0, 0, 0.04), 0 1px 1px rgba(0, 0, 0, 0.03)",
  "--shadow-md": "0 16px 32px rgba(0, 0, 0, 0.09), 0 2px 8px rgba(0, 0, 0, 0.05)",
  "--theme-selection": "rgba(10, 132, 255, 0.22)",
};

const DARK_VARS: Record<string, string> = {
  "--color-bg": "#000000",
  "--color-bg-elevated": "#1c1c1e",
  "--color-ink": "#000000",
  "--color-panel": "#1c1c1e",
  "--color-panel-2": "#2c2c2e",
  "--color-panel-3": "#3a3a3c",
  "--color-surface": "#1c1c1e",
  "--color-surface-hover": "#2c2c2e",
  "--color-line": "rgba(255, 255, 255, 0.1)",
  "--color-line-strong": "rgba(255, 255, 255, 0.18)",
  "--color-fog": "#98989d",
  "--color-muted": "#8e8e93",
  "--color-snow": "#f5f5f7",
  "--color-white": "#ffffff",
  "--color-success": "#30d158",
  "--color-success-dim": "rgba(48, 209, 88, 0.16)",
  "--color-warn": "#ff9f0a",
  "--color-warn-dim": "rgba(255, 159, 10, 0.16)",
  "--color-danger": "#ff453a",
  "--color-danger-dim": "rgba(255, 69, 58, 0.16)",
  "--color-info": "#0a84ff",
  "--color-info-dim": "rgba(10, 132, 255, 0.16)",
  "--shadow-sm": "0 0 0 1px rgba(255, 255, 255, 0.05)",
  "--shadow-md": "0 20px 40px rgba(0, 0, 0, 0.55), 0 2px 10px rgba(0, 0, 0, 0.4)",
  "--theme-selection": "rgba(10, 132, 255, 0.35)",
};

const HACKER_VARS: Record<string, string> = {
  "--color-bg": "#04060a",
  "--color-bg-elevated": "#070b0f",
  "--color-ink": "#04060a",
  "--color-panel": "#070b0f",
  "--color-panel-2": "#0b1117",
  "--color-panel-3": "#101922",
  "--color-surface": "#080d12",
  "--color-surface-hover": "#0d141b",
  "--color-line": "rgba(51, 255, 153, 0.18)",
  "--color-line-strong": "rgba(51, 255, 153, 0.34)",
  "--color-fog": "#7fdba0",
  "--color-muted": "#4f8f68",
  "--color-snow": "#baffcf",
  "--color-white": "#eafff0",
  "--color-signal": "#33ff99",
  "--color-signal-2": "#33ff99",
  "--color-signal-dim": "rgba(51, 255, 153, 0.16)",
  "--color-success": "#33ff99",
  "--color-success-dim": "rgba(51, 255, 153, 0.16)",
  "--color-warn": "#ffe066",
  "--color-warn-dim": "rgba(255, 224, 102, 0.16)",
  "--color-danger": "#ff5c5c",
  "--color-danger-dim": "rgba(255, 92, 92, 0.16)",
  "--color-info": "#5ce1ff",
  "--color-info-dim": "rgba(92, 225, 255, 0.16)",
  "--shadow-sm": "0 0 0 1px rgba(51, 255, 153, 0.12)",
  "--shadow-md": "0 20px 40px rgba(0, 0, 0, 0.6), 0 0 30px rgba(51, 255, 153, 0.08)",
  "--theme-selection": "rgba(51, 255, 153, 0.35)",
};

export function getAccent(id: string | null | undefined): AccentDef {
  return ACCENTS.find((a) => a.id === id) ?? ACCENTS[0]!;
}

export function readStoredAppearance(): Appearance {
  try {
    const v = localStorage.getItem(APPEARANCE_KEY);
    if (v === "light" || v === "dark" || v === "auto" || v === "hacker") return v;
  } catch {
    /* ignore */
  }
  return "auto";
}

export function storeAppearance(a: Appearance) {
  try {
    localStorage.setItem(APPEARANCE_KEY, a);
  } catch {
    /* ignore */
  }
}

export function readStoredAccent(): AccentId {
  try {
    const v = localStorage.getItem(ACCENT_KEY);
    if (ACCENTS.some((a) => a.id === v)) return v as AccentId;
  } catch {
    /* ignore */
  }
  return "blue";
}

export function storeAccent(id: AccentId) {
  try {
    localStorage.setItem(ACCENT_KEY, id);
  } catch {
    /* ignore */
  }
}

export function systemPrefersDark(): boolean {
  return typeof window !== "undefined" && window.matchMedia("(prefers-color-scheme: dark)").matches;
}

export function resolveMode(appearance: Appearance): ResolvedMode {
  if (appearance === "hacker") return "hacker";
  if (appearance === "auto") return systemPrefersDark() ? "dark" : "light";
  return appearance;
}

/** Apply the resolved theme's CSS variables before React paint (also used by provider). */
export function applyTheme(appearance: Appearance, accentId: AccentId) {
  const root = document.documentElement;
  const mode = resolveMode(appearance);
  root.setAttribute("data-appearance", appearance);
  root.setAttribute("data-mode", mode);
  root.setAttribute("data-accent", accentId);

  const base = mode === "light" ? LIGHT_VARS : mode === "dark" ? DARK_VARS : HACKER_VARS;
  for (const [key, value] of Object.entries(base)) {
    root.style.setProperty(key, value);
  }
  if (mode !== "hacker") {
    for (const [key, value] of Object.entries(getAccent(accentId).vars)) {
      root.style.setProperty(key, value);
    }
  }
}
