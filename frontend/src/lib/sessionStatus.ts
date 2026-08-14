import type { BrowserSession } from "../api/client";

// Was defined once each (near-identically) in SessionsPage.tsx and AdminSessionsPage.tsx --
// the admin version additionally treated "idle" as warn, which the user-facing one didn't;
// unified on the admin version's behavior (idle sessions are worth an admin's attention too).
export function statusTone(status: string): "success" | "warn" | "danger" | "neutral" | "accent" {
  const s = status.toLowerCase();
  if (s.includes("run")) return "success";
  if (s.includes("err") || s.includes("fail")) return "danger";
  if (s.includes("pend") || s.includes("creat") || s.includes("start") || s.includes("idle")) return "warn";
  if (s.includes("stop") || s.includes("end")) return "neutral";
  return "accent";
}

// While any session is mid-transition (booting or tearing down), poll tight enough to
// catch it landing on RUNNING within a few seconds. Once everything's stable, back off --
// a slow keepalive poll still catches server-side drift (admin stop, idle timeout).
export function hasTransientStatus(items: BrowserSession[]): boolean {
  return items.some((s) => {
    const v = s.status.toLowerCase();
    return v.includes("creat") || v.includes("start") || v.includes("stop");
  });
}
