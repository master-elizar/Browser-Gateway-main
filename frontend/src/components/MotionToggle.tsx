import { useTranslation } from "react-i18next";
import { useMotion } from "../motion/MotionContext";
import { Toggle } from "./ui";

export function MotionToggle() {
  const { t } = useTranslation();
  const { motionEnabled, setMotionEnabled } = useMotion();

  return (
    <div className="rounded-[var(--radius-md)] border border-[var(--color-line)] bg-[var(--color-panel-2)] px-3 py-2">
      <Toggle
        checked={motionEnabled}
        onChange={() => setMotionEnabled(!motionEnabled)}
        label={t("motion.enable")}
        description={t("motion.hint")}
      />
    </div>
  );
}
