import { useTranslation } from "react-i18next";
import { useMotion } from "../motion/MotionContext";
import { PARTICLE_MAX, PARTICLE_MIN } from "../motion/prefs";
import { Toggle } from "./ui";

export function MotionToggle() {
  const { t } = useTranslation();
  const { motionEnabled, setMotionEnabled, particleCount, setParticleCount } = useMotion();

  return (
    <div className="space-y-3">
      <div className="rounded-[var(--radius-md)] border border-[var(--color-line)] bg-[var(--color-ink)]/40 px-3 py-2">
        <Toggle
          checked={motionEnabled}
          onChange={() => setMotionEnabled(!motionEnabled)}
          label={t("motion.enable")}
          description={t("motion.hint")}
        />
      </div>

      <div
        className={`rounded-[var(--radius-md)] border border-[var(--color-line)] bg-[var(--color-ink)]/40 px-4 py-3 transition ${
          motionEnabled ? "opacity-100" : "opacity-45"
        }`}
      >
        <div className="mb-2 flex items-end justify-between gap-3">
          <div>
            <div className="text-sm font-medium text-[var(--color-snow)]">{t("motion.particles")}</div>
            <div className="text-xs text-[var(--color-muted)]">{t("motion.particlesHint")}</div>
          </div>
          <div className="font-mono text-lg font-semibold tabular-nums text-[var(--color-signal-2)]">
            {particleCount}
          </div>
        </div>

        <input
          type="range"
          min={PARTICLE_MIN}
          max={PARTICLE_MAX}
          step={5}
          value={particleCount}
          disabled={!motionEnabled}
          onChange={(e) => setParticleCount(Number(e.target.value))}
          className="w-full accent-[var(--color-signal)] disabled:cursor-not-allowed"
          aria-label={t("motion.particles")}
        />

        <div className="mt-1.5 flex justify-between font-mono text-[10px] text-[var(--color-muted)]">
          <span>{PARTICLE_MIN}</span>
          <span>{Math.round((PARTICLE_MIN + PARTICLE_MAX) / 2)}</span>
          <span>{PARTICLE_MAX}</span>
        </div>

        <div className="mt-3 flex flex-wrap gap-2">
          {[
            { label: t("motion.presetLow"), value: 28 },
            { label: t("motion.presetMed"), value: 64 },
            { label: t("motion.presetHigh"), value: 100 },
            { label: t("motion.presetMax"), value: PARTICLE_MAX },
          ].map((p) => (
            <button
              key={p.value}
              type="button"
              disabled={!motionEnabled}
              onClick={() => setParticleCount(p.value)}
              className={`rounded-full border px-3 py-1 text-xs transition ${
                particleCount === p.value
                  ? "border-[var(--color-signal)] bg-[var(--color-signal-dim)] text-[var(--color-snow)]"
                  : "border-[var(--color-line)] text-[var(--color-fog)] hover:border-[var(--color-line-strong)] hover:text-[var(--color-snow)]"
              } disabled:cursor-not-allowed disabled:opacity-50`}
            >
              {p.label}
            </button>
          ))}
        </div>
      </div>
    </div>
  );
}
