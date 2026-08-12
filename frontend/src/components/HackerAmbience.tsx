import { useEffect, useMemo, useState } from "react";
import { useMotion } from "../motion/MotionContext";
import { useTheme } from "../theme/ThemeContext";

const HEX_CHARS = "0123456789ABCDEF";

function randomHex(len: number): string {
  let out = "";
  for (let i = 0; i < len; i++) out += HEX_CHARS[Math.floor(Math.random() * 16)];
  return out;
}

function randomToken(): string {
  const kinds = [
    () => `0x${randomHex(4)}`,
    () => randomHex(8),
    () => `${randomHex(2)}:${randomHex(2)}:${randomHex(2)}:${randomHex(2)}`,
    () => `${Math.floor(Math.random() * 256)}.${Math.floor(Math.random() * 256)}.${Math.floor(Math.random() * 256)}.${Math.floor(Math.random() * 256)}`,
  ];
  return kinds[Math.floor(Math.random() * kinds.length)]!();
}

// Generated once per mount, not per frame -- it's a static texture, not an animation.
function generateNoiseBlock(rows: number, cols: number): string {
  const lines: string[] = [];
  for (let r = 0; r < rows; r++) {
    let line = "";
    for (let c = 0; c < cols; c++) line += HEX_CHARS[Math.floor(Math.random() * 16)] + (c % 4 === 3 ? " " : "");
    lines.push(line);
  }
  return lines.join("\n");
}

/**
 * Hacker-mode-only ambience, replacing the old always-on particle canvas. Two cheap,
 * non-canvas techniques instead of per-frame redraw:
 *  - a static hex texture + a single CSS transform animation (GPU-composited, no JS timer)
 *  - a small corner readout whose text is replaced a couple of times a second (not
 *    requestAnimationFrame) -- a handful of text-node writes, not a live render loop.
 * Both respect the motion toggle: with motion off, the texture stays but nothing moves or
 * updates.
 */
export function HackerAmbience() {
  const { isHacker } = useTheme();
  const { motionEnabled } = useMotion();

  const noise = useMemo(() => generateNoiseBlock(40, 48), [isHacker]);
  const [hudLines, setHudLines] = useState<string[]>(() => Array.from({ length: 5 }, () => randomToken()));

  useEffect(() => {
    if (!isHacker || !motionEnabled) return;
    const id = window.setInterval(() => {
      setHudLines((prev) => {
        const next = [...prev];
        next[Math.floor(Math.random() * next.length)] = randomToken();
        return next;
      });
    }, 550);
    return () => window.clearInterval(id);
  }, [isHacker, motionEnabled]);

  if (!isHacker) return null;

  return (
    <div aria-hidden className="pointer-events-none fixed inset-0 z-0 overflow-hidden">
      <pre
        className="absolute inset-0 select-none whitespace-pre font-mono text-[10px] leading-[14px] text-[var(--color-signal)]"
        style={{ opacity: 0.055 }}
      >
        {noise}
      </pre>
      {motionEnabled && <div className="hacker-scan" />}
      <div className="absolute bottom-4 right-4 select-none rounded-[var(--radius-sm)] border border-[var(--color-line)] bg-[var(--color-bg-elevated)]/40 px-3 py-2 font-mono text-[10px] leading-[15px] text-[var(--color-signal)] opacity-40">
        {hudLines.map((line, i) => (
          <div key={i}>{line}</div>
        ))}
      </div>
    </div>
  );
}
