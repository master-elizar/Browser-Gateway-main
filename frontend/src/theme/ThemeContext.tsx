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
  readStoredCorner,
  readStoredDensity,
  resolveMode,
  storeAccent,
  storeAppearance,
  storeCorner,
  storeDensity,
  type AccentDef,
  type AccentId,
  type Appearance,
  type Corner,
  type Density,
  type ResolvedMode,
} from "./themes";

type ThemeCtx = {
  appearance: Appearance;
  accent: AccentId;
  accentDef: AccentDef;
  accents: AccentDef[];
  corner: Corner;
  density: Density;
  mode: ResolvedMode;
  isHacker: boolean;
  setAppearance: (a: Appearance) => void;
  setAccent: (id: AccentId) => void;
  setCorner: (c: Corner) => void;
  setDensity: (d: Density) => void;
};

const Ctx = createContext<ThemeCtx | null>(null);

// Avoid FOUC on first paint
applyTheme(readStoredAppearance(), readStoredAccent(), readStoredCorner(), readStoredDensity());

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [appearance, setAppearanceState] = useState<Appearance>(() => readStoredAppearance());
  const [accent, setAccentState] = useState<AccentId>(() => readStoredAccent());
  const [corner, setCornerState] = useState<Corner>(() => readStoredCorner());
  const [density, setDensityState] = useState<Density>(() => readStoredDensity());

  const setAppearance = useCallback(
    (a: Appearance) => {
      applyTheme(a, accent, corner, density);
      storeAppearance(a);
      setAppearanceState(a);
    },
    [accent, corner, density],
  );

  const setAccent = useCallback(
    (id: AccentId) => {
      applyTheme(appearance, id, corner, density);
      storeAccent(id);
      setAccentState(id);
    },
    [appearance, corner, density],
  );

  const setCorner = useCallback(
    (c: Corner) => {
      applyTheme(appearance, accent, c, density);
      storeCorner(c);
      setCornerState(c);
    },
    [appearance, accent, density],
  );

  const setDensity = useCallback(
    (d: Density) => {
      applyTheme(appearance, accent, corner, d);
      storeDensity(d);
      setDensityState(d);
    },
    [appearance, accent, corner],
  );

  // Live-follow the OS scheme while "auto" is selected.
  useEffect(() => {
    if (appearance !== "auto") return;
    const mq = window.matchMedia("(prefers-color-scheme: dark)");
    const onChange = () => applyTheme("auto", accent, corner, density);
    mq.addEventListener("change", onChange);
    return () => mq.removeEventListener("change", onChange);
  }, [appearance, accent, corner, density]);

  const value = useMemo<ThemeCtx>(() => {
    const mode = resolveMode(appearance);
    return {
      appearance,
      accent,
      accentDef: ACCENTS.find((a) => a.id === accent) ?? ACCENTS[0]!,
      accents: ACCENTS,
      corner,
      density,
      mode,
      isHacker: mode === "hacker",
      setAppearance,
      setAccent,
      setCorner,
      setDensity,
    };
  }, [appearance, accent, corner, density, setAppearance, setAccent, setCorner, setDensity]);

  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}

export function useTheme() {
  const ctx = useContext(Ctx);
  if (!ctx) {
    throw new Error("useTheme must be used within ThemeProvider");
  }
  return ctx;
}
