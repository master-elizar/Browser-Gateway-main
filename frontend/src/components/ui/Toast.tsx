import { createContext, useCallback, useContext, useMemo, useState, type ReactNode } from "react";
import { IconCheck, IconClose } from "./icons";

type ToastTone = "info" | "success" | "warn" | "danger";
type ToastItem = { id: number; message: string; tone: ToastTone };

type ToastCtx = {
  push: (message: string, tone?: ToastTone) => void;
};

const Ctx = createContext<ToastCtx | null>(null);

export function ToastProvider({ children }: { children: ReactNode }) {
  const [items, setItems] = useState<ToastItem[]>([]);

  const push = useCallback((message: string, tone: ToastTone = "info") => {
    const id = Date.now() + Math.random();
    setItems((prev) => [...prev, { id, message, tone }]);
    window.setTimeout(() => {
      setItems((prev) => prev.filter((t) => t.id !== id));
    }, 3800);
  }, []);

  const value = useMemo(() => ({ push }), [push]);

  return (
    <Ctx.Provider value={value}>
      {children}
      <div className="pointer-events-none fixed bottom-6 right-6 z-[100] flex w-[360px] flex-col gap-2">
        {items.map((t) => (
          <div
            key={t.id}
            className={`toast pointer-events-auto flex items-start gap-2 border-l-2 ${
              t.tone === "success"
                ? "border-l-[var(--color-success)]"
                : t.tone === "warn"
                  ? "border-l-[var(--color-warn)]"
                  : t.tone === "danger"
                    ? "border-l-[var(--color-danger)]"
                    : "border-l-[var(--color-info)]"
            }`}
          >
            <span className="mt-0.5 text-[var(--color-fog)]">
              {t.tone === "success" ? <IconCheck size={16} /> : null}
            </span>
            <span className="flex-1 text-[var(--color-snow)]">{t.message}</span>
            <button
              type="button"
              className="btn-icon"
              onClick={() => setItems((prev) => prev.filter((x) => x.id !== t.id))}
            >
              <IconClose size={14} />
            </button>
          </div>
        ))}
      </div>
    </Ctx.Provider>
  );
}

export function useToast() {
  const ctx = useContext(Ctx);
  if (!ctx) {
    return { push: (_m: string, _t?: ToastTone) => undefined };
  }
  return ctx;
}
