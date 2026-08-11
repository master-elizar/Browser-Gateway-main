import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import {
  applyTheme,
  readStoredThemeId,
  storeThemeId,
  THEMES,
  type ThemeDef,
  type ThemeId,
} from "./themes";

type ThemeCtx = {
  themeId: ThemeId;
  theme: ThemeDef;
  themes: ThemeDef[];
  setThemeId: (id: ThemeId) => void;
};

const Ctx = createContext<ThemeCtx | null>(null);

// Avoid FOUC on first paint
applyTheme(readStoredThemeId());

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [themeId, setThemeIdState] = useState<ThemeId>(() => readStoredThemeId());

  const setThemeId = useCallback((id: ThemeId) => {
    applyTheme(id);
    storeThemeId(id);
    setThemeIdState(id);
  }, []);

  const value = useMemo(
    () => ({
      themeId,
      theme: getThemeSafe(themeId),
      themes: THEMES,
      setThemeId,
    }),
    [themeId, setThemeId],
  );

  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}

function getThemeSafe(id: ThemeId): ThemeDef {
  return THEMES.find((t) => t.id === id) ?? THEMES[0]!;
}

export function useTheme() {
  const ctx = useContext(Ctx);
  if (!ctx) {
    throw new Error("useTheme must be used within ThemeProvider");
  }
  return ctx;
}
