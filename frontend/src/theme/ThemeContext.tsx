import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import {
  ACCENTS,
  applyTheme,
  readStoredAccent,
  readStoredAppearance,
  resolveMode,
  storeAccent,
  storeAppearance,
  type AccentDef,
  type AccentId,
  type Appearance,
  type ResolvedMode,
} from "./themes";

type ThemeCtx = {
  appearance: Appearance;
  accent: AccentId;
  accentDef: AccentDef;
  accents: AccentDef[];
  mode: ResolvedMode;
  isHacker: boolean;
  setAppearance: (a: Appearance) => void;
  setAccent: (id: AccentId) => void;
};

const Ctx = createContext<ThemeCtx | null>(null);

// Avoid FOUC on first paint
applyTheme(readStoredAppearance(), readStoredAccent());

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [appearance, setAppearanceState] = useState<Appearance>(() => readStoredAppearance());
  const [accent, setAccentState] = useState<AccentId>(() => readStoredAccent());

  const setAppearance = useCallback(
    (a: Appearance) => {
      applyTheme(a, accent);
      storeAppearance(a);
      setAppearanceState(a);
    },
    [accent],
  );

  const setAccent = useCallback(
    (id: AccentId) => {
      applyTheme(appearance, id);
      storeAccent(id);
      setAccentState(id);
    },
    [appearance],
  );

  // Live-follow the OS scheme while "auto" is selected.
  useEffect(() => {
    if (appearance !== "auto") return;
    const mq = window.matchMedia("(prefers-color-scheme: dark)");
    const onChange = () => applyTheme("auto", accent);
    mq.addEventListener("change", onChange);
    return () => mq.removeEventListener("change", onChange);
  }, [appearance, accent]);

  const value = useMemo<ThemeCtx>(() => {
    const mode = resolveMode(appearance);
    return {
      appearance,
      accent,
      accentDef: ACCENTS.find((a) => a.id === accent) ?? ACCENTS[0]!,
      accents: ACCENTS,
      mode,
      isHacker: mode === "hacker",
      setAppearance,
      setAccent,
    };
  }, [appearance, accent, setAppearance, setAccent]);

  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}

export function useTheme() {
  const ctx = useContext(Ctx);
  if (!ctx) {
    throw new Error("useTheme must be used within ThemeProvider");
  }
  return ctx;
}
