import { useTranslation } from "react-i18next";
import { useTheme } from "../theme/ThemeContext";
import type { AccentId, Appearance } from "../theme/themes";
import { IconCheck, IconTerminal } from "./ui/icons";

const APPEARANCES: Exclude<Appearance, "hacker">[] = ["light", "dark", "auto"];

export function ThemePicker() {
  const { t } = useTranslation();
  const { appearance, accent, accents, setAppearance, setAccent, isHacker } = useTheme();

  return (
    <div className="space-y-5">
      <div>
        <div className="field-label mb-2">{t("theme.appearance")}</div>
        <div className="inline-flex rounded-[var(--radius-md)] border border-[var(--color-line)] bg-[var(--color-panel-2)] p-1">
          {APPEARANCES.map((a) => {
            const active = appearance === a;
            return (
              <button
                key={a}
                type="button"
                onClick={() => setAppearance(a)}
                aria-pressed={active}
                className={[
                  "rounded-[calc(var(--radius-md)-4px)] px-4 py-1.5 text-sm font-medium transition",
                  active
                    ? "bg-[var(--color-panel)] text-[var(--color-snow)] shadow-[var(--shadow-sm)]"
                    : "text-[var(--color-fog)] hover:text-[var(--color-snow)]",
                ].join(" ")}
              >
                {t(`theme.mode.${a}`)}
              </button>
            );
          })}
        </div>
      </div>

      <div className={isHacker ? "opacity-40" : ""}>
        <div className="field-label mb-2">{t("theme.accent")}</div>
        <div className="flex flex-wrap gap-2.5">
          {accents.map((a) => {
            const active = !isHacker && accent === a.id;
            return (
              <button
                key={a.id}
                type="button"
                disabled={isHacker}
                onClick={() => setAccent(a.id as AccentId)}
                aria-pressed={active}
                title={t(a.nameKey)}
                className="grid size-9 shrink-0 place-items-center rounded-full ring-1 ring-inset ring-[var(--color-line)] transition disabled:cursor-not-allowed"
                style={{ background: a.swatch }}
              >
                {active && <IconCheck size={16} className="text-white drop-shadow" />}
              </button>
            );
          })}
        </div>
      </div>

      <button
        type="button"
        onClick={() => setAppearance(isHacker ? "auto" : "hacker")}
        aria-pressed={isHacker}
        className={[
          "flex w-full items-center gap-3 rounded-[var(--radius-md)] border px-4 py-3 text-left transition",
          isHacker
            ? "border-[var(--color-signal)] bg-[var(--color-signal-dim)]"
            : "border-[var(--color-line)] bg-[var(--color-panel-2)] hover:border-[var(--color-line-strong)]",
        ].join(" ")}
      >
        <span
          className={[
            "grid size-9 shrink-0 place-items-center rounded-[var(--radius-sm)]",
            isHacker ? "bg-[var(--color-signal)] text-black" : "bg-[var(--color-panel-3)] text-[var(--color-fog)]",
          ].join(" ")}
        >
          <IconTerminal size={18} />
        </span>
        <span className="min-w-0 flex-1">
          <span className="block text-sm font-medium text-[var(--color-snow)]">{t("theme.hacker")}</span>
          <span className="mt-0.5 block text-xs text-[var(--color-muted)]">{t("theme.hackerHint")}</span>
        </span>
        {isHacker && <span className="badge badge-accent shrink-0">{t("theme.active")}</span>}
      </button>
    </div>
  );
}
