import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { applyMotionAttr, readMotionEnabled, storeMotionEnabled } from "./prefs";

type MotionCtx = {
  motionEnabled: boolean;
  setMotionEnabled: (on: boolean) => void;
};

const Ctx = createContext<MotionCtx | null>(null);

applyMotionAttr(readMotionEnabled());

export function MotionProvider({ children }: { children: ReactNode }) {
  const [motionEnabled, setState] = useState(() => readMotionEnabled());

  const setMotionEnabled = useCallback((on: boolean) => {
    applyMotionAttr(on);
    storeMotionEnabled(on);
    setState(on);
  }, []);

  const value = useMemo(() => ({ motionEnabled, setMotionEnabled }), [motionEnabled, setMotionEnabled]);

  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}

export function useMotion() {
  const ctx = useContext(Ctx);
  if (!ctx) throw new Error("useMotion must be used within MotionProvider");
  return ctx;
}
