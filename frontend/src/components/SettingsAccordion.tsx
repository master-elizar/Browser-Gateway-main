import type { ReactNode } from "react";

export function SettingsAccordion({
  id,
  title,
  open,
  onToggle,
  nested = false,
  children,
}: {
  id: string;
  title: string;
  open: boolean;
  onToggle: () => void;
  nested?: boolean;
  children: ReactNode;
}) {
  return (
    <section
      className={
        nested
          ? "overflow-hidden rounded-lg border border-[var(--color-line)]/70 bg-[var(--color-ink)]/30"
          : "overflow-hidden rounded-xl border border-[var(--color-line)] bg-[var(--color-panel)]/70"
      }
    >
      <button
        type="button"
        id={`acc-${id}`}
        aria-expanded={open}
        onClick={onToggle}
        className={`flex w-full items-center justify-between gap-3 text-left transition hover:bg-[var(--color-panel-2)]/40 ${
          nested ? "px-3 py-2" : "px-4 py-3"
        }`}
      >
        <span
          className={
            nested
              ? "text-xs font-medium text-[var(--color-snow)]"
              : "text-sm font-medium uppercase tracking-wider text-[var(--color-fog)]"
          }
        >
          {title}
        </span>
        <span className={`text-[var(--color-fog)] transition-transform ${open ? "rotate-180" : ""}`}>
          ▾
        </span>
      </button>
      {open && (
        <div
          className={
            nested
              ? "border-t border-[var(--color-line)]/60 px-3 py-3"
              : "border-t border-[var(--color-line)] px-4 py-4"
          }
        >
          {children}
        </div>
      )}
    </section>
  );
}
