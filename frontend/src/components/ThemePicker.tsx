import { useTranslation } from "react-i18next";
import { useTheme } from "../theme/ThemeContext";
import type { ThemeId } from "../theme/themes";

export function ThemePicker() {
  const { t } = useTranslation();
  const { themeId, themes, setThemeId } = useTheme();

  return (
    <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
      {themes.map((theme) => {
        const active = theme.id === themeId;
        return (
          <button
            key={theme.id}
            type="button"
            onClick={() => setThemeId(theme.id as ThemeId)}
            className={[
              "group rounded-[var(--radius-lg)] border p-3 text-left transition",
              active
                ? "border-[var(--color-signal)] bg-[var(--color-signal-dim)] shadow-[var(--shadow-glow)]"
                : "border-[var(--color-line)] bg-[var(--color-ink)]/40 hover:border-[var(--color-line-strong)] hover:bg-[var(--color-surface-hover)]",
            ].join(" ")}
            aria-pressed={active}
            title={t(theme.nameKey)}
          >
            <div className="mb-3 flex h-14 overflow-hidden rounded-[var(--radius-md)] ring-1 ring-[var(--color-line)]">
              <span className="w-[34%]" style={{ background: theme.swatches[0] }} />
              <span className="w-[33%]" style={{ background: theme.swatches[1] }} />
              <span className="w-[33%]" style={{ background: theme.swatches[2] }} />
            </div>
            <div className="flex items-center justify-between gap-2">
              <div>
                <div className="text-sm font-medium text-[var(--color-snow)]">{t(theme.nameKey)}</div>
                <div className="text-[10px] uppercase tracking-wider text-[var(--color-muted)]">
                  {theme.mode === "light" ? t("theme.modeLight") : t("theme.modeDark")}
                </div>
              </div>
              {active && <span className="badge badge-accent">{t("theme.active")}</span>}
            </div>
          </button>
        );
      })}
    </div>
  );
}
