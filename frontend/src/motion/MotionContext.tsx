import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import {
  applyMotionAttr,
  clampParticleCount,
  readMotionEnabled,
  readParticleCount,
  storeMotionEnabled,
  storeParticleCount,
} from "./prefs";

type MotionCtx = {
  motionEnabled: boolean;
  setMotionEnabled: (on: boolean) => void;
  particleCount: number;
  setParticleCount: (n: number) => void;
};

const Ctx = createContext<MotionCtx | null>(null);

applyMotionAttr(readMotionEnabled());

export function MotionProvider({ children }: { children: ReactNode }) {
  const [motionEnabled, setState] = useState(() => readMotionEnabled());
  const [particleCount, setParticleState] = useState(() => readParticleCount());

  const setMotionEnabled = useCallback((on: boolean) => {
    applyMotionAttr(on);
    storeMotionEnabled(on);
    setState(on);
  }, []);

  const setParticleCount = useCallback((n: number) => {
    const next = clampParticleCount(n);
    storeParticleCount(next);
    setParticleState(next);
  }, []);

  const value = useMemo(
    () => ({ motionEnabled, setMotionEnabled, particleCount, setParticleCount }),
    [motionEnabled, setMotionEnabled, particleCount, setParticleCount],
  );

  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}

export function useMotion() {
  const ctx = useContext(Ctx);
  if (!ctx) throw new Error("useMotion must be used within MotionProvider");
  return ctx;
}
