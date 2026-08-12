const STORAGE_KEY = "bg.motion";

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

export function applyMotionAttr(enabled: boolean) {
  const root = document.documentElement;
  root.setAttribute("data-motion", enabled ? "on" : "off");
  if (enabled) root.classList.remove("motion-off");
  else root.classList.add("motion-off");
}
