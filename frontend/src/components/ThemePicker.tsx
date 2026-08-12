import { useTranslation } from "react-i18next";
import { useTheme } from "../theme/ThemeContext";
import { CORNERS, DENSITIES, type AccentId, type Appearance } from "../theme/themes";
import { Segmented } from "./ui";
import { IconCheck, IconTerminal } from "./ui/icons";

const APPEARANCES: Exclude<Appearance, "hacker">[] = ["light", "dark", "auto"];

export function ThemePicker() {
  const { t } = useTranslation();
  const { appearance, accent, accents, corner, density, setAppearance, setAccent, setCorner, setDensity, isHacker } =
    useTheme();

  return (
    <div className="space-y-6">
      <div>
        <div className="field-label mb-2">{t("theme.appearance")}</div>
        <Segmented
          options={APPEARANCES.map((a) => ({ value: a, label: t(`theme.mode.${a}`) }))}
          value={appearance as Exclude<Appearance, "hacker">}
          onChange={setAppearance}
        />
      </div>

      <div className={isHacker ? "opacity-40" : ""}>
        <div className="field-label mb-2">{t("theme.accentLabel")}</div>
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

      <div>
        <div className="field-label mb-2">{t("theme.corner")}</div>
        <Segmented
          options={CORNERS.map((c) => ({ value: c, label: t(`theme.corner${cap(c)}`) }))}
          value={corner}
          onChange={setCorner}
        />
      </div>

      <div>
        <div className="field-label mb-2">{t("theme.density")}</div>
        <Segmented
          options={DENSITIES.map((d) => ({ value: d, label: t(`theme.density${cap(d)}`) }))}
          value={density}
          onChange={setDensity}
        />
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

function cap(s: string): string {
  return s.charAt(0).toUpperCase() + s.slice(1);
}
