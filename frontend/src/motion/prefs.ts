const STORAGE_KEY = "bg.motion";
const DENSITY_KEY = "bg.particles";

export const PARTICLE_MIN = 12;
export const PARTICLE_MAX = 160;
export const PARTICLE_DEFAULT = 64;

export function readMotionEnabled(): boolean {
  try {
    const v = localStorage.getItem(STORAGE_KEY);
    if (v === "0" || v === "off" || v === "false") return false;
    if (v === "1" || v === "on" || v === "true") return true;
  } catch {
    /* ignore */
  }
  if (typeof window !== "undefined" && window.matchMedia("(prefers-reduced-motion: reduce)").matches) {
    return false;
  }
  return true;
}

export function storeMotionEnabled(enabled: boolean) {
  try {
    localStorage.setItem(STORAGE_KEY, enabled ? "1" : "0");
  } catch {
    /* ignore */
  }
}

export function clampParticleCount(n: number): number {
  if (!Number.isFinite(n)) return PARTICLE_DEFAULT;
  return Math.max(PARTICLE_MIN, Math.min(PARTICLE_MAX, Math.round(n)));
}

export function readParticleCount(): number {
  try {
    const v = localStorage.getItem(DENSITY_KEY);
    if (v != null && v !== "") return clampParticleCount(Number(v));
  } catch {
    /* ignore */
  }
  return PARTICLE_DEFAULT;
}

export function storeParticleCount(n: number) {
  try {
    localStorage.setItem(DENSITY_KEY, String(clampParticleCount(n)));
  } catch {
    /* ignore */
  }
}

export function applyMotionAttr(enabled: boolean) {
  const root = document.documentElement;
  root.setAttribute("data-motion", enabled ? "on" : "off");
  if (enabled) root.classList.remove("motion-off");
  else root.classList.add("motion-off");
}
